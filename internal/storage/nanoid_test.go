package storage

import (
	"strings"
	"testing"
)

// With no authentication on the server, a guessable profile id is the whole security
// story. "raka" can be typed by anyone who reaches the port; an opaque id cannot.
func TestNewProfileIDIsUnguessableAndPathSafe(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 500; i++ {
		id, err := NewProfileID()
		if err != nil {
			t.Fatal(err)
		}
		if len(id) != ProfileIDLength {
			t.Fatalf("id %q is %d chars, want %d", id, len(id), ProfileIDLength)
		}
		if seen[id] {
			t.Fatalf("collision on %q after %d ids", id, i)
		}
		seen[id] = true
		if !ValidID(id) {
			t.Fatalf("generated id %q fails ValidID", id)
		}
		// Ids become directory names and AAD components. Lowercase only, so a
		// case-insensitive filesystem cannot collide two distinct profiles.
		if id != strings.ToLower(id) {
			t.Fatalf("id %q contains uppercase; case-insensitive filesystems would collide", id)
		}
		for _, r := range id {
			if !strings.ContainsRune("0123456789abcdefghijklmnopqrstuvwxyz", r) {
				t.Fatalf("id %q contains %q, which is not path-safe", id, r)
			}
		}
	}
}

// Existing slug ids must keep working — there is real data under "raka".
func TestValidIDStillAcceptsLegacySlugs(t *testing.T) {
	for _, id := range []string{"raka", "dina", "test-user", "a1"} {
		if !ValidID(id) {
			t.Errorf("ValidID(%q) = false; existing profiles would become unreachable", id)
		}
	}
	for _, bad := range []string{"", "Raka", "../etc", "a/b", strings.Repeat("x", 40)} {
		if ValidID(bad) {
			t.Errorf("ValidID(%q) = true", bad)
		}
	}
}

func TestProfileCarriesADisplayName(t *testing.T) {
	db, err := Open(t.TempDir()+"/t.db", testKey)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	id, err := NewProfileID()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SaveProfile(Profile{ID: id, DisplayName: "Raka",
		Equipment: []string{"body weight"}}); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetProfile(id)
	if err != nil {
		t.Fatal(err)
	}
	if got.DisplayName != "Raka" {
		t.Errorf("display name = %q; an opaque id needs a human name beside it", got.DisplayName)
	}
}
