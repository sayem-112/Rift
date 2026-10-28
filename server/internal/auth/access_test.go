package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var testSecret = []byte("0123456789abcdef0123456789abcdef")

func TestAccessTokenRoundTrip(t *testing.T) {
	tokens := NewAccessTokens(testSecret)
	want := Principal{UserID: uuid.New(), DeviceID: uuid.New()}

	token, exp, err := tokens.Issue(want)
	if err != nil {
		t.Fatal(err)
	}
	if d := time.Until(exp); d < AccessTTL-time.Minute || d > AccessTTL {
		t.Errorf("expiry %v from now, want ~%v", d, AccessTTL)
	}
	got, err := tokens.Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("principal = %+v, want %+v", got, want)
	}
}

func TestAccessTokenRejections(t *testing.T) {
	tokens := NewAccessTokens(testSecret)
	valid, _, _ := tokens.Issue(Principal{UserID: uuid.New(), DeviceID: uuid.New()})

	expired := NewAccessTokens(testSecret)
	expired.now = func() time.Time { return time.Now().Add(-AccessTTL - time.Minute) }
	old, _, _ := expired.Issue(Principal{UserID: uuid.New(), DeviceID: uuid.New()})

	otherKey, _, _ := NewAccessTokens([]byte("another-secret-another-secret-xx")).Issue(Principal{UserID: uuid.New(), DeviceID: uuid.New()})

	unsigned, _ := jwt.NewWithClaims(jwt.SigningMethodNone, jwt.MapClaims{
		"sub": uuid.NewString(), "did": uuid.NewString(), "iss": "rift", "exp": time.Now().Add(time.Hour).Unix(),
	}).SignedString(jwt.UnsafeAllowNoneSignatureType)

	tests := map[string]string{
		"expired":   old,
		"wrong key": otherKey,
		"alg none":  unsigned,
		"tampered":  valid[:len(valid)-2] + flip(valid[len(valid)-2:]),
		"garbage":   "not.a.token",
		"empty":     "",
		"truncated": strings.Join(strings.Split(valid, ".")[:2], "."),
	}
	for name, token := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := tokens.Verify(token); err == nil {
				t.Fatal("expected rejection")
			}
		})
	}
}

func flip(s string) string {
	if s[0] == 'A' {
		return "B" + s[1:]
	}
	return "A" + s[1:]
}

func TestOpaqueTokens(t *testing.T) {
	a, b := NewOpaqueToken(), NewOpaqueToken()
	if a == b || len(a) != 43 {
		t.Fatalf("tokens should be unique 43-char strings: %q %q", a, b)
	}
	if string(HashToken(a)) == string(HashToken(b)) || len(HashToken(a)) != 32 {
		t.Fatal("hashes should be distinct SHA-256 digests")
	}
}
