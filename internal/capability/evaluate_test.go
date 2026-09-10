package capability

import (
	"strings"
	"testing"
)

// A tool with an unmet hard need must refuse, and the refusal must name every missing
// thing at once. Naming one at a time turns a single question into four rounds.
func TestAToolWithUnmetHardNeedsIsBlocked(t *testing.T) {
	unmet, blocked := Blocked("estimate_energy", Have{})
	if !blocked {
		t.Fatal("estimate_energy is not blocked on a profile that knows nothing")
	}
	if len(unmet) != 4 {
		t.Fatalf("blocked on %d fields, want 4: %v", len(unmet), Names(unmet))
	}

	full := Have{FieldAge: true, FieldHeight: true, FieldSex: true, FieldWeightLog: true}
	if _, blocked := Blocked("estimate_energy", full); blocked {
		t.Error("blocked despite having every hard requirement")
	}
}

// A soft gap is never a refusal. It is something the tool says out loud while proceeding.
func TestASoftGapDoesNotBlockButIsNamed(t *testing.T) {
	full := Have{FieldAge: true, FieldHeight: true, FieldSex: true, FieldWeightLog: true}
	if _, blocked := Blocked("estimate_energy", full); blocked {
		t.Fatal("a soft gap blocked the tool")
	}
	gaps := SoftGaps("estimate_energy", full)
	if len(gaps) != 1 || gaps[0].Field != FieldSchedule {
		t.Fatalf("the assumed training frequency is not reported: %v", Names(gaps))
	}
	if !strings.Contains(gaps[0].Because, "three sessions a week") {
		t.Error("the gap does not say what is being assumed in its place")
	}

	withSchedule := Have{FieldAge: true, FieldHeight: true, FieldSex: true,
		FieldWeightLog: true, FieldSchedule: true}
	if len(SoftGaps("estimate_energy", withSchedule)) != 0 {
		t.Error("a satisfied requirement is still being reported as a gap")
	}
}

// An unknown tool needs nothing. Any other answer would let a typo silently gate a tool.
func TestAnUnregisteredToolIsNeverBlocked(t *testing.T) {
	if _, blocked := Blocked("log_weight", Have{}); blocked {
		t.Error("log_weight was blocked; it must work from minute one")
	}
	if _, blocked := Blocked("no_such_tool", Have{}); blocked {
		t.Error("an unknown tool was blocked")
	}
}

// What to ask is what is unmet AND has not been put yet. A question that was declined
// has been answered — asking it again tomorrow is how an assistant becomes a form that
// follows someone around.
func TestToAskSkipsWhatWasAlreadyPut(t *testing.T) {
	all := ToAsk(Have{}, Asked{})
	if len(all) != 10 {
		t.Fatalf("a blank profile is asked %d things, want 10", len(all))
	}
	if all[0].Field != FieldEquipment {
		t.Errorf("the first question is %q, not equipment", all[0].Field)
	}

	got := ToAsk(Have{}, Asked{FieldAge: true, FieldSex: true})
	for _, r := range got {
		if r.Field == FieldAge || r.Field == FieldSex {
			t.Errorf("%q came back after being answered", r.Field)
		}
	}
	if len(got) != 8 {
		t.Errorf("asked %d after two answers, want 8", len(got))
	}
}

// weight_log is not a question. Nobody is asked for it; log_weight satisfies it.
func TestTheWeightLogIsNeverAsked(t *testing.T) {
	for _, r := range ToAsk(Have{}, Asked{}) {
		if r.Field == FieldWeightLog {
			t.Fatal("weight_log is being asked as an intake question")
		}
	}
}

// Every question carries the reason it is being asked, so the brief never has to compose
// one and the model never has to invent one.
func TestWhatToAskCarriesTheReason(t *testing.T) {
	for _, r := range ToAsk(Have{}, Asked{}) {
		if strings.TrimSpace(r.Because) == "" {
			t.Errorf("%q would be asked with no reason attached", r.Field)
		}
	}
}

// The Mini App shows what is locked and why. A capability with every hard need met is
// not locked, whatever its soft gaps.
func TestLockedListsOnlyWhatIsHardBlocked(t *testing.T) {
	locked := Locked(Have{})
	names := []string{}
	for _, c := range locked {
		names = append(names, c.Tool)
	}
	if len(locked) != 1 || locked[0].Tool != "estimate_energy" {
		t.Fatalf("locked = %v, want only estimate_energy", names)
	}
	// Only the unmet needs travel: a person does not need to be told about the two
	// things they already answered.
	for _, n := range locked[0].Needs {
		if n.Field == FieldSchedule {
			t.Error("a soft requirement is being shown as a lock")
		}
	}

	full := Have{FieldAge: true, FieldHeight: true, FieldSex: true, FieldWeightLog: true}
	if len(Locked(full)) != 0 {
		t.Error("something is still locked once every hard need is met")
	}
}

// One question to a person is two sets of buttons on a phone, and the record should not
// care which way it arrived.
func TestQuestionAliasesResolve(t *testing.T) {
	for _, alias := range []string{"sessions per week", "session minutes"} {
		f, ok := Canonical(alias)
		if !ok || f != FieldSchedule {
			t.Errorf("%q did not resolve to the training schedule", alias)
		}
	}
	if f, ok := Canonical("age"); !ok || f != FieldAge {
		t.Error("a field name does not resolve to itself")
	}
	if _, ok := Canonical("favourite colour"); ok {
		t.Error("an invented question resolved")
	}
}
