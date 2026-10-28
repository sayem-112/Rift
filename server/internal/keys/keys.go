// Package keys implements Rift's envelope encryption.
//
//	master key (env / secrets manager)
//	  └─ wraps each user's key   (stored in users.wrapped_key)
//	       └─ wraps each book's key (stored in books.wrapped_key)
//	            └─ encrypts the book file (client-side, AES-GCM)
//
// Wrapping is AES-256-GCM with the owner's ID as associated data, so a wrapped
// key copied onto another user's or book's row fails to unwrap instead of
// silently decrypting the wrong thing. A leaked database or bucket alone is
// useless, and destroying a user's wrapped key makes all of their books
// unreadable at once (crypto-shredding).
package keys

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

const KeySize = 32 // AES-256

var ErrUnwrap = errors.New("key unwrap failed: wrong key, wrong owner, or tampered data")

type Keyring struct {
	master cipher.AEAD
}

func NewKeyring(master []byte) (*Keyring, error) {
	if len(master) != KeySize {
		return nil, fmt.Errorf("master key must be %d bytes, got %d", KeySize, len(master))
	}
	aead, err := newAEAD(master)
	if err != nil {
		return nil, err
	}
	return &Keyring{master: aead}, nil
}

// NewKey returns a fresh random data key.
func NewKey() []byte {
	k := make([]byte, KeySize)
	if _, err := rand.Read(k); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return k
}

// WrapUserKey encrypts a user's key under the master key, bound to the user ID.
func (r *Keyring) WrapUserKey(userID uuid.UUID, key []byte) []byte {
	return seal(r.master, key, userID[:])
}

func (r *Keyring) UnwrapUserKey(userID uuid.UUID, wrapped []byte) ([]byte, error) {
	return open(r.master, wrapped, userID[:])
}

// WrapBookKey encrypts a book's key under its owner's key, bound to the book ID.
func WrapBookKey(userKey []byte, bookID uuid.UUID, key []byte) ([]byte, error) {
	aead, err := newAEAD(userKey)
	if err != nil {
		return nil, err
	}
	return seal(aead, key, bookID[:]), nil
}

func UnwrapBookKey(userKey []byte, bookID uuid.UUID, wrapped []byte) ([]byte, error) {
	aead, err := newAEAD(userKey)
	if err != nil {
		return nil, err
	}
	return open(aead, wrapped, bookID[:])
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

// seal returns nonce || ciphertext || tag. Nonces are random: each key wraps
// at most a handful of values, far below GCM's random-nonce limits.
func seal(aead cipher.AEAD, plaintext, aad []byte) []byte {
	nonce := make([]byte, aead.NonceSize(), aead.NonceSize()+len(plaintext)+aead.Overhead())
	if _, err := rand.Read(nonce); err != nil {
		panic("crypto/rand: " + err.Error())
	}
	return aead.Seal(nonce, nonce, plaintext, aad)
}

func open(aead cipher.AEAD, sealed, aad []byte) ([]byte, error) {
	if len(sealed) < aead.NonceSize()+aead.Overhead() {
		return nil, ErrUnwrap
	}
	nonce, ct := sealed[:aead.NonceSize()], sealed[aead.NonceSize():]
	pt, err := aead.Open(nil, nonce, ct, aad)
	if err != nil {
		return nil, ErrUnwrap
	}
	return pt, nil
}
