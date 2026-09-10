package storage

import (
	"bytes"
	"os"
	"testing"
)

// This is the whole reason the package moved off modernc.org/sqlite. Under the old driver
// this test FAILED: the note was findable in the database file with plain grep, and that
// file ships to the NAS every night.
func TestDatabaseFileIsNotPlaintext(t *testing.T) {
	path := t.TempDir() + "/t.db"
	db, err := Open(path, testKey)
	if err != nil {
		t.Fatal(err)
	}
	const secret = "DISTINCTIVE-NOTE-THAT-MUST-NOT-APPEAR"
	if err := db.SaveProfile(Profile{ID: "x", Goal: secret}); err != nil {
		t.Fatal(err)
	}
	if err := db.LogWeight("x", WeightEntry{Date: "2026-09-10", WeightKg: 80, Note: secret}); err != nil {
		t.Fatal(err)
	}
	db.Close()

	// The WAL matters as much as the main file: a page written but not yet checkpointed
	// is still a page on disk, and checking only the .db would miss it.
	for _, suffix := range []string{"", "-wal"} {
		raw, err := os.ReadFile(path + suffix)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("%s holds the note in plaintext — the database is not encrypted", path+suffix)
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.HasPrefix(raw, []byte("SQLite format 3")) {
		t.Fatal("file starts with the SQLite magic — it is a plain database")
	}
}

// A wrong key must FAIL, not open an empty database. Silently creating a fresh one would
// look like data loss to the user and would overwrite the real file on the next write.
func TestWrongKeyIsRejected(t *testing.T) {
	path := t.TempDir() + "/t.db"
	db, err := Open(path, testKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveProfile(Profile{ID: "x", Goal: "g"}); err != nil {
		t.Fatal(err)
	}
	db.Close()

	const otherKey = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	if _, err := Open(path, otherKey); err == nil {
		t.Fatal("opened an encrypted database with the wrong key")
	}
}

func TestShortKeyIsRejected(t *testing.T) {
	if _, err := Open(t.TempDir()+"/t.db", "abc"); err == nil {
		t.Fatal("accepted a key that is not 64 hex digits")
	}
}
