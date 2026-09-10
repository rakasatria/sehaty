package storage

import (
	"testing"

	"github.com/rakasatria/sehaty/internal/capability"
)

// The bug this design exists to prevent: three sessions a week is BOTH the default and a
// perfectly normal answer. Inferring "unanswered" from the value means the one person who
// actually trains three times a week gets asked about it every single day.
func TestAnAnswerEqualToTheDefaultStillCountsAsAnswered(t *testing.T) {
	p := Profile{SessionsPerWeek: 3, SessionMinutes: 50, Goal: "general",
		Experience: "beginner", Equipment: []string{"body weight"}}

	if !inList(p.Missing(), "training schedule") {
		t.Fatal("a profile nobody has been asked about reports nothing missing")
	}

	// They answer, and the answer happens to match the default.
	p.MarkAnswered("training schedule", "goal", "experience", "equipment")

	for _, q := range []string{"training schedule", "goal", "experience", "equipment"} {
		if inList(p.Missing(), q) {
			t.Errorf("%q is still being asked after it was answered", q)
		}
	}
}

// A declined question is an answered question. Anything else is nagging.
func TestDecliningCountsAsAnsweringAndIsIdempotent(t *testing.T) {
	var p Profile
	p.MarkAnswered("age")
	p.MarkAnswered("age")
	p.MarkAnswered("age", "sex")

	if len(p.Answered) != 2 {
		t.Fatalf("MarkAnswered duplicated entries: %v", p.Answered)
	}
	if inList(p.Missing(), "age") || inList(p.Missing(), "sex") {
		t.Error("a declined question came back")
	}
}

// Order is behaviour, not decoration: equipment is cheap to answer and age is not.
func TestMissingComesBackInTheOrderToAsk(t *testing.T) {
	got := Profile{}.Missing()
	if len(got) != len(capability.AskOrder()) {
		t.Fatalf("a blank profile should be missing everything, got %d of %d",
			len(got), len(capability.AskOrder()))
	}
	if got[0] != "equipment" {
		t.Errorf("the first question is %q, not equipment", got[0])
	}
	if indexOf(got, "age") < indexOf(got, "height") {
		t.Error("age is asked before height, which is the intrusive order")
	}
	if indexOf(got, "sex") != len(got)-1 {
		t.Error("sex should be last — it usually answers itself before it is reached")
	}
}

func inList(l []string, s string) bool { return indexOf(l, s) >= 0 }

func indexOf(l []string, s string) int {
	for i, v := range l {
		if v == s {
			return i
		}
	}
	return -1
}

// Have and Asked are different questions, and the profile answers both. Someone who
// declined to give their age has been asked and still has no age: the intake must stop
// asking, and estimate_energy must still refuse.
func TestADeclinedQuestionIsAnsweredButNotKnown(t *testing.T) {
	var p Profile
	p.MarkAnswered("age")

	if !p.Asked()[capability.FieldAge] {
		t.Error("the age question was put and the profile does not know it")
	}
	if p.Have(false)[capability.FieldAge] {
		t.Error("a declined age is being reported as an age on file")
	}
	if inList(p.Missing(), "age") {
		t.Error("a declined question came back")
	}
}

// A field with a default that is also a valid answer cannot be read from its value.
// For those, having been asked IS having it.
func TestUndetectableFieldsCountAsKnownOnceAsked(t *testing.T) {
	p := Profile{SessionsPerWeek: 3}
	if p.Have(false)[capability.FieldSchedule] {
		t.Error("a defaulted training schedule reads as an answer")
	}
	p.MarkAnswered("training schedule")
	if !p.Have(false)[capability.FieldSchedule] {
		t.Error("an answered training schedule does not read as known")
	}
}

// A logged weight is a fact in the log, not an answer to a question.
func TestTheWeightComesFromTheLogNotTheAnsweredSet(t *testing.T) {
	var p Profile
	if p.Have(false)[capability.FieldWeightLog] {
		t.Error("an unweighed profile reports a weight")
	}
	if !p.Have(true)[capability.FieldWeightLog] {
		t.Error("a weighed profile does not report one")
	}
}
