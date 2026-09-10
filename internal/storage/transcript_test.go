package storage

import (
	"errors"
	"testing"
)

func transDB(t *testing.T) (*DB, string) {
	t.Helper()
	db, err := Open(t.TempDir()+"/t.db", testKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	id, _ := NewProfileID()
	if err := db.SaveProfile(Profile{ID: id, DisplayName: "Raka",
		Equipment: []string{"body weight"}}); err != nil {
		t.Fatal(err)
	}
	return db, id
}

func TestTranscriptRoundTrip(t *testing.T) {
	db, id := transDB(t)
	h := "abc123"
	if err := db.PutTranscript(id, h, "tadi makan nasi goreng", "google/gemini-3.8-flash", "id"); err != nil {
		t.Fatal(err)
	}
	got, err := db.GetTranscript(id, h)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "tadi makan nasi goreng" {
		t.Errorf("text = %q", got.Text)
	}
	if got.Source == "" || got.Language != "id" {
		t.Errorf("provenance lost: %+v", got)
	}
}

// A human correction must replace the machine guess, and the source must change with it —
// otherwise a guess silently hardens into a record.
func TestUserCorrectionReplacesTheMachineGuess(t *testing.T) {
	db, id := transDB(t)
	h := "abc123"
	must := func(e error) {
		if e != nil {
			t.Fatal(e)
		}
	}
	must(db.PutTranscript(id, h, "nasi goreng", "google/gemini-3.8-flash", "id"))
	must(db.PutTranscript(id, h, "nasi goreng ayam, porsi besar", "user", "id"))

	got, err := db.GetTranscript(id, h)
	if err != nil {
		t.Fatal(err)
	}
	if got.Source != "user" {
		t.Errorf("source = %q after a correction, want user", got.Source)
	}
	all, err := db.Transcripts(id, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("%d transcripts for one recording — a correction should replace, not append", len(all))
	}
}

func TestTranscriptsAreIsolatedByProfile(t *testing.T) {
	db, mine := transDB(t)
	other, _ := NewProfileID()
	if err := db.SaveProfile(Profile{ID: other, DisplayName: "Dina",
		Equipment: []string{"body weight"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.PutTranscript(mine, "h1", "something private", "model", "en"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.GetTranscript(other, "h1"); !errors.Is(err, ErrNoTranscript) {
		t.Fatal("another profile read the transcript using only its hash")
	}
	if got, _ := db.Transcripts(other, 10); len(got) != 0 {
		t.Fatalf("another profile's listing shows %d transcripts", len(got))
	}
}

func TestMissingTranscriptIsADistinctError(t *testing.T) {
	db, id := transDB(t)
	if _, err := db.GetTranscript(id, "never"); !errors.Is(err, ErrNoTranscript) {
		t.Fatalf("err = %v, want ErrNoTranscript", err)
	}
}
