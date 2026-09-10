package storage

import "testing"

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func seed(t *testing.T, db *DB, ids ...string) {
	t.Helper()
	for _, id := range ids {
		must(t, db.SaveProfile(Profile{ID: id, Equipment: []string{"body weight"},
			Goal: "general", SessionsPerWeek: 3, SessionMinutes: 50,
			Experience: "beginner", MaxDifficulty: 3, Locale: "en"}))
	}
}

func TestLogSetRoundTrip(t *testing.T) {
	db := testDB(t)
	seed(t, db, "raka")
	e := SetEntry{Date: "2026-09-10", Exercise: "goblet squat", BodyPart: "upper legs",
		Sets: 3, Reps: 12, WeightKg: 20, VolumeKg: 720}
	must(t, db.LogSet("raka", e))
	got, err := db.Sets("raka", 3650)
	must(t, err)
	if len(got) != 1 || got[0].Exercise != "goblet squat" || got[0].VolumeKg != 720 {
		t.Errorf("got %+v", got)
	}
}

// The isolation guarantee. If this ever fails, one person is reading another's
// health data — the single worst bug this system can have.
func TestLogsAreIsolatedByProfile(t *testing.T) {
	db := testDB(t)
	seed(t, db, "raka", "other")
	must(t, db.LogSet("raka", SetEntry{Date: "2026-09-10", Exercise: "push-up",
		Sets: 3, Reps: 10}))
	must(t, db.LogWeight("raka", WeightEntry{Date: "2026-09-10", WeightKg: 85}))
	must(t, db.LogSet("other", SetEntry{Date: "2026-09-10", Exercise: "burpee",
		Sets: 1, Reps: 5}))

	sets, _ := db.Sets("raka", 3650)
	if len(sets) != 1 || sets[0].Exercise != "push-up" {
		t.Errorf("raka sets leaked: %+v", sets)
	}
	otherSets, _ := db.Sets("other", 3650)
	if len(otherSets) != 1 || otherSets[0].Exercise != "burpee" {
		t.Errorf("other sets wrong: %+v", otherSets)
	}
	otherWeights, _ := db.Weights("other", 3650)
	if len(otherWeights) != 0 {
		t.Errorf("other saw raka's weight: %+v", otherWeights)
	}
}

func TestSinceDaysExcludesOlder(t *testing.T) {
	db := testDB(t)
	seed(t, db, "raka")
	must(t, db.LogSet("raka", SetEntry{Date: "2020-01-01", Exercise: "push-up",
		Sets: 1, Reps: 1}))
	got, _ := db.Sets("raka", 7)
	if len(got) != 0 {
		t.Errorf("old row returned: %+v", got)
	}
}

func TestCardioFoodWeightRoundTrip(t *testing.T) {
	db := testDB(t)
	seed(t, db, "raka")
	must(t, db.LogCardio("raka", CardioEntry{Date: "2026-09-10", Minutes: 35,
		SpeedKmh: 5.5, InclinePct: 6}))
	must(t, db.LogFood("raka", FoodEntry{Date: "2026-09-10", Item: "nasi", Grams: 100,
		Kcal: 130, Source: "openfoodfacts"}))
	must(t, db.LogWeight("raka", WeightEntry{Date: "2026-09-10", WeightKg: 85}))

	c, _ := db.Cardio("raka", 3650)
	f, _ := db.Foods("raka", 3650)
	w, _ := db.Weights("raka", 3650)
	if len(c) != 1 || c[0].Minutes != 35 {
		t.Errorf("cardio: %+v", c)
	}
	if len(f) != 1 || f[0].Source != "openfoodfacts" {
		t.Errorf("food: %+v", f)
	}
	if len(w) != 1 || w[0].WeightKg != 85 {
		t.Errorf("weight: %+v", w)
	}
}
