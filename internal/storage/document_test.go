package storage

import (
	"errors"
	"strings"
	"testing"

	"github.com/rakasatria/sehaty/internal/crypto"
)

func testCipher(t *testing.T) *crypto.Cipher {
	t.Helper()
	k, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	c, err := crypto.New(k)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestPutGetDocumentEncrypted(t *testing.T) {
	db, c := testDB(t), testCipher(t)
	seed(t, db, "raka")
	body := "Pagi: telur putih 3 atau ayam fillet 100g. Malam: tanpa tumis."
	v, err := db.PutDocument(c, "raka", Document{Key: "diet-plan",
		Title: "Nutritionist plan", Kind: "prescription", Body: body, Encrypted: true})
	must(t, err)
	if v != 1 {
		t.Errorf("first version = %d, want 1", v)
	}
	got, err := db.GetDocument(c, "raka", "diet-plan", 0)
	must(t, err)
	if got.Body != body {
		t.Errorf("body round-trip failed: %q", got.Body)
	}
	if !got.Encrypted {
		t.Error("encrypted flag lost")
	}
}

// Satuan Penukar must survive verbatim — 1P is a portion, not a weight, and a store
// that normalises it has destroyed the prescription while appearing to keep it.
func TestPrescriptionStoredVerbatim(t *testing.T) {
	db, c := testDB(t), testCipher(t)
	seed(t, db, "raka")
	body := "Siang: nasi 100g, ayam 120g, tempe 80g, sayuran 1P, minyak 1 sdt"
	_, err := db.PutDocument(c, "raka", Document{Key: "diet-plan", Title: "P",
		Kind: "prescription", Body: body, Encrypted: true})
	must(t, err)
	got, _ := db.GetDocument(c, "raka", "diet-plan", 0)
	if !strings.Contains(got.Body, "1P") || !strings.Contains(got.Body, "1 sdt") {
		t.Errorf("exchange units lost: %q", got.Body)
	}
}

func TestPutDocumentVersions(t *testing.T) {
	db, c := testDB(t), testCipher(t)
	seed(t, db, "raka")
	d := Document{Key: "diet-plan", Title: "v1", Kind: "prescription",
		Body: "old plan", Encrypted: true}
	v1, _ := db.PutDocument(c, "raka", d)
	d.Body, d.Title = "new plan", "v2"
	v2, err := db.PutDocument(c, "raka", d)
	must(t, err)
	if v1 != 1 || v2 != 2 {
		t.Fatalf("versions %d,%d want 1,2", v1, v2)
	}
	latest, _ := db.GetDocument(c, "raka", "diet-plan", 0)
	if latest.Body != "new plan" {
		t.Errorf("latest = %q", latest.Body)
	}
	old, err := db.GetDocument(c, "raka", "diet-plan", 1)
	must(t, err)
	if old.Body != "old plan" {
		t.Errorf("history lost: %q", old.Body)
	}
}

func TestDocumentsIsolatedByProfile(t *testing.T) {
	db, c := testDB(t), testCipher(t)
	seed(t, db, "raka", "other")
	_, err := db.PutDocument(c, "raka", Document{Key: "diet-plan", Title: "t",
		Kind: "prescription", Body: "raka only", Encrypted: true})
	must(t, err)
	if _, err := db.GetDocument(c, "other", "diet-plan", 0); !errors.Is(err, ErrNoDocument) {
		t.Error("other profile reached raka's document")
	}
	docs, _ := db.ListDocuments("other")
	if len(docs) != 0 {
		t.Errorf("other sees %d documents", len(docs))
	}
}

func TestUnencryptedDocumentNeedsNoKey(t *testing.T) {
	db := testDB(t)
	seed(t, db, "raka")
	_, err := db.PutDocument(nil, "raka", Document{Key: "weekly-w37",
		Title: "Weekly", Kind: "report", Body: "3 sessions", Encrypted: false})
	must(t, err)
	got, err := db.GetDocument(nil, "raka", "weekly-w37", 0)
	must(t, err)
	if got.Body != "3 sessions" || got.Encrypted {
		t.Errorf("got %+v", got)
	}
}

func TestEncryptedDocumentWithoutKeyFails(t *testing.T) {
	db, c := testDB(t), testCipher(t)
	seed(t, db, "raka")
	_, err := db.PutDocument(c, "raka", Document{Key: "diet-plan", Title: "t",
		Kind: "prescription", Body: "secret", Encrypted: true})
	must(t, err)
	if _, err := db.GetDocument(nil, "raka", "diet-plan", 0); !errors.Is(err, crypto.ErrNoKey) {
		t.Errorf("err = %v, want ErrNoKey", err)
	}
}
