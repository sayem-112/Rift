package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// NewOpaqueToken returns 256 bits of randomness, URL-safe. Used for refresh
// tokens, magic links and OAuth state: anything the server only needs to
// recognise, never to read.
func NewOpaqueToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand never fails on supported platforms; if it does, nothing is safe.
		panic("crypto/rand: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// HashToken is what gets stored. A database leak then yields hashes that
// can't be replayed. Tokens are high-entropy, so a fast hash is sufficient.
func HashToken(token string) []byte {
	sum := sha256.Sum256([]byte(token))
	return sum[:]
}
