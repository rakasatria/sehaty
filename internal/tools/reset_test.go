package tools

import (
	"testing"

	"github.com/rakasatria/sehaty/internal/storage"
)

// Resetting the assessment must clear the answers and NOTHING else.
//
// This is the test that makes the tool safe to hand to a language model. If a reset could
// take a weight log with it, "bisa delete semua?" would become a way to lose months of
// data to a misread sentence.
func TestResetClearsAnswersAndKeepsEveryLoggedThing(t *testing.T) {
	d := testDeps(t)
	p, err := Register(d, "probe", "reset-1", "Probe", "pw", "pw")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := UpdateProfile(d, UpdateProfileArgs{Profile: p.ID,
		Equipment: []string{"dumbbell"}, Goal: "fat_loss", Experience: "advanced",
		Age: 34, HeightCm: 173, Sex: "male", Allergies: []string{"udang"},
		Dislikes: []string{"petai"}, DietNotes: "puasa senin kamis",
		Limitations: []string{"bahu kiri"}}); err != nil {
		t.Fatal(err)
	}
	if err := d.DB.LogWeight(p.ID, storage.WeightEntry{Date: "2026-09-10", WeightKg: 72.3}); err != nil {
		t.Fatal(err)
	}

	after, err := ResetAssessment(d, p.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Every answer gone.
	if after.Age != 0 || after.HeightCm != 0 || after.Sex != "" {
		t.Errorf("personal detail survived the reset: %+v", after)
	}
	if len(after.Allergies) != 0 || len(after.Dislikes) != 0 || after.DietNotes != "" {
		t.Errorf("food detail survived the reset: %+v", after)
	}
	if len(after.Limitations) != 0 {
		t.Error("injuries survived the reset")
	}
	if after.Goal != "general" || after.Experience != "beginner" {
		t.Errorf("goal/experience not back to their starting state: %+v", after)
	}
	if len(after.Equipment) != 1 || after.Equipment[0] != "body weight" {
		t.Errorf("equipment not back to its starting state: %v", after.Equipment)
	}

	// Identity kept: this is not a new person.
	if after.ID != p.ID || after.DisplayName != "Probe" {
		t.Errorf("reset changed who they are: %+v", after)
	}
	if got, err := d.DB.ResolveIdentity("probe", "reset-1"); err != nil || got != p.ID {
		t.Errorf("the Telegram account was unlinked by a reset: %v %v", got, err)
	}

	// And the health record is untouched. This is the assertion that matters.
	pr, err := Progress(d, p.ID, 30)
	if err != nil {
		t.Fatal(err)
	}
	if pr.LatestWeightKg == nil || *pr.LatestWeightKg != 72.3 {
		t.Fatalf("the weight log did not survive the reset: %v", pr.LatestWeightKg)
	}

	// Reset again from the already-clean state: must not error or lose the weight.
	if _, err := ResetAssessment(d, p.ID); err != nil {
		t.Fatalf("resetting twice failed: %v", err)
	}
	if pr, _ := Progress(d, p.ID, 30); pr.LatestWeightKg == nil {
		t.Error("a second reset took the weight log")
	}
}

// A reset must refuse an id that is not a real profile rather than creating one.
func TestResetRefusesAnUnknownProfile(t *testing.T) {
	if _, err := ResetAssessment(testDeps(t), "nobodyhasthisid00000"); err == nil {
		t.Fatal("reset invented a profile")
	}
}
