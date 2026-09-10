package agent

import "testing"

// Holding someone's age, height and weight is exactly the state in which a model starts
// volunteering "you should eat 1,800 kcal". Sehaty must never do that: the number comes
// from the person's own dietitian. This test guards the sentence that says so.
// The rule moved, deliberately: Sehaty now gives an energy estimate. What did NOT
// move is where the arithmetic happens. A model asked to multiply a weight will
// produce something that reads exactly like a correct answer, and nobody can tell
// a mistaken 2,340 from a right one by looking at it. So the sum belongs to Go and
// the prompt has to keep saying so.
func TestTheModelMayNotDoTheArithmeticItself(t *testing.T) {
	for _, want := range []string{
		"you get it",
		"estimate_energy, never by doing the arithmetic yourself",
		"multiply a weight by anything",
		"Never assume an age, a height or a weight",
		"You still do not PRESCRIBE",
		"dietitian",
	} {
		if !contains(SystemPrompt, want) {
			t.Errorf("the system prompt no longer says %q", want)
		}
	}
	// An estimate must never be handed over bare.
	for _, want := range []string{"RANGE", "twenty percent either side"} {
		if !contains(SystemPrompt, want) {
			t.Errorf("the prompt no longer requires the uncertainty: missing %q", want)
		}
	}
}

// One question at a time, never as an opening move, and dropped the moment it is
// deflected. This is the difference between a history and an interrogation.
func TestThePromptAsksOneThingAtATime(t *testing.T) {
	for _, want := range []string{
		"At most one question per message",
		"Never open with a\nquestion",
		"drop it. Do not rephrase it",
		"something volunteered is never asked about",
	} {
		if !contains(SystemPrompt, want) {
			t.Errorf("the system prompt no longer says %q", want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
