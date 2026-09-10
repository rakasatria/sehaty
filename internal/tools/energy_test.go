package tools

import (
	"strings"
	"testing"

	"github.com/rakasatria/sehaty/internal/storage"
)

// The equation, checked by hand.
//
// Henry/Oxford (2005), men 30-60: BMR = 14.2W + 593. For 72.3 kg:
//
//	14.2(72.3) + 593 = 1026.66 + 593 = 1619.66 → 1620
//
// Worked longhand precisely because this is the one number in Sehaty a model would
// otherwise have produced, and a wrong one is indistinguishable from a right one by
// reading it.
func TestTheEquationIsTheEquation(t *testing.T) {
	d := testDeps(t)
	p, err := Register(d, "probe", "energy", "Probe", "pw", "pw")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := UpdateProfile(d, UpdateProfileArgs{Profile: p.ID,
		Age: 34, HeightCm: 173, Sex: "male", SessionsPerWeek: 4}); err != nil {
		t.Fatal(err)
	}
	if err := d.DB.LogWeight(p.ID, storage.WeightEntry{Date: "2026-09-10", WeightKg: 72.3}); err != nil {
		t.Fatal(err)
	}

	got, err := EstimateEnergy(d, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.BMR != 1620 {
		t.Errorf("BMR = %d, want 1620 (14.2·72.3 + 593)", got.BMR)
	}
	// Four sessions a week is "moderately active" — and the floor of this ladder is
	// 1.40, not 1.2. Nobody free-living sits below 1.40; people sealed in a
	// respiration chamber measure 1.40 ± 0.06.
	if got.Activity != 1.70 {
		t.Errorf("activity factor = %v, want 1.70", got.Activity)
	}
	if want := 2753; got.TDEE != want {
		t.Errorf("maintenance = %d, want %d (1619.66 × 1.70)", got.TDEE, want)
	}
	if got.LowKcal >= got.HighKcal {
		t.Error("the estimate is not a range")
	}
	// The caveat is load-bearing, not decoration.
	for _, want := range []string{"STARTING GUESS", "never been tested on Indonesians",
		"two or three weeks", "dietitian"} {
		if !strings.Contains(got.Caveat, want) {
			t.Errorf("the caveat no longer says %q", want)
		}
	}
	if !got.Provisional {
		t.Error("the estimate does not declare itself provisional")
	}
	if spread := float64(got.HighKcal-got.LowKcal) / float64(got.TDEE); spread < 0.45 {
		t.Errorf("the range narrowed to %.0f%% — it should be about 50%% wide", spread*100)
	}
}

// No rung of the activity ladder may sit below what a person sealed in a metabolic
// chamber measures. The old ladder started at 1.2 — the published floor for NON-AMBULANT
// subjects — which assigned bedbound expenditure to anyone with a desk job.
func TestNoActivityFactorIsBelowThePhysiologicalFloor(t *testing.T) {
	for _, n := range []int{0, 1, 2, 3, 4, 5, 6, 10} {
		f, basis := activityFor(n)
		if f < 1.40 {
			t.Errorf("%d sessions → %v, below the 1.40 floor measured in a respiration chamber", n, f)
		}
		if f > 2.10 {
			t.Errorf("%d sessions → %v, above what free-living adults sustain", n, f)
		}
		if basis == "" {
			t.Errorf("%d sessions has no stated basis", n)
		}
	}
	// And the steps must not be evenly spaced, because that is the signature of
	// interpolation rather than measurement.
	a, _ := activityFor(0)
	b, _ := activityFor(1)
	c, _ := activityFor(3)
	if (b-a)-(c-b) == 0 && b-a != 0 {
		t.Error("the ladder is evenly spaced again — that is arithmetic, not evidence")
	}
}

// The female coefficient is a different number, not a rounding of the male one.
func TestTheSexCoefficientIsApplied(t *testing.T) {
	d := testDeps(t)
	mk := func(id, sex string) int {
		p, err := Register(d, "probe", id, "Probe", "pw", "pw")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := UpdateProfile(d, UpdateProfileArgs{Profile: p.ID,
			Age: 34, HeightCm: 173, Sex: sex, SessionsPerWeek: 4}); err != nil {
			t.Fatal(err)
		}
		if err := d.DB.LogWeight(p.ID, storage.WeightEntry{Date: "2026-09-10", WeightKg: 72.3}); err != nil {
			t.Fatal(err)
		}
		got, err := EstimateEnergy(d, p.ID)
		if err != nil {
			t.Fatal(err)
		}
		return got.BMR
	}
	male, female := mk("m", "male"), mk("f", "female")
	// Henry/Oxford 30-60: men 14.2W + 593, women 9.74W + 694. At 72.3 kg that is
	// 1619.66 and 1398.20, which round to 1620 and 1398 — a gap of 222 between the
	// reported figures, though the unrounded difference is 221.46. The test compares
	// what is actually shown, so it expects 222.
	//
	// The gap comes from different SLOPES, not a constant offset, which is the point
	// of sex-specific equations and the reason a single one with an adjustment term
	// fits both sexes worse.
	if male-female != 222 {
		t.Errorf("male − female = %d, want 222 (1620 against 1398)", male-female)
	}
}

// THE test. This function has the most obvious incentive in the codebase to
// invent its inputs: it cannot answer without them, and answering is what it is
// for. It must refuse instead, every time, naming what it needs.
func TestItRefusesRatherThanAssumingAnyInput(t *testing.T) {
	d := testDeps(t)

	for name, setup := range map[string]func(string){
		"no age at all": func(id string) {
			_, _ = UpdateProfile(d, UpdateProfileArgs{Profile: id, HeightCm: 173, Sex: "male"})
			_ = d.DB.LogWeight(id, storage.WeightEntry{Date: "2026-09-10", WeightKg: 72.3})
		},
		"no height": func(id string) {
			_, _ = UpdateProfile(d, UpdateProfileArgs{Profile: id, Age: 34, Sex: "male"})
			_ = d.DB.LogWeight(id, storage.WeightEntry{Date: "2026-09-10", WeightKg: 72.3})
		},
		"no sex": func(id string) {
			_, _ = UpdateProfile(d, UpdateProfileArgs{Profile: id, Age: 34, HeightCm: 173})
			_ = d.DB.LogWeight(id, storage.WeightEntry{Date: "2026-09-10", WeightKg: 72.3})
		},
		"never weighed": func(id string) {
			_, _ = UpdateProfile(d, UpdateProfileArgs{Profile: id, Age: 34, HeightCm: 173, Sex: "male"})
		},
	} {
		p, err := Register(d, "probe", "refuse-"+name, "Probe", "pw", "pw")
		if err != nil {
			t.Fatal(err)
		}
		setup(p.ID)

		got, err := EstimateEnergy(d, p.ID)
		if err == nil {
			t.Errorf("%s: produced an estimate anyway — %d kcal", name, got.TDEE)
			continue
		}
		if got.TDEE != 0 || got.BMR != 0 {
			t.Errorf("%s: refused but still returned numbers", name)
		}
	}
}

// The equation is not validated for children, and an equation applied outside its
// population is a number with a citation and no meaning.
func TestItRefusesForSomeoneUnderEighteen(t *testing.T) {
	d := testDeps(t)
	p, _ := Register(d, "probe", "young", "Probe", "pw", "pw")
	_, _ = UpdateProfile(d, UpdateProfileArgs{Profile: p.ID,
		Age: 15, HeightCm: 165, Sex: "male", SessionsPerWeek: 3})
	_ = d.DB.LogWeight(p.ID, storage.WeightEntry{Date: "2026-09-10", WeightKg: 55})

	if _, err := EstimateEnergy(d, p.ID); err == nil {
		t.Fatal("estimated for a 15-year-old")
	} else if !strings.Contains(err.Error(), "paediatric") {
		t.Errorf("refused for the wrong reason: %v", err)
	}
}

// Frequency must still move the factor monotonically — that part of the old test was
// right, and it is kept. Its bounds were not: they enforced 1.2 to 1.9, which is
// exactly the range this file now rejects.
func TestActivityFactorTracksFrequency(t *testing.T) {
	prev := 0.0
	for _, n := range []int{0, 2, 4, 6, 9} {
		f, _ := activityFor(n)
		if f < prev {
			t.Errorf("%d sessions gave a lower factor than fewer sessions", n)
		}
		prev = f
	}
}
