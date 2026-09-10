package storage

import "fmt"

// DeleteProfile removes a person and every trace of them.
//
// This is the right-to-be-forgotten path, and it is deliberately NOT exposed as an MCP
// tool. Destructive operations should not be reachable by a language model that might
// misread "delete that entry" as "delete everything"; a human runs this from the command
// line, where the blast radius is visible.
//
// Everything happens in one transaction: a half-deleted profile — logs gone, documents
// left — would be worse than either outcome.
func (d *DB) DeleteProfile(id string) error {
	if _, err := d.GetProfile(id); err != nil {
		return err
	}
	tx, err := d.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// document and the logs first: they reference profile(id).
	for _, table := range []string{"training_log", "cardio_log", "weight_log", "food_log",
		"document", "identity"} {
		if _, err := tx.Exec("DELETE FROM "+table+" WHERE profile_id=?", id); err != nil {
			return fmt.Errorf("delete %s for %s: %w", table, id, err)
		}
	}
	if _, err := tx.Exec("DELETE FROM profile WHERE id=?", id); err != nil {
		return fmt.Errorf("delete profile %s: %w", id, err)
	}
	return tx.Commit()
}
