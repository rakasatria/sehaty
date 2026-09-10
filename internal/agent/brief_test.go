package agent

import (
	"strings"
	"testing"

	"github.com/rakasatria/sehaty/internal/storage"
	"github.com/rakasatria/sehaty/internal/tools"
)

// The brief exists so the model asks for what it does not have — and, just as important,
// stops asking the moment it does.
func TestBriefNamesWhatIsMissingAndStopsWhenItIsNot(t *testing.T) {
	bare := Brief(tools.Deps{}, storage.Profile{DisplayName: "Raka", Goal: "fat_loss"})
	for _, want := range []string{"STILL UNKNOWN", "age", "height", "ask nothing else"} {
		if !contains(bare, want) {
			t.Errorf("a bare profile's brief does not mention %q", want)
		}
	}

	done := storage.Profile{
		DisplayName: "Raka", Goal: "fat_loss", Age: 34, HeightCm: 173, Sex: "male",
		Equipment: []string{"dumbbell", "treadmill"}, Experience: "intermediate",
		Limitations: []string{"left shoulder"}, Allergies: []string{"prawn"},
	}
	// Answered, not merely set: a value written straight into the struct was never put to
	// them as a question, and the assessment tracks the asking, not the field.
	done.MarkAnswered(storage.Assessment...)
	full := Brief(tools.Deps{}, done)
	if contains(full, "STILL UNKNOWN") {
		t.Error("a complete profile is still being interrogated")
	}
	if !contains(full, "Do not ask profile") {
		t.Error("a complete profile's brief does not tell the model to stop asking")
	}
	for _, want := range []string{"34", "173", "left shoulder", "prawn"} {
		if !contains(full, want) {
			t.Errorf("brief omits %q, so the model cannot use it", want)
		}
	}
}

// An empty field must be absent, not present and blank: "Sex: " invites the model to
// treat the blank as an answer it already has.
func TestBriefOmitsFieldsRatherThanShowingThemEmpty(t *testing.T) {
	got := Brief(tools.Deps{}, storage.Profile{DisplayName: "Raka", Goal: "general"})
	for _, unwanted := range []string{"Age:", "Height:", "Sex:", "Food allergies:"} {
		if contains(got, unwanted) {
			t.Errorf("brief carries an empty %q", unwanted)
		}
	}
}

// Equipment first, age late, and the three that read as data harvesting when asked cold
// carry the moment that makes them reasonable instead.
func TestBriefOrdersTheAskAndGatesTheIntrusiveOnes(t *testing.T) {
	got := Brief(tools.Deps{}, storage.Profile{DisplayName: "Raka"})

	first := strings.Index(got, "equipment")
	age := strings.Index(got, "age")
	if first < 0 || age < 0 || first > age {
		t.Error("age is being asked before equipment, which is backwards")
	}

	for field, moment := range map[string]string{
		"height":         "only when a weight has just been logged",
		"food allergies": "only when food is being logged",
		"sex":            "prefer never asking",
	} {
		if !contains(got, moment) {
			t.Errorf("%q is offered without the moment that justifies it", field)
		}
	}
}

// A profile that answered the gated ones must not still be carrying their gates around.
func TestGatedFieldsVanishOnceAnswered(t *testing.T) {
	answered := storage.Profile{
		DisplayName: "Raka", HeightCm: 173, Sex: "male",
		Allergies: []string{"udang"}, DietPreference: "non_vegetarian",
	}
	answered.MarkAnswered("height", "sex", "food allergies", "diet preference")
	got := Brief(tools.Deps{}, answered)
	for _, unwanted := range []string{"only when a weight", "only when food", "prefer never asking"} {
		if contains(got, unwanted) {
			t.Errorf("brief still gates %q after it was answered", unwanted)
		}
	}
}
