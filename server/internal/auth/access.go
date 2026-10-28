package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// AccessTTL is short on purpose: access tokens are verified without a database
// lookup of the session, so their lifetime bounds how long a stolen one works.
// Device revocation is still checked on every request (see httpapi).
const AccessTTL = 15 * time.Minute

const issuer = "rift"

var ErrInvalidToken = errors.New("invalid access token")

// Principal is who an authenticated request acts as.
type Principal struct {
	UserID   uuid.UUID
	DeviceID uuid.UUID
}

type accessClaims struct {
	DeviceID string `json:"did"`
	jwt.RegisteredClaims
}

// AccessTokens issues and verifies HMAC-signed JWTs.
type AccessTokens struct {
	secret []byte
	now    func() time.Time
}

func NewAccessTokens(secret []byte) *AccessTokens {
	return &AccessTokens{secret: secret, now: time.Now}
}

func (a *AccessTokens) Issue(p Principal) (token string, expiresAt time.Time, err error) {
	now := a.now()
	expiresAt = now.Add(AccessTTL)
	claims := accessClaims{
		DeviceID: p.DeviceID.String(),
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    issuer,
			Subject:   p.UserID.String(),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token, err = jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return token, expiresAt, nil
}

func (a *AccessTokens) Verify(token string) (Principal, error) {
	var claims accessClaims
	_, err := jwt.ParseWithClaims(token, &claims,
		func(*jwt.Token) (any, error) { return a.secret, nil },
		// Pinning the algorithm blocks "alg: none" and key-confusion attacks.
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(a.now),
	)
	if err != nil {
		return Principal{}, ErrInvalidToken
	}
	userID, err1 := uuid.Parse(claims.Subject)
	deviceID, err2 := uuid.Parse(claims.DeviceID)
	if err1 != nil || err2 != nil {
		return Principal{}, ErrInvalidToken
	}
	return Principal{UserID: userID, DeviceID: deviceID}, nil
}
