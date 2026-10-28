package auth

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"rift/server/internal/testdb"
)

var phone = DeviceInfo{Name: "Phone", Kind: "phone", Client: "Chrome on Android"}

func newService(t *testing.T) *Service {
	t.Helper()
	return NewService(testdb.New(t), NewAccessTokens(testSecret))
}

func signIn(t *testing.T, s *Service, email string) Session {
	t.Helper()
	ctx := context.Background()
	token, err := s.CreateMagicLink(ctx, email)
	if err != nil {
		t.Fatal(err)
	}
	sess, err := s.RedeemMagicLink(ctx, token, phone)
	if err != nil {
		t.Fatal(err)
	}
	return sess
}

func TestMagicLinkIsSingleUseUnderConcurrency(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	token, err := s.CreateMagicLink(ctx, "reader@example.com")
	if err != nil {
		t.Fatal(err)
	}

	var wins, invalid atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.RedeemMagicLink(ctx, token, phone)
			switch {
			case err == nil:
				wins.Add(1)
			case errors.Is(err, ErrLinkInvalid):
				invalid.Add(1)
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if wins.Load() != 1 || invalid.Load() != 7 {
		t.Fatalf("wins=%d invalid=%d, want exactly one redemption", wins.Load(), invalid.Load())
	}
}

func TestSameEmailIsOneAccount(t *testing.T) {
	s := newService(t)
	a := signIn(t, s, "Reader@Example.com")
	b := signIn(t, s, "reader@example.com")
	if a.UserID != b.UserID {
		t.Fatal("emails differing only in case must map to one user")
	}
	if a.DeviceID == b.DeviceID {
		t.Fatal("each sign-in registers its own device")
	}
}

func TestGoogleLinksToExistingVerifiedEmail(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	viaEmail := signIn(t, s, "reader@example.com")

	id := GoogleIdentity{Subject: "g-123", Email: "reader@example.com", EmailVerified: true, Name: "Reader"}
	viaGoogle, err := s.SignInWithGoogle(ctx, id, phone)
	if err != nil {
		t.Fatal(err)
	}
	if viaGoogle.UserID != viaEmail.UserID {
		t.Fatal("verified Google email should link to the existing account")
	}

	again, err := s.SignInWithGoogle(ctx, GoogleIdentity{Subject: "g-123", Email: "changed@example.com"}, phone)
	if err != nil || again.UserID != viaEmail.UserID {
		t.Fatalf("a linked Google subject should keep finding the same user (err %v)", err)
	}

	_, err = s.SignInWithGoogle(ctx, GoogleIdentity{Subject: "g-999", Email: "other@example.com", EmailVerified: false}, phone)
	if !errors.Is(err, ErrUnverifiedEmail) {
		t.Fatalf("unverified email must not create or link an account, got %v", err)
	}
}

func TestRefreshRotates(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	first := signIn(t, s, "reader@example.com")

	second, err := s.Refresh(ctx, first.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	if second.RefreshToken == first.RefreshToken {
		t.Fatal("refresh must issue a new token")
	}
	if _, err := s.Authenticate(ctx, second.AccessToken); err != nil {
		t.Fatalf("new access token rejected: %v", err)
	}
	if _, err := s.Refresh(ctx, second.RefreshToken); err != nil {
		t.Fatalf("the newest token keeps working: %v", err)
	}
}

func TestRefreshReplayRevokesFamily(t *testing.T) {
	s := newService(t)
	s.grace = 0
	ctx := context.Background()
	stolen := signIn(t, s, "reader@example.com")

	legit, err := s.Refresh(ctx, stolen.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	// The attacker replays the old token after the owner already rotated it.
	if _, err := s.Refresh(ctx, stolen.RefreshToken); !errors.Is(err, ErrRefreshReplayed) {
		t.Fatalf("replay should be detected, got %v", err)
	}
	// Every token in the family is now dead, including the owner's current one.
	if _, err := s.Refresh(ctx, legit.RefreshToken); !errors.Is(err, ErrRefreshInvalid) {
		t.Fatalf("family should be revoked after a replay, got %v", err)
	}
}

func TestConcurrentRefreshWithinGraceIsNotAReplay(t *testing.T) {
	s := newService(t)
	s.grace = 5 * time.Second
	ctx := context.Background()
	sess := signIn(t, s, "reader@example.com")

	a, errA := s.Refresh(ctx, sess.RefreshToken)
	b, errB := s.Refresh(ctx, sess.RefreshToken) // second tab, same moment
	if errA != nil || errB != nil {
		t.Fatalf("both refreshes inside the grace window should succeed: %v / %v", errA, errB)
	}
	if a.RefreshToken == b.RefreshToken {
		t.Fatal("each refresh gets its own successor")
	}
	for _, tok := range []string{a.RefreshToken, b.RefreshToken} {
		if _, err := s.Refresh(ctx, tok); err != nil {
			t.Fatalf("both successors must stay valid: %v", err)
		}
	}
}

func TestRevokedDeviceIsLockedOut(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	pc := signIn(t, s, "reader@example.com")
	ph := signIn(t, s, "reader@example.com")

	if err := s.RevokeDevice(ctx, pc.UserID, ph.DeviceID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Authenticate(ctx, ph.AccessToken); !errors.Is(err, ErrDeviceRevoked) {
		t.Fatalf("revoked device's access token must stop working at once, got %v", err)
	}
	if _, err := s.Refresh(ctx, ph.RefreshToken); !errors.Is(err, ErrRefreshInvalid) {
		t.Fatalf("revoked device can't refresh, got %v", err)
	}
	if _, err := s.Authenticate(ctx, pc.AccessToken); err != nil {
		t.Fatalf("other devices are unaffected: %v", err)
	}
	if err := s.RevokeDevice(ctx, pc.UserID, ph.DeviceID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("revoking twice reports not found, got %v", err)
	}
}

func TestSignOutRevokesSession(t *testing.T) {
	s := newService(t)
	ctx := context.Background()
	sess := signIn(t, s, "reader@example.com")
	if err := s.SignOut(ctx, sess.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Refresh(ctx, sess.RefreshToken); !errors.Is(err, ErrRefreshInvalid) {
		t.Fatalf("signed-out token must not refresh, got %v", err)
	}
	if err := s.SignOut(ctx, "unknown-token"); err != nil {
		t.Fatalf("signing out an unknown token is a no-op, got %v", err)
	}
}
