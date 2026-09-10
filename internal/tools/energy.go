package tools

import (
	"fmt"
	"math"

	"github.com/rakasatria/sehaty/internal/storage"
)

// Energy estimation.
//
// This is computed HERE, in Go, and never by the model. That is the same rule the
// rest of Sehaty runs on: a language model asked to do arithmetic will produce
// something that looks like an answer, and there is no way to tell a mistaken
// 2,340 from a correct one by reading it. Mifflin-St Jeor is four multiplications
// and a lookup — deterministic, reproducible, and testable, which is what a
// number in a health record has to be.
//
// The equation is Mifflin MD, St Jeor ST, et al., "A new predictive equation for
// resting energy expenditure in healthy individuals", Am J Clin Nutr 1990;51:241-7.
// It is the one most dietetic bodies use for adults, and it is a POPULATION
// equation: it predicts a group mean. For an individual, roughly 70% of people
// fall within ±10% of it and about 95% within ±20%, and it has not been validated
// for pregnancy, for people under 18, or for the very lean or very heavy — which
// is why what comes out of here is a range with its uncertainty attached, and
// never a target.

// EnergyEstimate is a range, deliberately.
//
// A single figure invites someone to eat to it exactly. The band is the honest
// shape of the answer.
type EnergyEstimate struct {
	BMR      int     `json:"bmr_kcal"`
	TDEE     int     `json:"maintenance_kcal"`
	LowKcal  int     `json:"range_low_kcal"`
	HighKcal int     `json:"range_high_kcal"`
	Activity float64 `json:"activity_factor"`
	Basis    string  `json:"basis"`
	ForGoal  string  `json:"adjusted_for_goal,omitempty"`
	GoalLow  int     `json:"goal_low_kcal,omitempty"`
	GoalHigh int     `json:"goal_high_kcal,omitempty"`
	ProteinG int     `json:"protein_g_per_day,omitempty"`
	Caveat   string  `json:"caveat"`
	// Provisional is always true today, and is here so the client can say so and so
	// that a future self-calibrated figure can say it is NOT. The equation is a prior;
	// the weight log is the evidence.
	Provisional bool   `json:"provisional"`
	Equation    string `json:"equation"`
	UsedWeigh   string `json:"weight_used"`
}

// maxDailyDeficit is the absolute ceiling, in kcal/day, regardless of body size.
//
// It binds only above roughly 2,600 kcal of maintenance; below that a 20% cut is the
// smaller number. Set here rather than inline so the one place it can be argued about is
// obvious.
const maxDailyDeficit = 500.0

// activityFor maps training frequency onto the standard multipliers. Someone who
// trains four times a week and sits down the rest of the time is not "very
// active"; the factor describes the whole day, not the hour in the gym.
func activityFor(sessionsPerWeek int) (float64, string) {
	switch {
	case sessionsPerWeek <= 0:
		return 1.2, "sedentary — little or no training"
	case sessionsPerWeek <= 2:
		return 1.375, "lightly active — 1-2 sessions a week"
	case sessionsPerWeek <= 4:
		return 1.55, "moderately active — 3-4 sessions a week"
	case sessionsPerWeek <= 6:
		return 1.725, "very active — 5-6 sessions a week"
	default:
		return 1.9, "extremely active — daily training or physical work"
	}
}

// EstimateEnergy computes a maintenance range from what is on file.
//
// It refuses rather than guessing at anything missing. An estimate built on an
// assumed age or an assumed weight is not an estimate, it is a number with a
// story attached, and this is the one function in Sehaty with the most obvious
// incentive to fabricate its inputs.
func EstimateEnergy(d Deps, profileID string) (EnergyEstimate, error) {
	p, err := requireProfile(d, profileID)
	if err != nil {
		return EnergyEstimate{}, err
	}

	var missing []string
	if p.Age == 0 {
		missing = append(missing, "age")
	}
	if p.HeightCm == 0 {
		missing = append(missing, "height")
	}
	if p.Sex == "" {
		missing = append(missing, "sex")
	}

	weight, weighed := latestWeight(d, profileID)
	if !weighed {
		missing = append(missing, "a recorded weight")
	}
	if len(missing) > 0 {
		return EnergyEstimate{}, fmt.Errorf(
			"cannot estimate without %s — ask for it rather than assuming; an estimate "+
				"built on a guessed input is not an estimate", joinAnd(missing))
	}
	if p.Age < 18 {
		return EnergyEstimate{}, fmt.Errorf(
			"Mifflin-St Jeor is validated for adults; for someone under 18 this needs a " +
				"paediatric dietitian, not an equation")
	}

	// Mifflin-St Jeor, resting energy expenditure in kcal/day.
	bmr := 10*weight + 6.25*float64(p.HeightCm) - 5*float64(p.Age)
	switch p.Sex {
	case "male":
		bmr += 5
	case "female":
		bmr -= 161
	default:
		// No third-sex coefficient exists in the literature. The midpoint is
		// stated as what it is rather than quietly picking one of the two.
		bmr -= 78
	}

	factor, basis := activityFor(p.SessionsPerWeek)
	tdee := bmr * factor

	out := EnergyEstimate{
		BMR:      round(bmr),
		TDEE:     round(tdee),
		Activity: factor,
		Basis:    basis,
		// ±25%, and the width is the honest part.
		//
		// The familiar "±10% covers 70%" figure describes MEASURED RMR in US-like
		// populations. This is an ESTIMATED TDEE — a predicted BMR multiplied by a
		// guessed activity factor — which compounds two errors: RMR SD around 10%, PAL
		// misclassification around 12%. That puts ±10% at roughly 55-60% coverage.
		//
		// And no RMR equation has ever been validated in an Indonesian or Malay adult
		// cohort. The nearest data is Korean (69% within ±10%) and Chinese (17.5-59%
		// depending on equation). The reason is structural rather than incidental:
		// measured REE differs between Asian and white adults by about 16% in absolute
		// terms and NOT AT ALL once adjusted for fat-free mass. An equation built from
		// weight, height, age and sex cannot see body composition, so it inherits that
		// difference and carries it. The direction of bias is inconsistent across Asian
		// cohorts, so there is no constant to correct by either.
		LowKcal:     round(tdee * 0.75),
		HighKcal:    round(tdee * 1.25),
		Equation:    "Mifflin-St Jeor (1990), × activity factor — unvalidated for this population",
		Provisional: true,
		UsedWeigh:   fmt.Sprintf("%.1f kg", weight),
		Caveat: "A STARTING GUESS, not a target and not a measurement. This equation was " +
			"built on 498 American adults and has never been tested on Indonesians, so " +
			"the range around it is wide on purpose. Its real job is to be replaced: " +
			"after two or three weeks of logged weight, what actually happened to that " +
			"weight is a far better number than any equation, because a person's energy " +
			"use is very stable over time even though it differs a lot between people. " +
			"If a dietitian has given a figure, theirs wins — it was built on you.",
	}

	// A goal-adjusted band, where the goal implies one.
	switch p.Goal {
	case "fat_loss":
		// Two rules exist in the literature and they disagree above roughly 2,600 kcal:
		// a percentage of maintenance, and an absolute ceiling near 500 kcal/day. They
		// coincide at the single best data point — a trial whose slower arm ran ~469
		// kcal/day with 1.6 g/kg protein and four lifting sessions a week and GAINED
		// lean mass — which is why the question has stayed unsettled.
		//
		// Above that crossover the absolute ceiling is extrapolated from a pool of
		// sedentary, untrained people around sixty with uncontrolled protein intake, so
		// we take the smaller of the two rather than the more permissive one. Protein
		// and training dose appear to matter more than the exact percentage anyway.
		deficit := math.Min(tdee*0.20, maxDailyDeficit)
		out.ForGoal = "fat_loss — a moderate deficit"
		out.GoalLow, out.GoalHigh = round(tdee-deficit), round(tdee-deficit*0.75)
	case "hypertrophy":
		out.ForGoal = "hypertrophy — a small surplus, roughly 5-10% over maintenance"
		out.GoalLow, out.GoalHigh = round(tdee*1.05), round(tdee*1.10)
	case "strength":
		// Strength is largely insensitive to deficit size in both of the trials that
		// examined it — squat and bench held up in fast and slow arms alike. So the
		// energy argument matters much less for someone chasing strength than it does
		// for someone chasing mass, and this band is deliberately loose.
		out.ForGoal = "strength — around maintenance"
		out.GoalLow, out.GoalHigh = round(tdee), round(tdee*1.05)
	}

	// Protein, which is the part of this that is actually well evidenced: 1.6-2.2
	// g/kg for someone training with resistance. The lower end of the range is
	// used, since the upper end is where returns have flattened.
	out.ProteinG = round(weight * 1.6)
	return out, nil
}

func latestWeight(d Deps, profileID string) (float64, bool) {
	ws, err := d.DB.Weights(profileID, 3650)
	if err != nil || len(ws) == 0 {
		return 0, false
	}
	return ws[len(ws)-1].WeightKg, true
}

func round(v float64) int { return int(math.Round(v)) }

func joinAnd(items []string) string {
	switch len(items) {
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	default:
		out := ""
		for i, s := range items[:len(items)-1] {
			if i > 0 {
				out += ", "
			}
			out += s
		}
		return out + " and " + items[len(items)-1]
	}
}

var _ = storage.Profile{}
