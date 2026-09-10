package catalog

import "testing"

func load(t *testing.T) *Catalog {
	t.Helper()
	c, err := Load("testdata/exercises.json")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestForFiltersByEquipment(t *testing.T) {
	for _, e := range load(t).For([]string{"body weight"}, 5) {
		if e.Equipment != "body weight" {
			t.Errorf("returned %q needing %q", e.Name, e.Equipment)
		}
	}
	if n := len(load(t).For([]string{"body weight"}, 5)); n != 3 {
		t.Errorf("got %d bodyweight exercises, want 3", n)
	}
}

// The prototype prescribed a pistol squat and korean dips to a beginner on its first
// run. This is the test that stops that happening again.
func TestForRespectsMaxDifficulty(t *testing.T) {
	got := load(t).For([]string{"body weight", "dumbbell"}, 3)
	if len(got) == 0 {
		t.Fatal("no exercises returned at maxDifficulty 3")
	}
	for _, e := range got {
		switch e.Name {
		case "single leg squat (pistol)", "korean dips":
			t.Errorf("%q (difficulty %d) returned at maxDifficulty 3", e.Name, e.Difficulty)
		}
		if e.Difficulty > 3 {
			t.Errorf("%q has difficulty %d > 3", e.Name, e.Difficulty)
		}
	}
}

// An unrated exercise must be excluded from a beginner's plan, not included by default.
func TestUnratedFailsSafe(t *testing.T) {
	c := load(t)
	if c.Rated() != c.Count() {
		t.Logf("%d of %d rated", c.Rated(), c.Count())
	}
	if unratedDifficulty <= 3 {
		t.Fatal("unratedDifficulty must exceed a beginner cap, or unknowns get prescribed")
	}
}

func TestEquipmentMatchIsCaseInsensitive(t *testing.T) {
	if len(load(t).For([]string{"BODY WEIGHT"}, 5)) != 3 {
		t.Error("equipment match should be case-insensitive")
	}
}

func TestByName(t *testing.T) {
	c := load(t)
	e, ok := c.ByName("Goblet Squat")
	if !ok {
		t.Fatal("ByName should be case-insensitive")
	}
	if e.Equipment != "dumbbell" || e.Difficulty != 2 {
		t.Errorf("got %+v", e)
	}
	if _, ok := c.ByName("nonexistent"); ok {
		t.Error("unknown exercise reported as found")
	}
}
