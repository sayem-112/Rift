package keys

import (
	"bytes"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	ring, err := NewKeyring(NewKey())
	if err != nil {
		t.Fatal(err)
	}
	user, book := uuid.New(), uuid.New()
	userKey, bookKey := NewKey(), NewKey()

	wrappedUser := ring.WrapUserKey(user, userKey)
	gotUser, err := ring.UnwrapUserKey(user, wrappedUser)
	if err != nil || !bytes.Equal(gotUser, userKey) {
		t.Fatalf("user key round trip failed: %v", err)
	}

	wrappedBook, err := WrapBookKey(gotUser, book, bookKey)
	if err != nil {
		t.Fatal(err)
	}
	gotBook, err := UnwrapBookKey(gotUser, book, wrappedBook)
	if err != nil || !bytes.Equal(gotBook, bookKey) {
		t.Fatalf("book key round trip failed: %v", err)
	}
}

func TestUnwrapRefusesWrongOwnerKeyOrTampering(t *testing.T) {
	ring, _ := NewKeyring(NewKey())
	other, _ := NewKeyring(NewKey())
	user := uuid.New()
	wrapped := ring.WrapUserKey(user, NewKey())

	tampered := bytes.Clone(wrapped)
	tampered[len(tampered)-1] ^= 1

	cases := map[string]func() error{
		"different user (row copied)": func() error { _, err := ring.UnwrapUserKey(uuid.New(), wrapped); return err },
		"different master key":        func() error { _, err := other.UnwrapUserKey(user, wrapped); return err },
		"flipped bit":                 func() error { _, err := ring.UnwrapUserKey(user, tampered); return err },
		"truncated":                   func() error { _, err := ring.UnwrapUserKey(user, wrapped[:10]); return err },
	}
	for name, unwrap := range cases {
		t.Run(name, func(t *testing.T) {
			if err := unwrap(); !errors.Is(err, ErrUnwrap) {
				t.Fatalf("want ErrUnwrap, got %v", err)
			}
		})
	}
}

func TestWrapsAreRandomized(t *testing.T) {
	ring, _ := NewKeyring(NewKey())
	user, key := uuid.New(), NewKey()
	if bytes.Equal(ring.WrapUserKey(user, key), ring.WrapUserKey(user, key)) {
		t.Fatal("wrapping the same key twice must not produce identical ciphertext")
	}
}

func TestMasterKeySize(t *testing.T) {
	if _, err := NewKeyring(make([]byte, 16)); err == nil {
		t.Fatal("a 16-byte master key must be rejected")
	}
}
