package storage

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNoToken = errors.New("token not recognised")

// Token is an API credential bound to one profile.
//
// The PLAINTEXT IS NEVER STORED. Only its SHA-256 lands in the database, exactly as a
// password would be — anyone who reads a backup gets hashes, not working credentials.
// SHA-256 without a slow KDF is right here and wrong for passwords: these are 256-bit
// random strings, so there is no dictionary to attack and nothing for bcrypt to buy.
type Token struct {
	ID        string // short public identifier, safe to print and to revoke by
	ProfileID string // empty means an ADMIN token: may act on any profile
	Label     string
	CreatedAt string
	RevokedAt string
}

func (t Token) Revoked() bool { return t.RevokedAt != "" }
func (t Token) Admin() bool   { return t.ProfileID == "" }

// NewTokenSecret returns a fresh 256-bit credential. Shown ONCE at creation; after that
// only its hash exists, so a lost token is reissued rather than recovered.
func NewTokenSecret() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return "sht_" + hex.EncodeToString(b), nil
}

func HashToken(secret string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return hex.EncodeToString(sum[:])
}

// tokenID is a short, non-secret handle derived from the hash, so a token can be listed
// and revoked without ever printing the credential itself.
func tokenID(hash string) string { return hash[:12] }

// IssueToken creates a credential for a profile. An empty profileID makes an admin token,
// which is what registration needs: a person has no profile until register creates one, so
// something has to be able to call it before any profile token can exist.
func (d *DB) IssueToken(profileID, label string) (secret string, t Token, err error) {
	if profileID != "" {
		if _, err := d.GetProfile(profileID); err != nil {
			return "", Token{}, err
		}
	}
	secret, err = NewTokenSecret()
	if err != nil {
		return "", Token{}, err
	}
	hash := HashToken(secret)
	now := time.Now().UTC().Format(time.RFC3339)
	_, err = d.Exec(`INSERT INTO api_token (id, token_hash, profile_id, label, created_at)
		VALUES (?,?,?,?,?)`, tokenID(hash), hash, profileID, label, now)
	if err != nil {
		return "", Token{}, fmt.Errorf("store token: %w", err)
	}
	return secret, Token{ID: tokenID(hash), ProfileID: profileID, Label: label,
		CreatedAt: now}, nil
}

// ResolveToken identifies the caller. A revoked token is rejected exactly like an unknown
// one — the reply must not tell an attacker that a credential once existed.
func (d *DB) ResolveToken(secret string) (Token, error) {
	if strings.TrimSpace(secret) == "" {
		return Token{}, ErrNoToken
	}
	var t Token
	err := d.QueryRow(`SELECT id, profile_id, label, created_at, COALESCE(revoked_at,'')
		FROM api_token WHERE token_hash=?`, HashToken(secret)).
		Scan(&t.ID, &t.ProfileID, &t.Label, &t.CreatedAt, &t.RevokedAt)
	if err != nil || t.Revoked() {
		return Token{}, ErrNoToken
	}
	return t, nil
}

func (d *DB) ListTokens() ([]Token, error) {
	rows, err := d.Query(`SELECT id, profile_id, label, created_at, COALESCE(revoked_at,'')
		FROM api_token ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Token{}
	for rows.Next() {
		var t Token
		if err := rows.Scan(&t.ID, &t.ProfileID, &t.Label, &t.CreatedAt, &t.RevokedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// RevokeToken disables a credential. The row is kept rather than deleted so the history of
// what once had access survives — that is the point of an audit trail.
func (d *DB) RevokeToken(id string) error {
	res, err := d.Exec(`UPDATE api_token SET revoked_at=? WHERE id=? AND revoked_at IS NULL`,
		time.Now().UTC().Format(time.RFC3339), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("%q: %w", id, ErrNoToken)
	}
	return nil
}

// RevokeProfileTokens disables every credential for one profile — "reset my access".
func (d *DB) RevokeProfileTokens(profileID string) (int, error) {
	res, err := d.Exec(`UPDATE api_token SET revoked_at=?
		WHERE profile_id=? AND revoked_at IS NULL`,
		time.Now().UTC().Format(time.RFC3339), profileID)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}
