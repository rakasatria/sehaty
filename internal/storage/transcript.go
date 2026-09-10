package storage

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var ErrNoTranscript = errors.New("no transcript for that recording")

// Transcript is what a voice note said.
//
// Kept separate from the recording itself so the audio stays where it is — encrypted, on
// disk — while the text becomes searchable and quotable. Source records WHO produced it,
// because a machine transcription and a human correction should never be confused: one is
// a guess about what was said, the other is what was said.
type Transcript struct {
	Hash      string
	Text      string
	Source    string // model id, or "user" for a correction
	Language  string
	CreatedAt string
}

// PutTranscript stores or replaces the text for one recording.
func (d *DB) PutTranscript(profileID, hash, text, source, lang string) error {
	if strings.TrimSpace(hash) == "" {
		return fmt.Errorf("hash is required")
	}
	if !ValidID(profileID) {
		return fmt.Errorf("invalid profile id %q", profileID)
	}
	_, err := d.Exec(`INSERT INTO media_transcript
		(profile_id, hash, text, source, language, created_at) VALUES (?,?,?,?,?,?)
		ON CONFLICT(profile_id, hash) DO UPDATE SET
		    text=excluded.text, source=excluded.source,
		    language=excluded.language, created_at=excluded.created_at`,
		profileID, hash, text, source, lang, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save transcript: %w", err)
	}
	return nil
}

func (d *DB) GetTranscript(profileID, hash string) (Transcript, error) {
	var t Transcript
	err := d.QueryRow(`SELECT hash, text, source, language, created_at
		FROM media_transcript WHERE profile_id=? AND hash=?`, profileID, hash).
		Scan(&t.Hash, &t.Text, &t.Source, &t.Language, &t.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Transcript{}, fmt.Errorf("%s: %w", hash, ErrNoTranscript)
	}
	return t, err
}

// Transcripts lists what has been transcribed for one profile, newest first.
func (d *DB) Transcripts(profileID string, limit int) ([]Transcript, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := d.Query(`SELECT hash, text, source, language, created_at
		FROM media_transcript WHERE profile_id=? ORDER BY created_at DESC LIMIT ?`,
		profileID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Transcript{}
	for rows.Next() {
		var t Transcript
		if err := rows.Scan(&t.Hash, &t.Text, &t.Source, &t.Language, &t.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
