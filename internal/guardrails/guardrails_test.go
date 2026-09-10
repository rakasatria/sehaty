package guardrails

import (
	"strings"
	"testing"

	"github.com/rakasatria/sehaty/internal/catalog"
)

// Some situations are not "train around it" situations. Sehaty must decline and say who to
// ask instead — it is not a clinician and must not behave like one.
func TestHighRiskLimitationsRefusePlanningEntirely(t *testing.T) {
	for _, stated := range []string{
		"chest pain when climbing stairs",
		"diagnosed heart condition",
		"I am pregnant",
		"recovering from an eating disorder",
		"fainting spells",
		"severe injury to my spine last month",
		"had surgery two weeks ago",
	} {
		ref, blocked := Screen([]string{stated})
		if !blocked {
			t.Errorf("Screen(%q) allowed planning — this needs a professional", stated)
			continue
		}
		if !strings.Contains(strings.ToLower(ref.Message), "professional") {
			t.Errorf("refusal for %q does not point anywhere useful: %q", stated, ref.Message)
		}
	}
}

// The reference implementation this borrows from is negation-blind: "no chest pain" trips
// its filter. Refusing to help someone who explicitly said they are FINE is a real failure,
// not a safe default.
func TestNegatedTermsDoNotRefuse(t *testing.T) {
	for _, stated := range []string{
		"no chest pain",
		"never had a heart condition",
		"denies fainting",
		"not pregnant",
		"no history of eating disorder",
		"without chest pain",
	} {
		if ref, blocked := Screen([]string{stated}); blocked {
			t.Errorf("Screen(%q) refused on a NEGATED statement (matched %q)", stated, ref.Term)
		}
	}
}

// Ordinary aches must narrow the plan, not end it. Refusing everyone with a sore knee
// would make the tool useless and push people to ignore it.
func TestOrdinaryLimitationsDoNotRefuse(t *testing.T) {
	for _, stated := range []string{"bad knee", "sore left shoulder", "tight lower back"} {
		if _, blocked := Screen([]string{stated}); blocked {
			t.Errorf("Screen(%q) refused outright; it should only narrow the plan", stated)
		}
	}
}

func ex(name, bodyPart, target string) catalog.Exercise {
	return catalog.Exercise{Name: name, BodyPart: bodyPart, Target: target, Equipment: "body weight"}
}

func TestKneeInjuryExcludesKneeLoadingMovements(t *testing.T) {
	lim := []string{"bad knee"}
	for _, e := range []catalog.Exercise{
		ex("bodyweight squat", "upper legs", "quads"),
		ex("walking lunge", "upper legs", "quads"),
		ex("box jump", "upper legs", "quads"),
		ex("lever leg extension", "upper legs", "quads"),
	} {
		if excluded, _ := Excluded(lim, e); !excluded {
			t.Errorf("%q was allowed for a bad knee", e.Name)
		}
	}
	// Upper body must survive — a knee problem is not a reason to skip pressing.
	for _, e := range []catalog.Exercise{
		ex("push-up", "chest", "pectorals"),
		ex("dumbbell row", "back", "upper back"),
	} {
		if excluded, why := Excluded(lim, e); excluded {
			t.Errorf("%q was excluded for a bad knee (%s) — too aggressive", e.Name, why)
		}
	}
}

func TestShoulderInjuryExcludesOverheadWork(t *testing.T) {
	lim := []string{"rotator cuff injury"}
	for _, e := range []catalog.Exercise{
		ex("barbell overhead press", "shoulders", "delts"),
		ex("dumbbell lateral raise", "shoulders", "delts"),
		ex("chest dip", "chest", "pectorals"),
	} {
		if excluded, _ := Excluded(lim, e); !excluded {
			t.Errorf("%q was allowed for a rotator cuff injury", e.Name)
		}
	}
	if excluded, why := Excluded(lim, ex("bodyweight squat", "upper legs", "quads")); excluded {
		t.Errorf("squat excluded for a shoulder injury (%s)", why)
	}
}

func TestLowerBackExcludesSpinalLoading(t *testing.T) {
	lim := []string{"herniated disc in my lower back"}
	for _, e := range []catalog.Exercise{
		ex("barbell deadlift", "upper legs", "glutes"),
		ex("barbell good morning", "upper legs", "hamstrings"),
		ex("sit-up", "waist", "abs"),
	} {
		if excluded, _ := Excluded(lim, e); !excluded {
			t.Errorf("%q was allowed with a herniated disc", e.Name)
		}
	}
}

func TestNoLimitationsExcludesNothing(t *testing.T) {
	for _, lim := range [][]string{nil, {}, {""}} {
		if excluded, why := Excluded(lim, ex("barbell deadlift", "upper legs", "glutes")); excluded {
			t.Errorf("excluded a deadlift with no limitations stated (%s)", why)
		}
	}
}

// The reason must name the limitation, so a person can see WHY something vanished from
// their plan rather than assuming the app is broken.
func TestExclusionExplainsItself(t *testing.T) {
	_, why := Excluded([]string{"bad knee"}, ex("bodyweight squat", "upper legs", "quads"))
	if why == "" {
		t.Fatal("exclusion gave no reason")
	}
	if !strings.Contains(strings.ToLower(why), "knee") {
		t.Fatalf("reason %q does not mention the limitation it came from", why)
	}
}
