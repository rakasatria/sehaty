package crypto

import (
	"crypto/hkdf"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// TWO ciphers protect Sehaty's data, and they must not share key material.
//
// Adiantum encrypts the whole database FILE; AES-256-GCM encrypts document BODIES inside
// it. Handing the same 32 bytes to both is the classic key-reuse mistake — two different
// constructions operating on related plaintext under one key, so a weakness in either
// implicates the other. HKDF costs nothing and removes the question.
//
// The labels are versioned: bumping one rotates that subkey without disturbing the other.
const (
	infoDB    = "sehaty:db:adiantum:v1"
	infoDocs  = "sehaty:doc:aes256gcm:v1"
	infoMedia = "sehaty:media:aes256gcm:v1"
)

// decodeMaster validates SEHATY_KEY and returns the raw master key.
//
// A short key is rejected loudly rather than silently padded, because silent padding is
// how "encrypted" data ends up trivially breakable.
func decodeMaster(b64 string) ([]byte, error) {
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
	return key, nil
}

func derive(master []byte, info string) ([]byte, error) {
	return hkdf.Key(sha256.New, master, nil, info, KeyBytes)
}

// DBKeyHex returns the 64-hex-digit subkey for the Adiantum VFS, derived from the master
// SEHATY_KEY. This is what encrypts every table — logs, profiles, identities and all.
func DBKeyHex(b64 string) (string, error) {
	master, err := decodeMaster(b64)
	if err != nil {
		return "", err
	}
	k, err := derive(master, infoDB)
	if err != nil {
		return "", fmt.Errorf("derive database key: %w", err)
	}
	return hex.EncodeToString(k), nil
}
