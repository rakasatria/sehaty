package storage

import (
	"errors"
	"fmt"
	"time"

	"github.com/rakasatria/sehaty/internal/crypto"
)

// ErrVersionConflict means the document moved since the caller last read it.
var ErrVersionConflict = errors.New("document version conflict")

// PutDocumentIfVersion writes a new version only if the head is where the caller thinks.
//
//	expected <  0   no check — append regardless
//	expected == 0   the document must NOT exist yet
//	expected == N   the current head must be exactly N
//
// On a single-user box this is barely about concurrency. Its real value is as a fence
// against the best-documented agent failure: acting on a copy read earlier in the
// conversation. A stale agent cannot clobber the store — it gets a conflict and must
// re-read. This is HTTP's If-Match, repurposed as a guardrail.
//
// The check and the insert share one transaction, so the head cannot move between them.
func (d *DB) PutDocumentIfVersion(c *crypto.Cipher, profileID string, doc Document, expected int) (int, error) {
	tx, err := d.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var head int
	err = tx.QueryRow(`SELECT COALESCE(MAX(version),0) FROM document
		WHERE profile_id=? AND key=?`, profileID, doc.Key).Scan(&head)
	if err != nil {
		return 0, fmt.Errorf("read current version: %w", err)
	}
	if expected >= 0 && head != expected {
		if expected == 0 {
			return 0, fmt.Errorf("%q already exists at version %d: %w", doc.Key, head, ErrVersionConflict)
		}
		return 0, fmt.Errorf("%q is at version %d, not %d — re-read it before writing: %w",
			doc.Key, head, expected, ErrVersionConflict)
	}

	version := head + 1
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
	_, err = tx.Exec(`INSERT INTO document
		(profile_id, key, version, title, kind, body, encrypted, updated_at)
		VALUES (?,?,?,?,?,?,?,?)`,
		profileID, doc.Key, version, doc.Title, doc.Kind, body, encrypted,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("write document %s: %w", doc.Key, err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return version, nil
}

// DocumentVersions lists every version of one document, newest first, WITHOUT bodies.
//
// Reading what exists must not require the key and must not drag every old revision into
// memory — history is usually consulted to pick a version, not to read them all.
func (d *DB) DocumentVersions(profileID, key string) ([]Document, error) {
	rows, err := d.Query(`SELECT key, version, title, kind, encrypted, updated_at
		FROM document WHERE profile_id=? AND key=? ORDER BY version DESC`, profileID, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Document{}
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
