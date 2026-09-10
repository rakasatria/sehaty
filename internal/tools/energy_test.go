package tools

import (
	"strings"
	"testing"

	"github.com/rakasatria/sehaty/internal/storage"
)

// The equation, checked by hand against the paper.
//
// Mifflin-St Jeor: BMR = 10W + 6.25H − 5A + s, where s is +5 for men and −161 for
// women. For a 34-year-old man, 173 cm, 72.3 kg:
//
//	10(72.3) + 6.25(173) − 5(34) + 5 = 723 + 1081.25 − 170 + 5 = 1639.25 → 1639
//
// Worked out longhand precisely because this is the one number in Sehaty a model
// would otherwise have produced, and a wrong one is indistinguishable from a right
// one by reading it.
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
	if got.BMR != 1639 {
		t.Errorf("BMR = %d, want 1639 (10·72.3 + 6.25·173 − 5·34 + 5)", got.BMR)
	}
	// Four sessions a week is "moderately active", ×1.55.
	if got.Activity != 1.55 {
		t.Errorf("activity factor = %v, want 1.55", got.Activity)
	}
	if want := 2541; got.TDEE != want {
		t.Errorf("maintenance = %d, want %d (1639.25 × 1.55)", got.TDEE, want)
	}
	// A range, never a single figure.
	if got.LowKcal >= got.HighKcal {
		t.Error("the estimate is not a range")
	}
	if got.LowKcal > got.TDEE || got.HighKcal < got.TDEE {
		t.Error("maintenance falls outside its own range")
	}
	// 1.6 g/kg — the well-evidenced part of this.
	if got.ProteinG != 116 {
		t.Errorf("protein = %d g, want 116 (72.3 × 1.6)", got.ProteinG)
	}
	// The caveat is load-bearing, not decoration. It has to say three things: that
	// this is a guess, that the equation was never tested on people like this user,
	// and that the weight log will beat it. Losing any of them turns a wide prior into
	// a confident-looking number.
	for _, want := range []string{"STARTING GUESS", "never been tested on Indonesians",
		"two or three weeks", "dietitian"} {
		if !strings.Contains(got.Caveat, want) {
			t.Errorf("the caveat no longer says %q", want)
		}
	}
	if !got.Provisional {
		t.Error("the estimate does not declare itself provisional")
	}
	// The band must stay wide. ±10% would describe measured RMR in a US cohort, not an
	// estimated TDEE for someone the equation has never been validated on.
	if spread := float64(got.HighKcal-got.LowKcal) / float64(got.TDEE); spread < 0.45 {
		t.Errorf("the range narrowed to %.0f%% — it should be about 50%% wide", spread*100)
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
	if male-female != 166 {
		t.Errorf("male − female = %d, want 166 (+5 against −161)", male-female)
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

// Training frequency changes the whole day's expenditure, so the factor has to
// move with it — and must not run away at the top end.
func TestActivityFactorTracksFrequency(t *testing.T) {
	prev := 0.0
	for _, n := range []int{0, 2, 4, 6, 9} {
		f, basis := activityFor(n)
		if f < prev {
			t.Errorf("%d sessions gave a lower factor than fewer sessions", n)
		}
		if f < 1.2 || f > 1.9 {
			t.Errorf("%d sessions → %v, outside the accepted 1.2–1.9", n, f)
		}
		if basis == "" {
			t.Errorf("%d sessions has no stated basis", n)
		}
		prev = f
	}
}
