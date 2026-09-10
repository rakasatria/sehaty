// Package crypto encrypts documents at rest.
//
// AES-256-GCM: the algorithm NSA's CNSA suite approves for TOP SECRET, and an AEAD, so
// ciphertext that has been altered fails to decrypt rather than returning wrong plaintext.
//
// WHAT THIS PROTECTS, stated plainly because "encrypted" is often claimed to mean more
// than it does: the key lives in the server's environment, so a running server can always
// decrypt. This defends BACKUPS, a STOLEN DISK, and a copied container image. It does NOT
// defend against someone who has compromised the running process. That limit is the price
// of Sehaty being able to send you a weekly review without you typing a passphrase, and it
// was chosen deliberately.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

var ErrNoKey = errors.New("SEHATY_KEY is not set — required to read or write documents")

const KeyBytes = 32 // AES-256

type Cipher struct{ aead cipher.AEAD }

// New builds a Cipher from a base64 key. The key must decode to exactly 32 bytes; a
// short key is rejected loudly rather than silently padded, because silent padding is
// how "encrypted" data ends up trivially breakable.
func New(b64 string) (*Cipher, error) {
	if b64 == "" {
		return nil, ErrNoKey
	}
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, fmt.Errorf("SEHATY_KEY is not valid base64: %w", err)
	}
	if len(key) != KeyBytes {
		return nil, fmt.Errorf("SEHATY_KEY decodes to %d bytes, need %d (AES-256)",
			len(key), KeyBytes)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Cipher{aead: aead}, nil
}

// GenerateKey returns a fresh base64 AES-256 key for SEHATY_KEY.
//
// LOSING THIS KEY MEANS LOSING EVERY ENCRYPTED DOCUMENT. There is no recovery path and
// that is the point. Back it up somewhere that is not the server it protects.
func GenerateKey() (string, error) {
	k := make([]byte, KeyBytes)
	if _, err := io.ReadFull(rand.Reader, k); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(k), nil
}

// Seal encrypts plaintext, binding it to aad.
//
// The AAD is what stops a ciphertext being MOVED. Without it, anyone who can write to the
// database could copy one profile's encrypted diet plan into another profile's row and it
// would decrypt perfectly — the bytes are valid, only the context changed. Binding
// profile/key/version means such a row fails to open at all. This matters more here than
// the key length does.
func (c *Cipher) Seal(plaintext, aad []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}
	// Nonce is prepended, not stored separately: a nonce that goes missing makes the
	// ciphertext undecryptable, and GCM nonces are not secret.
	return c.aead.Seal(nonce, nonce, plaintext, aad), nil
}

func (c *Cipher) Open(ciphertext, aad []byte) ([]byte, error) {
	n := c.aead.NonceSize()
	if len(ciphertext) < n {
		return nil, errors.New("ciphertext shorter than nonce — truncated or corrupt")
	}
	pt, err := c.aead.Open(nil, ciphertext[:n], ciphertext[n:], aad)
	if err != nil {
		return nil, fmt.Errorf("decrypt failed — wrong key, altered data, or wrong "+
			"profile/key/version: %w", err)
	}
	return pt, nil
}
