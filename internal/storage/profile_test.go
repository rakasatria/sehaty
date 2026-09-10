package storage

import (
	"errors"
	"testing"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSaveAndGetProfile(t *testing.T) {
	db := testDB(t)
	want := Profile{ID: "raka", Equipment: []string{"body weight", "dumbbell"},
		Goal: "fat_loss", SessionsPerWeek: 3, SessionMinutes: 50,
		Experience: "beginner", MaxDifficulty: 3, Locale: "id"}
	if err := db.SaveProfile(want); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	got, err := db.GetProfile("raka")
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.Goal != want.Goal || got.Locale != want.Locale {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if len(got.Equipment) != 2 || got.Equipment[1] != "dumbbell" {
		t.Errorf("equipment = %v", got.Equipment)
	}
}

func TestGetProfileMissing(t *testing.T) {
	db := testDB(t)
	if _, err := db.GetProfile("nobody"); !errors.Is(err, ErrNoProfile) {
		t.Errorf("err = %v, want ErrNoProfile", err)
	}
}

func TestSaveProfileIsUpsert(t *testing.T) {
	db := testDB(t)
	p := Profile{ID: "raka", Equipment: []string{"body weight"}, Goal: "general",
		SessionsPerWeek: 3, SessionMinutes: 50, Experience: "beginner",
		MaxDifficulty: 3, Locale: "en"}
	if err := db.SaveProfile(p); err != nil {
		t.Fatal(err)
	}
	p.Goal = "fat_loss"
	if err := db.SaveProfile(p); err != nil {
		t.Fatalf("second save: %v", err)
	}
	got, _ := db.GetProfile("raka")
	if got.Goal != "fat_loss" {
		t.Errorf("goal = %q, want fat_loss", got.Goal)
	}
	all, _ := db.ListProfiles()
	if len(all) != 1 {
		t.Errorf("ListProfiles = %d rows, want 1", len(all))
	}
}

func TestValidIDRejectsTraversal(t *testing.T) {
	for _, bad := range []string{"../etc", "a/b", "", "UPPER", "with space"} {
		if ValidID(bad) {
			t.Errorf("ValidID(%q) = true, want false", bad)
		}
	}
	for _, ok := range []string{"raka", "user_2", "a-b"} {
		if !ValidID(ok) {
			t.Errorf("ValidID(%q) = false, want true", ok)
		}
	}
}
