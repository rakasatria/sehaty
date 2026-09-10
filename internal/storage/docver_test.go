package storage

import (
	"errors"
	"testing"

	"github.com/rakasatria/sehaty/internal/crypto"
)

func docDB(t *testing.T) (*DB, *crypto.Cipher) {
	t.Helper()
	db, err := Open(t.TempDir()+"/t.db", testKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.SaveProfile(Profile{ID: "raka", Equipment: []string{"body weight"}}); err != nil {
		t.Fatal(err)
	}
	c, err := crypto.New("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil {
		t.Fatal(err)
	}
	return db, c
}

// expected == 0 means "this document must not exist yet". Two agents both believing they
// are creating it must not silently produce two versions.
func TestPutDocumentIfVersionRejectsCreateWhenItAlreadyExists(t *testing.T) {
	db, c := docDB(t)
	doc := Document{Key: "diet", Title: "Plan", Kind: "prescription", Body: "v1", Encrypted: true}

	if _, err := db.PutDocumentIfVersion(c, "raka", doc, 0); err != nil {
		t.Fatal(err)
	}
	doc.Body = "v2 from a second caller who also thought it was new"
	_, err := db.PutDocumentIfVersion(c, "raka", doc, 0)
	if !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("second create returned %v, want ErrVersionConflict", err)
	}
}

// The guardrail that matters: an agent acting on a copy it read earlier cannot clobber a
// newer version. It gets a conflict and has to re-read.
func TestPutDocumentIfVersionRejectsAStaleWrite(t *testing.T) {
	db, c := docDB(t)
	doc := Document{Key: "diet", Title: "Plan", Kind: "prescription", Body: "v1", Encrypted: true}
	v1, _ := db.PutDocumentIfVersion(c, "raka", doc, 0)

	doc.Body = "v2"
	v2, err := db.PutDocumentIfVersion(c, "raka", doc, v1)
	if err != nil {
		t.Fatal(err)
	}
	if v2 != 2 {
		t.Fatalf("second write produced version %d, want 2", v2)
	}

	doc.Body = "written by someone still holding v1"
	if _, err := db.PutDocumentIfVersion(c, "raka", doc, v1); !errors.Is(err, ErrVersionConflict) {
		t.Fatalf("stale write returned %v, want ErrVersionConflict", err)
	}

	// The head must still be v2 — a rejected write changes nothing.
	got, err := db.GetDocument(c, "raka", "diet", 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "v2" {
		t.Errorf("head body = %q; the rejected write leaked through", got.Body)
	}
}

// A negative expected version means "append, I know what I'm doing".
func TestPutDocumentIfVersionSkipsTheCheckWhenNegative(t *testing.T) {
	db, c := docDB(t)
	doc := Document{Key: "notes", Title: "Notes", Kind: "note", Body: "a", Encrypted: true}
	if _, err := db.PutDocumentIfVersion(c, "raka", doc, -1); err != nil {
		t.Fatal(err)
	}
	doc.Body = "b"
	v, err := db.PutDocumentIfVersion(c, "raka", doc, -1)
	if err != nil {
		t.Fatal(err)
	}
	if v != 2 {
		t.Fatalf("version = %d, want 2", v)
	}
}

// History must list versions without decrypting bodies — reading what exists should not
// require the key, and should not drag every old revision into memory.
func TestDocumentVersionsListsHistoryWithoutBodies(t *testing.T) {
	db, c := docDB(t)
	doc := Document{Key: "diet", Title: "Plan", Kind: "prescription", Body: "v1", Encrypted: true}
	for i, body := range []string{"v1", "v2", "v3"} {
		doc.Body = body
		if _, err := db.PutDocumentIfVersion(c, "raka", doc, i); err != nil {
			t.Fatal(err)
		}
	}
	hist, err := db.DocumentVersions("raka", "diet")
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 3 {
		t.Fatalf("%d versions, want 3", len(hist))
	}
	if hist[0].Version != 3 {
		t.Errorf("history starts at version %d; newest should come first", hist[0].Version)
	}
	for _, h := range hist {
		if h.Body != "" {
			t.Errorf("version %d carries a body in the history listing", h.Version)
		}
		if h.UpdatedAt == "" {
			t.Errorf("version %d has no timestamp", h.Version)
		}
	}
}

func TestDocumentVersionsOfAnUnknownKeyIsEmptyNotAnError(t *testing.T) {
	db, _ := docDB(t)
	hist, err := db.DocumentVersions("raka", "nothing-here")
	if err != nil {
		t.Fatal(err)
	}
	if len(hist) != 0 {
		t.Fatalf("%d versions for a key that does not exist", len(hist))
	}
}
