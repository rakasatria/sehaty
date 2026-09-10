package agent

import (
	"strings"
	"testing"

	"github.com/rakasatria/sehaty/internal/storage"
	"github.com/rakasatria/sehaty/internal/tools"
)

// The brief has two modes, and which one it is in changes the rules the prompt
// applies. Getting the mode wrong is worse than getting a question wrong: it either
// interrogates somebody who is settled, or leaves a new person un-assessed.

// A brand-new person is in a consultation, and the energy inputs lead — because the
// estimate is what the intake is for.
func TestABareProfileIsAConsultation(t *testing.T) {
	got := Brief(tools.Deps{}, storage.Profile{DisplayName: "Raka"})

	for _, want := range []string{
		"FIRST CONSULTATION",
		"Still needed before you can estimate their energy",
		"a current weight",
		"Ask the NEXT question as soon as they answer",
		"offer to build their plan",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("a bare profile's brief does not say %q", want)
		}
	}
	if strings.Contains(got, "CONSULTATION COMPLETE") {
		t.Error("a profile that knows nothing was called complete")
	}
}

// The intake list itself keeps the considered order: equipment is cheap to answer,
// age is not.
func TestTheUnknownListKeepsItsOrder(t *testing.T) {
	got := Brief(tools.Deps{}, storage.Profile{DisplayName: "Raka"})
	list := got[strings.Index(got, "STILL UNKNOWN"):]

	if strings.Index(list, "equipment") > strings.Index(list, "age") {
		t.Error("age is listed before equipment, which is backwards")
	}
	if i := strings.Index(list, "sex"); i < 0 || i < strings.Index(list, "height") {
		t.Error("sex is not last")
	}
}

// Someone who has answered everything must not be asked anything.
func TestAnAnsweredProfileIsComplete(t *testing.T) {
	done := storage.Profile{
		DisplayName: "Raka", Goal: "fat_loss", Age: 34, HeightCm: 173, Sex: "male",
		Equipment: []string{"dumbbell"}, Experience: "intermediate",
		Limitations: []string{"left shoulder"}, Allergies: []string{"prawn"},
	}
	done.MarkAnswered(storage.Assessment...)

	// Deps has no DB here, so no weight can be found — which is itself the point:
	// the consultation is not over until there is one, because the estimate needs it.
	got := Brief(tools.Deps{}, done)
	if strings.Contains(got, "CONSULTATION COMPLETE") {
		t.Error("called complete while the person has never been weighed")
	}
	if !strings.Contains(got, "never been weighed") {
		t.Error("does not ask for the weight it still needs")
	}
}

// An empty field must be absent, not present and blank: "Sex: " invites the model to
// treat the blank as an answer it already has.
func TestBriefOmitsFieldsRatherThanShowingThemEmpty(t *testing.T) {
	got := Brief(tools.Deps{}, storage.Profile{DisplayName: "Raka", Goal: "general"})
	for _, unwanted := range []string{"Age:", "Height:", "Sex:", "Food allergies:"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("brief carries an empty %q", unwanted)
		}
	}
}

// Known detail must reach the model, or it cannot use any of it.
func TestKnownDetailIsPassedOn(t *testing.T) {
	got := Brief(tools.Deps{}, storage.Profile{
		DisplayName: "Raka", Goal: "fat_loss", Age: 34, HeightCm: 173, Sex: "male",
		Equipment: []string{"dumbbell", "treadmill"}, Limitations: []string{"left shoulder"},
		Allergies: []string{"prawn"},
	})
	for _, want := range []string{"34", "173", "left shoulder", "prawn", "dumbbell"} {
		if !strings.Contains(got, want) {
			t.Errorf("brief omits %q, so the model cannot use it", want)
		}
	}
}
