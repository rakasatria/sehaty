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
	BMR       int     `json:"bmr_kcal"`
	TDEE      int     `json:"maintenance_kcal"`
	LowKcal   int     `json:"range_low_kcal"`
	HighKcal  int     `json:"range_high_kcal"`
	Activity  float64 `json:"activity_factor"`
	Basis     string  `json:"basis"`
	ForGoal   string  `json:"adjusted_for_goal,omitempty"`
	GoalLow   int     `json:"goal_low_kcal,omitempty"`
	GoalHigh  int     `json:"goal_high_kcal,omitempty"`
	ProteinG  int     `json:"protein_g_per_day,omitempty"`
	Caveat    string  `json:"caveat"`
	Equation  string  `json:"equation"`
	UsedWeigh string  `json:"weight_used"`
}

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
		// ±10% is roughly where 70% of individuals fall against this equation.
		LowKcal:   round(tdee * 0.90),
		HighKcal:  round(tdee * 1.10),
		Equation:  "Mifflin-St Jeor (1990), × activity factor",
		UsedWeigh: fmt.Sprintf("%.1f kg", weight),
		Caveat: "An ESTIMATE, not a target. This equation predicts a group average; " +
			"an individual can sit 20% either side of it. What actually decides the " +
			"number is what happens to the weight log over two or three weeks. If a " +
			"dietitian has given a figure, theirs wins — it is based on you.",
	}

	// A goal-adjusted band, where the goal implies one.
	switch p.Goal {
	case "fat_loss":
		out.ForGoal = "fat_loss — a moderate deficit, roughly 15-20% under maintenance"
		out.GoalLow, out.GoalHigh = round(tdee*0.80), round(tdee*0.85)
	case "hypertrophy":
		out.ForGoal = "hypertrophy — a small surplus, roughly 5-10% over maintenance"
		out.GoalLow, out.GoalHigh = round(tdee*1.05), round(tdee*1.10)
	case "strength":
		out.ForGoal = "strength — around maintenance to a slight surplus"
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
