package storage

import (
	"errors"
	"testing"

	"github.com/rakasatria/sehaty/internal/crypto"
)

// "Delete my data" has to mean every trace of it, or the promise is false.
func TestDeleteProfileRemovesEverything(t *testing.T) {
	db, err := Open(t.TempDir()+"/t.db", testKey)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	c, _ := crypto.New("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")

	must := func(e error) {
		if e != nil {
			t.Fatal(e)
		}
	}
	must(db.SaveProfile(Profile{ID: "gone", DisplayName: "Gone", Equipment: []string{"body weight"}}))
	must(db.SaveProfile(Profile{ID: "stays", DisplayName: "Stays", Equipment: []string{"body weight"}}))
	for _, id := range []string{"gone", "stays"} {
		must(db.LogSet(id, SetEntry{Date: "2026-09-10", Exercise: "push-up", Sets: 3, Reps: 10}))
		must(db.LogWeight(id, WeightEntry{Date: "2026-09-10", WeightKg: 80}))
		must(db.LogFood(id, FoodEntry{Date: "2026-09-10", Item: "rice", Grams: 100, Source: "tkpi:AR001"}))
		must(db.LogCardio(id, CardioEntry{Date: "2026-09-10", Minutes: 20}))
		must(db.LinkIdentity("telegram", "acct-"+id, id))
		_, err := db.PutDocument(c, id, Document{Key: "diet", Title: "P", Kind: "prescription",
			Body: "private", Encrypted: true})
		must(err)
	}

	must(db.DeleteProfile("gone"))

	if _, err := db.GetProfile("gone"); !errors.Is(err, ErrNoProfile) {
		t.Errorf("profile still present: %v", err)
	}
	if s, _ := db.Sets("gone", 30); len(s) != 0 {
		t.Errorf("%d training entries survived", len(s))
	}
	if w, _ := db.Weights("gone", 30); len(w) != 0 {
		t.Errorf("%d weight entries survived", len(w))
	}
	if f, _ := db.Foods("gone", 30); len(f) != 0 {
		t.Errorf("%d food entries survived", len(f))
	}
	if cd, _ := db.Cardio("gone", 30); len(cd) != 0 {
		t.Errorf("%d cardio entries survived", len(cd))
	}
	if docs, _ := db.ListDocuments("gone"); len(docs) != 0 {
		t.Errorf("%d documents survived", len(docs))
	}
	if _, err := db.ResolveIdentity("telegram", "acct-gone"); !errors.Is(err, ErrNoIdentity) {
		t.Error("identity link survived — the account would resolve to a dead profile")
	}

	// The other person must be untouched.
	if _, err := db.GetProfile("stays"); err != nil {
		t.Fatalf("deleting one profile removed another: %v", err)
	}
	if s, _ := db.Sets("stays", 30); len(s) != 1 {
		t.Errorf("other profile has %d training entries, want 1", len(s))
	}
	if docs, _ := db.ListDocuments("stays"); len(docs) != 1 {
		t.Errorf("other profile has %d documents, want 1", len(docs))
	}
	if _, err := db.ResolveIdentity("telegram", "acct-stays"); err != nil {
		t.Errorf("other profile's identity link was removed: %v", err)
	}
}

func TestDeleteProfileOfAnUnknownIDIsAnError(t *testing.T) {
	db, _ := Open(t.TempDir()+"/t.db", testKey)
	defer db.Close()
	if err := db.DeleteProfile("never-existed"); !errors.Is(err, ErrNoProfile) {
		t.Fatalf("err = %v, want ErrNoProfile", err)
	}
}
