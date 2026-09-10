package agent

import (
	"strings"
	"testing"

	"github.com/rakasatria/sehaty/internal/capability"
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
	done.MarkAnswered(capability.Names(capability.AskOrder())...)

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

// Every question in the brief arrives with the reason it is being asked. A reason
// derived from the tool that needs the field is true by construction; a reason the model
// composes is only as true as the sentence it happens to produce.
func TestEveryUnknownCarriesItsReason(t *testing.T) {
	got := Brief(tools.Deps{}, storage.Profile{DisplayName: "Raka"})
	list := got[strings.Index(got, "STILL UNKNOWN"):]

	for _, want := range []string{
		"only uses things you actually have",
		"pitched where you are",
		"aggravates it",
		"Recovery and pacing shift with age",
	} {
		if !strings.Contains(list, want) {
			t.Errorf("the unknown list does not carry the reason %q", want)
		}
	}
}

// The gate on the intrusive questions still applies outside the first consultation, and
// is still relaxed inside it.
func TestGatedQuestionsKeepTheirMoment(t *testing.T) {
	got := Brief(tools.Deps{}, storage.Profile{DisplayName: "Raka"})
	if !strings.Contains(got, "only when a weight has just been logged") {
		t.Error("height lost the moment it belongs to")
	}
	if !strings.Contains(got, "the moment-gating above is relaxed") {
		t.Error("the first consultation no longer relaxes the gating")
	}
}

// The reason the intake exists is the estimate, and what it is still short of comes
// from the tool itself rather than a second list kept beside it.
func TestTheEnergyInputsComeFromTheTool(t *testing.T) {
	got := Brief(tools.Deps{}, storage.Profile{DisplayName: "Raka"})
	if !strings.Contains(got, "Still needed before you can estimate their energy") {
		t.Fatal("the brief no longer leads with the energy inputs")
	}
	head := got[strings.Index(got, "Still needed"):]
	if i := strings.Index(head, "\n"); i > 0 {
		head = head[:i]
	}
	for _, want := range []string{"age", "height", "sex", "weight"} {
		if !strings.Contains(head, want) {
			t.Errorf("the energy-input line omits %q: %q", want, head)
		}
	}
}
