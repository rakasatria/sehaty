package capability

import (
	"strings"
	"testing"
)

// The order is behaviour, not decoration, and it was chosen deliberately: equipment is
// cheap to answer and age is not. It is asserted here rather than in storage because
// this file is now the only place it lives.
func TestAskOrderIsTheConsideredOrder(t *testing.T) {
	got := AskOrder()
	if len(got) != 10 {
		t.Fatalf("expected the ten assessment questions, got %d", len(got))
	}
	if got[0].Field != FieldEquipment {
		t.Errorf("the first question is %q, not equipment", got[0].Field)
	}
	if got[len(got)-1].Field != FieldSex {
		t.Error("sex should be last — it usually answers itself before it is reached")
	}
	if index(got, FieldAge) < index(got, FieldHeight) {
		t.Error("age is asked before height, which is the intrusive order")
	}
}

// A field nobody can explain is a field nobody should be asked. Every question carries
// the true reason and the shape it is asked in, or it does not belong in the list.
func TestEveryQuestionCarriesItsReasonAndItsWording(t *testing.T) {
	for _, q := range AskOrder() {
		if strings.TrimSpace(q.Because) == "" {
			t.Errorf("%q has no reason", q.Field)
		}
		if strings.TrimSpace(q.Ask) == "" {
			t.Errorf("%q has no wording", q.Field)
		}
		if strings.Contains(q.Because, "personalise") {
			t.Errorf("%q gives a reason nobody believes", q.Field)
		}
	}
}

// Only the four with a real computation behind them can be told apart from their own
// zero value. The rest have defaults that are also valid answers — three sessions a week
// is both — so for those, presence means the question was put.
func TestOnlyComputableFieldsAreDetectable(t *testing.T) {
	want := map[Field]bool{FieldAge: true, FieldHeight: true, FieldSex: true}
	for _, q := range AskOrder() {
		if q.Detectable != want[q.Field] {
			t.Errorf("%q detectable = %v, want %v", q.Field, q.Detectable, want[q.Field])
		}
	}
}

// Three fields must not be asked cold. Each has a moment that makes it obvious instead.
func TestTheIntrusiveQuestionsAreGated(t *testing.T) {
	for _, f := range []Field{FieldHeight, FieldAllergies, FieldDietPreference, FieldSex} {
		q, ok := Ask(f)
		if !ok {
			t.Fatalf("%q is not a question", f)
		}
		if q.Gate == "" {
			t.Errorf("%q may be asked out of nowhere", f)
		}
	}
	q, _ := Ask(FieldEquipment)
	if q.Gate != "" {
		t.Error("equipment is gated, which makes the intake impossible to start")
	}
}

// The registry is what the tools need, and estimate_energy is the only tool that can be
// blocked: it is the only one with arithmetic behind it.
func TestTheRegistryDeclaresWhatEachToolNeeds(t *testing.T) {
	byTool := map[string]Capability{}
	for _, c := range Known() {
		byTool[c.Tool] = c
	}

	e, ok := byTool["estimate_energy"]
	if !ok {
		t.Fatal("estimate_energy declares nothing")
	}
	hard := map[Field]bool{}
	for _, n := range e.Needs {
		if n.Hard {
			hard[n.Field] = true
		}
	}
	for _, f := range []Field{FieldAge, FieldHeight, FieldSex, FieldWeightLog} {
		if !hard[f] {
			t.Errorf("estimate_energy does not hard-require %q", f)
		}
	}
	if len(hard) != 4 {
		t.Errorf("estimate_energy hard-requires %d fields, want 4", len(hard))
	}

	if _, ok := byTool["log_weight"]; ok {
		t.Error("log_weight declares requirements; it must work from minute one")
	}

	for _, c := range Known() {
		for _, n := range c.Needs {
			if strings.TrimSpace(n.Because) == "" {
				t.Errorf("%s needs %q for no stated reason", c.Tool, n.Field)
			}
		}
		if strings.TrimSpace(c.Unlocks) == "" {
			t.Errorf("%s unlocks nothing anyone can name", c.Tool)
		}
	}
}

// estimate_energy runs on a training frequency it will silently assume is three a week.
// Naming that assumption is the whole reason the soft category exists.
func TestTheAssumedTrainingFrequencyIsDeclaredNotHidden(t *testing.T) {
	for _, c := range Known() {
		if c.Tool != "estimate_energy" {
			continue
		}
		for _, n := range c.Needs {
			if n.Field == FieldSchedule && !n.Hard {
				return
			}
		}
	}
	t.Error("estimate_energy assumes a training frequency without declaring it")
}

func index(qs []Question, f Field) int {
	for i, q := range qs {
		if q.Field == f {
			return i
		}
	}
	return -1
}
