package tools

import (
	"strings"
	"testing"

	"github.com/rakasatria/sehaty/internal/catalog"
	"github.com/rakasatria/sehaty/internal/crypto"
	"github.com/rakasatria/sehaty/internal/food"
	"github.com/rakasatria/sehaty/internal/storage"
)

const testDBKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

func testDeps(t *testing.T) Deps {
	t.Helper()
	db, err := storage.Open(t.TempDir()+"/t.db", testDBKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	cat, err := catalog.Load("../catalog/testdata/exercises.json")
	if err != nil {
		t.Fatal(err)
	}
	c, err := crypto.New("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil {
		t.Fatal(err)
	}
	tbl, err := food.Load("../../data/tkpi/tkpi-2020.json")
	if err != nil {
		t.Fatal(err)
	}
	return Deps{DB: db, Cat: cat, Cipher: c, Food: tbl}
}

func seed(t *testing.T, d Deps, p storage.Profile) {
	t.Helper()
	if err := d.DB.SaveProfile(p); err != nil {
		t.Fatal(err)
	}
}

// THE safety test. A pistol squat is body-weight, so equipment filtering alone lets it
// through — only the difficulty cap stops it. If this ever fails, a beginner is being
// prescribed a movement that can injure them.
func TestBeginnerIsNeverPrescribedAnEliteMovement(t *testing.T) {
	d := testDeps(t)
	seed(t, d, storage.Profile{ID: "novice", Equipment: []string{"body weight"},
		Goal: "general", SessionsPerWeek: 3, SessionMinutes: 45,
		Experience: "beginner", MaxDifficulty: 3})

	plan, err := PlanSession(d, "novice", "full", 45)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range plan.Exercises {
		if e.Difficulty > 3 {
			t.Fatalf("prescribed %q at difficulty %d to a beginner capped at 3",
				e.Name, e.Difficulty)
		}
		if strings.Contains(strings.ToLower(e.Name), "pistol") {
			t.Fatalf("prescribed a pistol squat to a beginner: %q", e.Name)
		}
	}
	if len(plan.Exercises) == 0 {
		t.Fatal("planned nothing at all — the cap is filtering everything out")
	}
}

// find_exercises must obey the same cap. A person who cannot be PRESCRIBED a movement
// should not be handed it through search either.
func TestFindExercisesRespectsTheDifficultyCap(t *testing.T) {
	d := testDeps(t)
	seed(t, d, storage.Profile{ID: "novice", Equipment: []string{"body weight"},
		Goal: "general", MaxDifficulty: 3})

	found, err := FindExercises(d, "novice", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range found {
		if e.Difficulty > 3 {
			t.Fatalf("find_exercises returned %q at difficulty %d", e.Name, e.Difficulty)
		}
	}
}

// Logging a movement the person has no equipment for means either a typo or a plan they
// cannot follow. Accepting it silently corrupts the training history.
func TestLogSetRejectsEquipmentTheProfileLacks(t *testing.T) {
	d := testDeps(t)
	seed(t, d, storage.Profile{ID: "raka", Equipment: []string{"body weight"},
		Goal: "general", MaxDifficulty: 5})

	if _, err := LogSet(d, "raka", "barbell bench press", 3, 8, 60); err == nil {
		t.Fatal("logged a barbell lift for a body-weight-only profile")
	}
	if _, err := LogSet(d, "raka", "push-up", 3, 12, 0); err != nil {
		t.Fatalf("refused a movement the profile CAN do: %v", err)
	}
}

// An empty log is not the same as a zero. Reporting "latest weight 0 kg" to someone who
// has never weighed in is a fabricated number, and the kind an assistant will repeat.
func TestProgressDistinguishesNoDataFromZero(t *testing.T) {
	d := testDeps(t)
	seed(t, d, storage.Profile{ID: "raka", Equipment: []string{"body weight"},
		Goal: "general", SessionsPerWeek: 3, MaxDifficulty: 5})

	p, err := Progress(d, "raka", 14)
	if err != nil {
		t.Fatal(err)
	}
	if p.LatestWeightKg != nil {
		t.Fatalf("reported a latest weight of %v with no weigh-ins recorded", *p.LatestWeightKg)
	}

	if err := d.DB.LogWeight("raka", storage.WeightEntry{Date: today(), WeightKg: 72.3}); err != nil {
		t.Fatal(err)
	}
	p, err = Progress(d, "raka", 14)
	if err != nil {
		t.Fatal(err)
	}
	if p.LatestWeightKg == nil || *p.LatestWeightKg != 72.3 {
		t.Fatalf("after logging 72.3, progress reported %v", p.LatestWeightKg)
	}
}

// A typo in equipment used to be stored happily, leaving the person with zero matching
// exercises and no explanation — indistinguishable from the app being broken.
func TestUpdateProfileRejectsUnknownEquipment(t *testing.T) {
	d := testDeps(t)
	seed(t, d, storage.Profile{ID: "raka", Equipment: []string{"body weight"}, Goal: "general"})

	_, err := UpdateProfile(d, UpdateProfileArgs{Profile: "raka", Equipment: []string{"dumbells"}})
	if err == nil {
		t.Fatal("accepted a misspelled equipment name")
	}
	if !strings.Contains(err.Error(), "dumbells") {
		t.Fatalf("error does not name the offending value: %v", err)
	}
}

// The treadmill is real equipment but appears in no exercise record, because cardio is
// served by cardio_protocol rather than the catalog. Rejecting it would be technically
// correct and practically absurd.
func TestUpdateProfileAcceptsTreadmill(t *testing.T) {
	d := testDeps(t)
	seed(t, d, storage.Profile{ID: "raka", Equipment: []string{"body weight"}, Goal: "general"})

	p, err := UpdateProfile(d, UpdateProfileArgs{Profile: "raka",
		Equipment: []string{"body weight", "treadmill"}})
	if err != nil {
		t.Fatalf("rejected the treadmill: %v", err)
	}
	var found bool
	for _, e := range p.Equipment {
		if e == "treadmill" {
			found = true
		}
	}
	if !found {
		t.Fatal("treadmill was accepted but not stored")
	}
}

// A caller setting only equipment must not blank everything else.
func TestUpdateProfileOnlyChangesWhatWasSupplied(t *testing.T) {
	d := testDeps(t)
	seed(t, d, storage.Profile{ID: "raka", Equipment: []string{"body weight"},
		Goal: "fat_loss", SessionsPerWeek: 4, SessionMinutes: 50,
		Experience: "intermediate", MaxDifficulty: 4})

	p, err := UpdateProfile(d, UpdateProfileArgs{Profile: "raka", Equipment: []string{"dumbbell"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Goal != "fat_loss" || p.SessionsPerWeek != 4 || p.SessionMinutes != 50 ||
		p.Experience != "intermediate" || p.MaxDifficulty != 4 {
		t.Fatalf("an equipment-only update altered other fields: %+v", p)
	}
}

func TestExperienceSetsTheDifficultyCeiling(t *testing.T) {
	d := testDeps(t)
	seed(t, d, storage.Profile{ID: "raka", Equipment: []string{"body weight"},
		Goal: "general", Experience: "beginner", MaxDifficulty: 3})

	p, err := UpdateProfile(d, UpdateProfileArgs{Profile: "raka", Experience: "advanced"})
	if err != nil {
		t.Fatal(err)
	}
	if p.MaxDifficulty != 5 {
		t.Fatalf("advanced experience gave max_difficulty %d, want 5", p.MaxDifficulty)
	}
	// An explicit cap in the same call must win over the one experience implies.
	p, err = UpdateProfile(d, UpdateProfileArgs{Profile: "raka",
		Experience: "advanced", MaxDifficulty: 2})
	if err != nil {
		t.Fatal(err)
	}
	if p.MaxDifficulty != 2 {
		t.Fatalf("explicit max_difficulty was overridden by experience: got %d", p.MaxDifficulty)
	}
}

func TestUpdateProfileRejectsOutOfRangeValues(t *testing.T) {
	d := testDeps(t)
	seed(t, d, storage.Profile{ID: "raka", Equipment: []string{"body weight"}, Goal: "general"})

	for _, a := range []UpdateProfileArgs{
		{Profile: "raka", MaxDifficulty: 9},
		{Profile: "raka", SessionsPerWeek: 40},
		{Profile: "raka", SessionMinutes: 5},
		{Profile: "raka", Goal: "get_swole"},
		{Profile: "raka", Experience: "expert"},
	} {
		if _, err := UpdateProfile(d, a); err == nil {
			t.Errorf("accepted out-of-range or unknown value: %+v", a)
		}
	}
}
