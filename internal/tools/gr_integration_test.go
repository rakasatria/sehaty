package tools

import (
	"strings"
	"testing"

	"github.com/rakasatria/sehaty/internal/storage"
)

// A high-risk profile must not receive a plan at all — not a reduced one, not a gentle one.
func TestPlanSessionRefusesAHighRiskProfile(t *testing.T) {
	d := testDeps(t)
	seed(t, d, storage.Profile{ID: "atrisk", Equipment: []string{"body weight", "dumbbell"},
		Goal: "general", SessionsPerWeek: 3, SessionMinutes: 45, MaxDifficulty: 5,
		Limitations: []string{"chest pain when climbing stairs"}})

	_, err := PlanSession(d, "atrisk", "full", 45)
	if err == nil {
		t.Fatal("built a training plan for someone reporting chest pain")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "professional") {
		t.Fatalf("refusal does not point to a professional: %v", err)
	}
}

// ...but an ordinary injury must narrow the plan, not end it.
func TestPlanSessionExcludesContraindicatedMovementsAndSaysSo(t *testing.T) {
	d := testDeps(t)
	seed(t, d, storage.Profile{ID: "raka", Equipment: []string{"body weight", "dumbbell"},
		Goal: "general", SessionsPerWeek: 3, SessionMinutes: 45, MaxDifficulty: 5,
		Limitations: []string{"bad knee"}})

	plan, err := PlanSession(d, "raka", "full", 45)
	if err != nil {
		t.Fatalf("refused to plan for an ordinary knee complaint: %v", err)
	}
	for _, e := range plan.Exercises {
		n := strings.ToLower(e.Name)
		if strings.Contains(n, "squat") || strings.Contains(n, "lunge") {
			t.Errorf("prescribed %q to someone with a bad knee", e.Name)
		}
	}
	ex := plan.ExcludedForSafety
	if ex == nil || ex.Count == 0 {
		t.Fatal("nothing reported as excluded; the person cannot tell why movements vanished")
	}
	// A summary, not a dump: hundreds of entries for a seven-exercise plan is noise.
	if len(ex.Examples) > 5 {
		t.Errorf("summary listed %d examples; it should stay short", len(ex.Examples))
	}
	for _, why := range ex.Reasons {
		if !strings.Contains(strings.ToLower(why), "knee") {
			t.Errorf("reason %q does not cite the limitation it came from", why)
		}
	}
}

// Search must obey the same limits as prescription, or it becomes the way around them.
func TestFindExercisesRespectsLimitations(t *testing.T) {
	d := testDeps(t)
	seed(t, d, storage.Profile{ID: "raka", Equipment: []string{"body weight", "dumbbell"},
		Goal: "general", MaxDifficulty: 5, Limitations: []string{"bad knee"}})

	found, err := FindExercises(d, "raka", "", 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range found {
		if strings.Contains(strings.ToLower(e.Name), "squat") {
			t.Errorf("find_exercises offered %q to someone with a bad knee", e.Name)
		}
	}
}

// Recovering from an injury has to be expressible, so the list must be clearable.
func TestUpdateProfileStoresAndClearsLimitations(t *testing.T) {
	d := testDeps(t)
	seed(t, d, storage.Profile{ID: "raka", Equipment: []string{"body weight"},
		Goal: "general", MaxDifficulty: 5})

	p, err := UpdateProfile(d, UpdateProfileArgs{Profile: "raka",
		Limitations: []string{"bad knee", "  ", "sore shoulder"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Limitations) != 2 {
		t.Fatalf("stored %v; blank entries should be dropped", p.Limitations)
	}

	// Persisted, not just returned.
	reloaded, err := d.DB.GetProfile("raka")
	if err != nil {
		t.Fatal(err)
	}
	if len(reloaded.Limitations) != 2 {
		t.Fatalf("limitations did not survive a round trip: %v", reloaded.Limitations)
	}

	p, err = UpdateProfile(d, UpdateProfileArgs{Profile: "raka", Limitations: []string{}})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Limitations) != 0 {
		t.Fatalf("could not clear limitations: %v — someone who has recovered stays flagged", p.Limitations)
	}
}

// Omitting the field must not wipe it.
func TestUpdateProfileWithoutLimitationsLeavesThemAlone(t *testing.T) {
	d := testDeps(t)
	seed(t, d, storage.Profile{ID: "raka", Equipment: []string{"body weight"},
		Goal: "general", MaxDifficulty: 5, Limitations: []string{"bad knee"}})

	p, err := UpdateProfile(d, UpdateProfileArgs{Profile: "raka", Goal: "fat_loss"})
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Limitations) != 1 || p.Limitations[0] != "bad knee" {
		t.Fatalf("a goal-only update altered limitations: %v", p.Limitations)
	}
}
