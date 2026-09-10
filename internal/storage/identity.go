package storage

import (
	"database/sql"
	"errors"
	"fmt"
)

var ErrNoIdentity = errors.New("identity not linked to any profile")

// LinkIdentity binds an external account to a profile. Channels are separate
// namespaces: telegram/8412 and access/8412 are unrelated. The foreign key on
// profile_id is what rejects a link to a profile that does not exist — which is why
// PRAGMA foreign_keys=ON in Open() is load-bearing rather than decoration.
func (d *DB) LinkIdentity(channel, externalID, profileID string) error {
	_, err := d.Exec(`INSERT INTO identity (channel, external_id, profile_id)
		VALUES (?,?,?)
		ON CONFLICT(channel, external_id) DO UPDATE SET profile_id=excluded.profile_id`,
		channel, externalID, profileID)
	if err != nil {
		return fmt.Errorf("link %s/%s -> %s: %w", channel, externalID, profileID, err)
	}
	return nil
}

// ResolveIdentity answers "who is this?" for an incoming message. Every front door
// calls this once; after that the profile id is carried, and nobody is asked again.
func (d *DB) ResolveIdentity(channel, externalID string) (string, error) {
	var id string
	err := d.QueryRow(`SELECT profile_id FROM identity
		WHERE channel=? AND external_id=?`, channel, externalID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%s/%s: %w", channel, externalID, ErrNoIdentity)
	}
	return id, err
}
