package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/rakasatria/sehaty/internal/crypto"
)

var ErrNoDocument = errors.New("no such document")

type Document struct {
	Key, Title, Kind, Body string
	Version                int
	Encrypted              bool
	UpdatedAt              string
}

// aad binds a ciphertext to exactly one profile, key and version. Moving the bytes
// anywhere else makes them undecryptable — see internal/crypto.
func aad(profileID, key string, version int) []byte {
	return []byte(fmt.Sprintf("%s|%s|%d", profileID, key, version))
}

// PutDocument writes a new VERSION rather than overwriting. A nutritionist revises a
// plan and the old one must remain readable — "what was I told in September" is a
// question people actually ask.
func (d *DB) PutDocument(c *crypto.Cipher, profileID string, doc Document) (int, error) {
	var last sql.NullInt64
	err := d.QueryRow(`SELECT MAX(version) FROM document WHERE profile_id=? AND key=?`,
		profileID, doc.Key).Scan(&last)
	if err != nil {
		return 0, fmt.Errorf("read current version: %w", err)
	}
	version := int(last.Int64) + 1

	body := []byte(doc.Body)
	encrypted := 0
	if doc.Encrypted {
		if c == nil {
			return 0, crypto.ErrNoKey
		}
		body, err = c.Seal(body, aad(profileID, doc.Key, version))
		if err != nil {
			return 0, fmt.Errorf("encrypt %s: %w", doc.Key, err)
		}
		encrypted = 1
	}
	_, err = d.Exec(`INSERT INTO document
		(profile_id, key, version, title, kind, body, encrypted, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		profileID, doc.Key, version, doc.Title, doc.Kind, body, encrypted,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("write document %s: %w", doc.Key, err)
	}
	return version, nil
}

// GetDocument returns the latest version when version <= 0.
func (d *DB) GetDocument(c *crypto.Cipher, profileID, key string, version int) (Document, error) {
	var (
		doc Document
		raw []byte
		enc int
	)
	q := `SELECT key, version, title, kind, body, encrypted, updated_at
	      FROM document WHERE profile_id=? AND key=?`
	args := []any{profileID, key}
	if version > 0 {
		q += ` AND version=?`
		args = append(args, version)
	} else {
		q += ` ORDER BY version DESC LIMIT 1`
	}
	err := d.QueryRow(q, args...).Scan(&doc.Key, &doc.Version, &doc.Title, &doc.Kind,
		&raw, &enc, &doc.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Document{}, fmt.Errorf("%s/%s: %w", profileID, key, ErrNoDocument)
	}
	if err != nil {
		return Document{}, err
	}
	doc.Encrypted = enc == 1
	if doc.Encrypted {
		if c == nil {
			return Document{}, crypto.ErrNoKey
		}
		pt, err := c.Open(raw, aad(profileID, doc.Key, doc.Version))
		if err != nil {
			return Document{}, fmt.Errorf("open %s v%d: %w", key, doc.Version, err)
		}
		raw = pt
	}
	doc.Body = string(raw)
	return doc, nil
}

// ListDocuments reports the latest version of each key. Bodies are never returned
// here — listing must not require the key, and must not decrypt what nobody asked for.
func (d *DB) ListDocuments(profileID string) ([]Document, error) {
	rows, err := d.Query(`SELECT key, MAX(version), title, kind, encrypted, updated_at
		FROM document WHERE profile_id=? GROUP BY key ORDER BY key`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Document
	for rows.Next() {
		var doc Document
		var enc int
		if err := rows.Scan(&doc.Key, &doc.Version, &doc.Title, &doc.Kind, &enc,
			&doc.UpdatedAt); err != nil {
			return nil, err
		}
		doc.Encrypted = enc == 1
		out = append(out, doc)
	}
	return out, rows.Err()
}
