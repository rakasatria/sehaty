package tools

import (
	"fmt"
	"math"

	"github.com/rakasatria/sehaty/internal/capability"
	"github.com/rakasatria/sehaty/internal/storage"
)

// Energy estimation.
//
// This is computed HERE, in Go, and never by the model. That is the same rule the
// rest of Sehaty runs on: a language model asked to do arithmetic will produce
// something that looks like an answer, and there is no way to tell a mistaken
// 2,340 from a correct one by reading it. The equation is four multiplications
// and a lookup — deterministic, reproducible, and testable, which is what a
// number in a health record has to be.
//
// Henry CJK, "Basal metabolic rate studies in humans: measurement and development
// of new equations", Public Health Nutrition 2005;8(7A):1133-52. Chosen over
// Mifflin-St Jeor because it removed the Italian bias in the Schofield database
// and raised tropical representation to 38%. It has not been validated for
// pregnancy, for people under 18, or for the very lean or very heavy — which
// is why what comes out of here is a range with its uncertainty attached, and
// never a target. See restingRate for the full reasoning.

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
	Provisional bool `json:"provisional"`
	// Assumed names what this estimate proceeded without, in the words the person gets.
	// An assumption nobody is told about is indistinguishable from a measurement, and
	// the training frequency carries most of the error in this figure.
	Assumed   []string `json:"assumed,omitempty"`
	Equation  string   `json:"equation"`
	UsedWeigh string   `json:"weight_used"`
}

// maxDailyDeficit is the absolute ceiling, in kcal/day, regardless of body size.
//
// It binds only above roughly 2,600 kcal of maintenance; below that a 20% cut is the
// smaller number. Set here rather than inline so the one place it can be argued about is
// obvious.
const maxDailyDeficit = 500.0

// activityFor maps training frequency onto a physical activity level.
//
// The familiar 1.2 / 1.375 / 1.55 / 1.725 / 1.9 ladder is not used here, and the reason is
// that it is not a measurement of anything. Its steps are exactly equidistant to three
// decimals — 0.175 apart, with 1.55 the exact midpoint — which no distribution of human
// activity produces. It is linear interpolation between two borrowed anchors with no
// traceable derivation.
//
// Worse, its floor is wrong by construction. 1.2 comes from a paper describing NON-AMBULANT
// subjects: it is the lower limit of human daily energy expenditure, not a desk job. People
// confined to a respiration chamber — physically unable to leave a sealed room — measure
// 1.40 ± 0.06 and never fall below 1.30. Free-living adults average about 1.70 (women) and
// 1.77 (men).
//
// This matters more than the equation it multiplies. Between-subject variation in PAL is
// about 15% against 8.5% for an estimated BMR, so the activity factor contributes roughly
// three quarters of the total error. One step here is worth more than the entire worst case
// of the resting-rate estimate, which is why the steps are now grounded rather than evenly
// spaced.
func activityFor(sessionsPerWeek int) (float64, string) {
	switch {
	case sessionsPerWeek <= 0:
		// The floor, not zero. Nobody free-living sits below this.
		return 1.40, "sedentary — desk work, little training"
	case sessionsPerWeek <= 2:
		return 1.55, "lightly active — 1-2 sessions a week"
	case sessionsPerWeek <= 4:
		return 1.70, "moderately active — 3-4 sessions a week"
	case sessionsPerWeek <= 6:
		return 1.85, "very active — 5-6 sessions a week"
	default:
		return 2.00, "extremely active — daily training or physical work"
	}
}

// restingRate estimates resting energy expenditure, in kcal/day.
//
// Henry/Oxford (2005) rather than Mifflin-St Jeor, for a specific reason. Mifflin was
// derived on 498 adults in Reno, Nevada, and its own authors wrote that "their clinical
// utility can only be assessed by testing in other populations." The equations everyone
// reaches for instead — FAO/WHO/UNU via Schofield — rest on a database that was 3,388 of
// 7,173 subjects Italian, with 13% from the tropics, and overestimate tropical resting rates
// by around 8%.
//
// Henry rebuilt that database on 10,552 measurements, excluded the Italians entirely, and
// raised tropical representation to 38%. It is the only widely-used equation constructed to
// remove precisely the bias that would otherwise apply here.
//
// It does not solve the problem. There is no published validation of ANY resting-rate
// equation in healthy Indonesian adults — a genuine void rather than a search failure. And
// the mechanism says a weight-and-height equation cannot close the gap: Indonesians carry
// about 4.8 percentage points more body fat than Dutch adults at the same weight, height,
// age and sex, so they carry less fat-free mass, which is what actually drives resting
// expenditure. An equation that cannot see body composition inherits that difference.
//
// Even Henry's own standard error is about 156 kcal/day for young men. Two of those is
// ±310 kcal — the irreducible floor for any weight-based equation, whichever one is chosen.
func restingRate(weightKg float64, age int, sex string) (kcal float64, equation string) {
	female := sex == "female"
	switch {
	case age < 30:
		if female {
			return 13.1*weightKg + 558, "Henry/Oxford (2005), women 18-30"
		}
		return 16.0*weightKg + 545, "Henry/Oxford (2005), men 18-30"
	case age < 60:
		if female {
			return 9.74*weightKg + 694, "Henry/Oxford (2005), women 30-60"
		}
		return 14.2*weightKg + 593, "Henry/Oxford (2005), men 30-60"
	default:
		if female {
			return 10.1*weightKg + 569, "Henry/Oxford (2005), women 60+"
		}
		return 13.5*weightKg + 514, "Henry/Oxford (2005), men 60+"
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

	have := Have(d, p)
	if unmet, blocked := capability.Blocked("estimate_energy", have); blocked {
		return EnergyEstimate{}, fmt.Errorf(
			"cannot estimate without %s — ask for it rather than assuming; an estimate "+
				"built on a guessed input is not an estimate",
			joinAnd(readable(unmet)))
	}

	weight, _ := latestWeight(d, profileID)

	if p.Age < 18 {
		return EnergyEstimate{}, fmt.Errorf(
			"these equations are validated for adults; for someone under 18 this needs " +
				"a paediatric dietitian, not an equation")
	}

	bmr, equation := restingRate(weight, p.Age, p.Sex)
	if p.Sex == "other" {
		// No third-sex coefficient exists in any of these equations. The midpoint is
		// stated as what it is rather than quietly picking one of the two.
		male, _ := restingRate(weight, p.Age, "male")
		female, _ := restingRate(weight, p.Age, "female")
		bmr = (male + female) / 2
		equation += " — midpoint of the two published forms"
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
		Equation:    equation + ", × activity factor — unvalidated for this population",
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

	// Protein. 1.6 g/kg is the widely quoted figure and it is softer than its
	// reputation: it is the point estimate of a meta-regression breakpoint that was
	// NOT statistically significant (p = 0.079), with a confidence interval running
	// 1.03 to 2.20, and the authors themselves wrote that it "may be prudent to
	// recommend ~2.2 g/kg" for anyone trying to maximise.
	//
	// In a deficit the case for more is much stronger. At a 40% deficit, 2.4 g/kg
	// gained 1.2 kg of lean mass where 1.2 g/kg gained 0.1 kg. So the figure rises
	// when someone is cutting, which is exactly when lean mass is at risk.
	out.ProteinG = round(weight * 1.6)
	if p.Goal == "fat_loss" {
		out.ProteinG = round(weight * 2.2)
	}
	for _, gap := range capability.SoftGaps("estimate_energy", have) {
		out.Assumed = append(out.Assumed, gap.Because)
	}
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

// readable turns field names into the words a person would use. "weight_log" is a
// database concept; "a recorded weight" is the thing they are being asked for.
func readable(rs []capability.Requirement) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		switch r.Field {
		case capability.FieldWeightLog:
			out = append(out, "a recorded weight")
		case capability.FieldHeight:
			out = append(out, "height")
		default:
			out = append(out, string(r.Field))
		}
	}
	return out
}
