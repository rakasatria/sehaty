package storage

import "testing"

func TestOpenCreatesSchema(t *testing.T) {
	db, err := Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	for _, table := range []string{"profile", "identity", "training_log",
		"cardio_log", "weight_log", "food_log", "document", "media"} {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`,
			table).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing: %v", table, err)
		}
	}
}

func TestOpenSetsWAL(t *testing.T) {
	db, err := Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
}
