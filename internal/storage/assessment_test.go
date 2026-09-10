package storage

import "testing"

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
	if len(got) != len(Assessment) {
		t.Fatalf("a blank profile should be missing everything, got %d of %d",
			len(got), len(Assessment))
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
