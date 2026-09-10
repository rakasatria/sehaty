package agent

import "testing"

// Holding someone's age, height and weight is exactly the state in which a model starts
// volunteering "you should eat 1,800 kcal". Sehaty must never do that: the number comes
// from the person's own dietitian. This test guards the sentence that says so.
func TestThePromptForbidsComputingTargetsEvenWithFullDetail(t *testing.T) {
	for _, want := range []string{
		"never calculate or state a calorie target",
		"dietitian",
		"Never invent a number",
	} {
		if !contains(SystemPrompt, want) {
			t.Errorf("the system prompt no longer says %q", want)
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
