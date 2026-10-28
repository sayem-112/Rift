package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"rift/server/internal/database"
	"rift/server/internal/metrics"
	"rift/server/internal/store"
)

const (
	// RefreshTTL slides: every refresh issues a token valid for another 90 days,
	// so a device signs out only after 90 days of not being used.
	RefreshTTL   = 90 * 24 * time.Hour
	MagicLinkTTL = 15 * time.Minute

	// refreshGrace tolerates two tabs refreshing with the same token at nearly
	// the same moment. Within it, a rotated token gets another successor
	// instead of being treated as stolen.
	defaultRefreshGrace = 10 * time.Second

	ProviderGoogle = "google"
)

var (
	ErrInvalidEmail    = errors.New("invalid email address")
	ErrInvalidDevice   = errors.New("invalid device details")
	ErrLinkInvalid     = errors.New("sign-in link is invalid, used or expired")
	ErrRefreshInvalid  = errors.New("refresh token is invalid, revoked or expired")
	ErrRefreshReplayed = errors.New("refresh token was already used; sessions for this device were revoked")
	ErrUnverifiedEmail = errors.New("google account email is not verified")
	ErrDeviceRevoked   = errors.New("device is signed out")
	ErrNotFound        = errors.New("not found")
)

// DeviceInfo is what a client reports about itself when signing in.
type DeviceInfo struct {
	Name   string `json:"name"`
	Kind   string `json:"kind"`
	Client string `json:"client"`
}

func (d DeviceInfo) validate() error {
	n := utf8.RuneCountInString(strings.TrimSpace(d.Name))
	if n < 1 || n > 60 || utf8.RuneCountInString(d.Client) > 80 {
		return ErrInvalidDevice
	}
	switch d.Kind {
	case "phone", "tablet", "desktop":
		return nil
	}
	return ErrInvalidDevice
}

// Session is the result of signing in or refreshing.
type Session struct {
	UserID           uuid.UUID
	DeviceID         uuid.UUID
	AccessToken      string
	AccessExpiresAt  time.Time
	RefreshToken     string
	RefreshExpiresAt time.Time
}

// GoogleIdentity is the verified subset of a Google ID token.
type GoogleIdentity struct {
	Subject       string
	Email         string
	EmailVerified bool
	Name          string
}

type Service struct {
	pool   *pgxpool.Pool
	q      *store.Queries
	access *AccessTokens
	grace  time.Duration
}

func NewService(pool *pgxpool.Pool, access *AccessTokens) *Service {
	return &Service{pool: pool, q: store.New(pool), access: access, grace: defaultRefreshGrace}
}

// NormalizeEmail validates an address and returns it trimmed, without a display name.
func NormalizeEmail(raw string) (string, error) {
	addr, err := mail.ParseAddress(strings.TrimSpace(raw))
	if err != nil || addr.Name != "" || len(addr.Address) > 254 {
		return "", ErrInvalidEmail
	}
	return addr.Address, nil
}

// CreateMagicLink stores a single-use link token and returns it. Only its hash
// is stored; the caller emails the token and then forgets it.
func (s *Service) CreateMagicLink(ctx context.Context, email string) (token string, err error) {
	token = NewOpaqueToken()
	err = s.q.CreateMagicLink(ctx, store.CreateMagicLinkParams{
		TokenHash: HashToken(token),
		Email:     email,
		ExpiresAt: time.Now().Add(MagicLinkTTL),
	})
	if err != nil {
		return "", fmt.Errorf("store magic link: %w", err)
	}
	return token, nil
}

// RedeemMagicLink consumes a link and signs the device in.
func (s *Service) RedeemMagicLink(ctx context.Context, token string, device DeviceInfo) (Session, error) {
	if err := device.validate(); err != nil {
		return Session{}, err
	}
	var sess Session
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		email, err := q.ConsumeMagicLink(ctx, HashToken(token))
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrLinkInvalid
		}
		if err != nil {
			return err
		}
		user, err := q.UpsertUserByEmail(ctx, store.UpsertUserByEmailParams{Email: email})
		if err != nil {
			return err
		}
		sess, err = s.startSession(ctx, q, user.ID, device)
		return err
	})
	if err == nil {
		metrics.AuthEvents.WithLabelValues("signin_email").Inc()
	}
	return sess, err
}

// SignInWithGoogle signs in the account linked to a Google identity, linking
// it to an existing account with the same verified email, or creating one.
func (s *Service) SignInWithGoogle(ctx context.Context, id GoogleIdentity, device DeviceInfo) (Session, error) {
	if err := device.validate(); err != nil {
		return Session{}, err
	}
	var sess Session
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		user, err := q.GetIdentityUser(ctx, store.GetIdentityUserParams{Provider: ProviderGoogle, Subject: id.Subject})
		if errors.Is(err, pgx.ErrNoRows) {
			// Linking by email is only safe when Google vouches for the address.
			if !id.EmailVerified {
				return ErrUnverifiedEmail
			}
			var name *string
			if id.Name != "" {
				name = &id.Name
			}
			user, err = q.UpsertUserByEmail(ctx, store.UpsertUserByEmailParams{Email: id.Email, Name: name})
			if err != nil {
				return err
			}
			err = q.LinkIdentity(ctx, store.LinkIdentityParams{Provider: ProviderGoogle, Subject: id.Subject, UserID: user.ID})
		}
		if err != nil {
			return err
		}
		sess, err = s.startSession(ctx, q, user.ID, device)
		return err
	})
	if err == nil {
		metrics.AuthEvents.WithLabelValues("signin_google").Inc()
	}
	return sess, err
}

func (s *Service) startSession(ctx context.Context, q *store.Queries, userID uuid.UUID, info DeviceInfo) (Session, error) {
	device, err := q.CreateDevice(ctx, store.CreateDeviceParams{
		UserID: userID,
		Name:   strings.TrimSpace(info.Name),
		Kind:   info.Kind,
		Client: info.Client,
	})
	if err != nil {
		return Session{}, fmt.Errorf("create device: %w", err)
	}
	return s.issue(ctx, q, userID, device.ID, uuid.New())
}

// issue creates a refresh token in the given family plus a fresh access token.
func (s *Service) issue(ctx context.Context, q *store.Queries, userID, deviceID, familyID uuid.UUID) (Session, error) {
	refresh := NewOpaqueToken()
	refreshExp := time.Now().Add(RefreshTTL)
	if _, err := q.CreateSession(ctx, store.CreateSessionParams{
		FamilyID:         familyID,
		DeviceID:         deviceID,
		RefreshTokenHash: HashToken(refresh),
		ExpiresAt:        refreshExp,
	}); err != nil {
		return Session{}, fmt.Errorf("create session: %w", err)
	}
	access, accessExp, err := s.access.Issue(Principal{UserID: userID, DeviceID: deviceID})
	if err != nil {
		return Session{}, err
	}
	return Session{
		UserID:           userID,
		DeviceID:         deviceID,
		AccessToken:      access,
		AccessExpiresAt:  accessExp,
		RefreshToken:     refresh,
		RefreshExpiresAt: refreshExp,
	}, nil
}

// Refresh rotates a refresh token. A token that was already rotated is a
// replay, likely a stolen copy, so its whole family is revoked and the device
// must sign in again.
func (s *Service) Refresh(ctx context.Context, token string) (Session, error) {
	var (
		sess     Session
		replayed bool
	)
	err := database.InTx(ctx, s.pool, func(q *store.Queries) error {
		row, err := q.GetSessionForRefresh(ctx, store.GetSessionForRefreshParams{
			TokenHash:    HashToken(token),
			GraceSeconds: int32(s.grace.Seconds()),
		})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrRefreshInvalid
		}
		if err != nil {
			return err
		}
		switch {
		case row.Revoked || row.Expired:
			return ErrRefreshInvalid
		case row.Rotated && !row.InGrace:
			// Commit the revocation, then report the replay outside the transaction.
			replayed = true
			return q.RevokeSessionFamily(ctx, row.FamilyID)
		case !row.Rotated:
			if err := q.MarkSessionRotated(ctx, row.ID); err != nil {
				return err
			}
		}
		sess, err = s.issue(ctx, q, row.UserID, row.DeviceID, row.FamilyID)
		return err
	})
	switch {
	case err != nil:
		return Session{}, err
	case replayed:
		metrics.AuthEvents.WithLabelValues("refresh_replayed").Inc()
		return Session{}, ErrRefreshReplayed
	}
	metrics.AuthEvents.WithLabelValues("refresh").Inc()
	return sess, nil
}

// SignOut revokes the token's whole family. Unknown tokens are ignored, so
// signing out is always safe to retry.
func (s *Service) SignOut(ctx context.Context, token string) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		row, err := q.GetSessionForRefresh(ctx, store.GetSessionForRefreshParams{TokenHash: HashToken(token)})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		return q.RevokeSessionFamily(ctx, row.FamilyID)
	})
}

// Authenticate verifies an access token and that its device is still signed
// in. The device check costs one indexed lookup per request; it is what makes
// "sign out this device" take effect immediately rather than in 15 minutes.
func (s *Service) Authenticate(ctx context.Context, accessToken string) (Principal, error) {
	p, err := s.access.Verify(accessToken)
	if err != nil {
		return Principal{}, err
	}
	device, err := s.q.GetActiveDevice(ctx, p.DeviceID)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && device.UserID != p.UserID) {
		return Principal{}, ErrDeviceRevoked
	}
	if err != nil {
		return Principal{}, err
	}
	_ = s.q.TouchDevice(ctx, p.DeviceID) // best effort; throttled in SQL
	return p, nil
}

// DeviceActive reports whether a device is still signed in. Long-lived
// connections (event streams) re-check it periodically.
func (s *Service) DeviceActive(ctx context.Context, deviceID uuid.UUID) bool {
	_, err := s.q.GetActiveDevice(ctx, deviceID)
	return err == nil
}

func (s *Service) User(ctx context.Context, id uuid.UUID) (store.User, error) {
	u, err := s.q.GetUser(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return store.User{}, ErrNotFound
	}
	return u, err
}

func (s *Service) Devices(ctx context.Context, userID uuid.UUID) ([]store.Device, error) {
	return s.q.ListDevices(ctx, userID)
}

// RevokeDevice signs a device out everywhere: its sessions die now, and its
// access tokens stop working on their next request.
func (s *Service) RevokeDevice(ctx context.Context, userID, deviceID uuid.UUID) error {
	return database.InTx(ctx, s.pool, func(q *store.Queries) error {
		n, err := q.RevokeDevice(ctx, store.RevokeDeviceParams{ID: deviceID, UserID: userID})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrNotFound
		}
		return q.RevokeDeviceSessions(ctx, deviceID)
	})
}

// Cleanup deletes expired sign-in links and long-dead sessions. Run as a periodic job.
func (s *Service) Cleanup(ctx context.Context) error {
	if _, err := s.q.DeleteExpiredMagicLinks(ctx); err != nil {
		return err
	}
	_, err := s.q.DeleteStaleSessions(ctx)
	return err
}
