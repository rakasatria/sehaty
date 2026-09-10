# Capability Layer and Soul Split — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Sehaty's tools declare what they need, derive the assessment and the system prompt from that single declaration, and split the prompt into reviewable behaviour files whose every claim carries a visible source.

**Architecture:** A new dependency-free package `internal/capability` holds one registry: which tool needs which field, whether the need is hard or soft, the true reason, and the Indonesian shape of the question. Three readers consume it — `EstimateEnergy` refuses on unmet hard needs, `Brief` derives what is still unknown, `/api/summary` reports what is locked. The system prompt stops being one 377-line Go string constant and becomes `intent.md` + `limits.md` + `manner.md` + `skills/*.md`, embedded with `go:embed`, composed at init, with HTML-comment provenance tags stripped before the model ever sees them, and with the two hand-maintained question sections replaced by text generated from the registry.

**Tech Stack:** Go 1.27.1, `go:embed`, stdlib only for the new package. No new dependencies. Existing: goai v0.10.1, telego v1.12.1, ncruces/go-sqlite3, bleve.

**Spec:** `docs/superpowers/specs/2026-09-10-sehaty-architecture-design.md` — sections 1, 2 and 4, and order-of-work items 1 and 2.

---

## Global Constraints

- **Build host.** Go is not installed on the Mac. Every `go build`, `go test` and `go vet` runs in `~/codes/sehaty` on hrmdev01 (10.254.1.104), reached with `hermes peer dm hrmdev01`. A peer DM is ONE TURN — do not hold one open for a long build.
- **Module path** is `github.com/rakasatria/sehaty`. Go version floor `1.27.1` (`go.mod`).
- **No new dependencies.** `internal/capability` imports only `fmt`, `sort` and `strings`.
- **The model never produces a number.** Every figure a person sees is computed in Go, next to the data, by code that can be tested.
- **`null` is not zero.** "Not recorded" and "none" are different facts and are presented differently.
- **Function, type, field, command and test names are English.** Raka's standing rule. User-facing copy is Indonesian.
- **`Requirement.Because` and `Question.Ask` are user-facing copy**, so `Ask` is Jakarta Indonesian and `Because` is the English reason the model relays and the Mini App renders through its own copy. Both are lifted verbatim from the current prompt — this plan moves them, it does not rewrite them.
- **No real measurement of a real person appears in code, copy, comments or fixtures.** Use 70 kg / 70.5 / 72.3.
- **TDD.** Failing test first, watch it fail, minimal implementation, watch it pass, commit. One commit per task minimum.
- **Every task ends with `go test ./...` green across all 15 packages.** The tree is green at `e7a1b63`; it stays green.

---

## Deviations from the spec, for the reviewer

Two, both deliberate, both flagged rather than buried:

1. **`plan_session`'s `equipment` and `experience` are SOFT, not hard.** The spec's table (§2) marks them hard. But hard means *refuse, because a number would have to be invented*, and neither is that: both have real defaults, and a session built from the recorded equipment with the gap named out loud — "ini aku susun pakai alat yang tercatat aja" — is strictly more useful than a refusal. After this plan, `estimate_energy` is the only tool with hard requirements, which is right: it is the only one with arithmetic behind it.

2. **A third top-level soul file, `manner.md`.** The spec (§4) lists `intent.md` and `limits.md`. MANNER and STYLE are neither purpose nor prohibition — they are the constant tone every skill assumes, and they are the most heavily-evidenced part of the whole prompt. Folding them into `intent.md` would hide that.

`draft_programme` is **not** declared here. It is order-of-work item 3 and gets its capability entry in the programme plan, where the tool exists. When it lands it will move `goal` and `training schedule` from soft to hard.

---

## File Structure

**Created:**

| path | responsibility |
|---|---|
| `internal/capability/capability.go` | `Field` constants, `Question`, `Requirement`, `Capability`, `AskOrder()`, `Known()` — the registry and nothing else |
| `internal/capability/evaluate.go` | `Have`, `Asked`, `Blocked`, `SoftGaps`, `ToAsk`, `Locked`, `Canonical`, `Names` |
| `internal/capability/describe.go` | `Describe()` — the generated prompt section |
| `internal/capability/capability_test.go` | registry shape: the table, the ask order, the reasons |
| `internal/capability/evaluate_test.go` | evaluation: blocked, soft gaps, what to ask |
| `internal/agent/soul.go` | `//go:embed soul`, `strip()`, `soulText()`, `Compose()` |
| `internal/agent/soul/intent.md` | what Sehaty is and what it is for |
| `internal/agent/soul/limits.md` | what it will never do |
| `internal/agent/soul/manner.md` | how it sounds, and encouragement |
| `internal/agent/soul/skills/taking-a-food-entry.md` | food and flagged foods |
| `internal/agent/soul/skills/asking-a-question.md` | the consultation, the history, starting over |
| `internal/agent/soul/skills/after-a-gap.md` | return as continuation |
| `internal/agent/soul/skills/declining-a-target.md` | estimate versus prescription |
| `internal/agent/soul/skills/noticing-progress.md` | earned, specific encouragement |
| `internal/agent/soul/skills/when-they-push-back.md` | the MI-nonadherent list |
| `internal/agent/soul_test.go` | golden equality, provenance stripping, generation |
| `internal/agent/testdata/prompt.golden` | the prompt as it stands at `e7a1b63` |
| `internal/tools/capability.go` | `Have(d, p)` — the only place that reads the log to answer "is there a weight" |

**Modified:**

| path | change |
|---|---|
| `internal/storage/profile.go` | delete `Assessment`, `questionAliases`, `Canonical`, `Gated`; reimplement `Missing()` via the registry; add `Have()` and `Asked()` on `Profile` |
| `internal/storage/assessment_test.go` | `len(Assessment)` → `len(capability.AskOrder())` |
| `internal/tools/tools.go` | `storage.Canonical` → `capability.Canonical`; `storage.Assessment` → `capability.Names(capability.AskOrder())` in the declined-question error |
| `internal/tools/energy.go` | refusal derived from `capability.Blocked`; `Assumed []string` on `EnergyEstimate`; "Mifflin-St Jeor" → the equation actually used |
| `internal/agent/registry.go` | `estimate_energy` description no longer names Mifflin-St Jeor |
| `internal/agent/brief.go` | derive from `capability.ToAsk`; carry `Because`; `storage.Gated` → `capability.Gate` |
| `internal/agent/brief_test.go` | `storage.Assessment...` → `capability.Names(capability.AskOrder())...` |
| `internal/agent/agent.go` | `const SystemPrompt` → `var SystemPrompt = Compose()`; the 377-line literal is deleted |
| `internal/dashboard/api.go` | `summary.Locked []lockedCapability` |
| `web/src/types.ts` | `locked: LockedCapability[]` |
| `web/src/components/Locked.tsx` | new component rendering it |
| `web/src/App.tsx` | render `<Locked />` |

---

## Task 1: The capability registry

**Files:**
- Create: `internal/capability/capability.go`
- Test: `internal/capability/capability_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `type Field string`; the eleven `Field` constants; `type Question struct{ Field Field; Detectable bool; Gate, Because, Ask string }`; `type Requirement struct{ Field Field; Because string; Hard bool }`; `type Capability struct{ Tool string; Needs []Requirement; Unlocks string }`; `func AskOrder() []Question`; `func Known() []Capability`; `func Ask(f Field) (Question, bool)`.

- [ ] **Step 1: Write the failing test**

Create `internal/capability/capability_test.go`:

```go
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
```

- [ ] **Step 2: Run it and watch it fail**

Send to hrmdev01:

```
cd ~/codes/sehaty && go test ./internal/capability/ 2>&1 | tail -20
```

Expected: `no required module provides package` or `undefined: AskOrder` — the package does not exist yet.

- [ ] **Step 3: Write the registry**

Create `internal/capability/capability.go`:

```go
// Package capability is the one place that records what Sehaty's tools need.
//
// Before this package the same knowledge lived in three: a hand-kept list of assessment
// questions in storage, a hand-written refusal inside each tool, and fifty lines of
// prose in the system prompt explaining why each question is asked. Three copies of one
// fact drift, and the drift is invisible — a tool gains an input and nobody adds the
// question, or a question is asked for a reason that stopped being true.
//
// Now the tool declares its need once, with the true reason attached, and the assessment,
// the refusals and the prompt are all derived from that declaration. A reason that is
// derived from a requirement is true by construction; a reason that is composed is only
// as true as whoever last edited the paragraph.
//
// This package imports nothing from the rest of Sehaty, deliberately: everything else may
// depend on it, so it may depend on nothing.
package capability

// Field is one thing Sehaty can know about a person.
//
// The string values are LOAD-BEARING. They are what `profile.answered` holds on disk, so
// changing one silently un-answers that question for every existing record and starts
// asking it again.
type Field string

const (
	FieldEquipment      Field = "equipment"
	FieldGoal           Field = "goal"
	FieldExperience     Field = "experience"
	FieldSchedule       Field = "training schedule"
	FieldInjuries       Field = "injuries or conditions"
	FieldHeight         Field = "height"
	FieldAge            Field = "age"
	FieldDietPreference Field = "diet preference"
	FieldAllergies      Field = "food allergies"
	FieldSex            Field = "sex"

	// FieldWeightLog is not a question. It is satisfied by a logged weight, so it never
	// appears in AskOrder — it is asked for by log_weight, not by the intake.
	FieldWeightLog Field = "weight_log"
)

// Question is a field that can be put to a person.
type Question struct {
	Field Field

	// Detectable says whether an absent value can be told from a given one.
	//
	// False for most of them, and that is the bug this flag exists to prevent: three
	// sessions a week is both the default and a perfectly normal answer, so inferring
	// "never asked" from the value would ask the one person who actually trains three
	// times a week about it every single day. For those fields, presence means the
	// question was put — a decline included.
	Detectable bool

	// Gate names the moment this must not be asked outside of. Empty means any time.
	//
	// Asked out of nowhere these read as data harvesting rather than as a health record
	// doing its job, and each has a moment that makes it obvious instead.
	Gate string

	// Because is the true consequence in the record, in the words the person gets if
	// they ask why. Never "to personalise your experience" — that is not true here and
	// they will smell it.
	Because string

	// Ask is the shape the question takes: Jakarta Indonesian, not textbook Indonesian.
	Ask string
}

// askOrder is the order to put the questions in, and the order is behaviour.
//
// Equipment is cheap to answer and age is not, so the list opens with something nobody
// minds and arrives at the personal end once there is a reason to be there. Sex is last
// because it usually settles itself in passing before it is ever reached.
var askOrder = []Question{
	{
		Field:   FieldEquipment,
		Because: "So a suggested session only uses things you actually have.",
		Ask:     "Biasa latihan pake apa — nge-gym, alat di rumah, atau bodyweight aja?",
	},
	{
		Field: FieldGoal,
		Because: "It decides what I watch in your numbers — a cut and a strength block " +
			"read the same log very differently.",
		Ask: "Latihannya lagi ngejar apa — nurunin lemak, nambah kuat, nambah otot, " +
			"atau jaga kondisi aja?",
	},
	{
		Field:   FieldExperience,
		Because: "So sessions are pitched where you are: not remedial, not reckless.",
		Ask:     "Udah berapa lama latihan? Baru mulai, atau udah lama?",
	},
	{
		Field: FieldSchedule,
		Because: "Every session I suggest is built on how often and how long you train. " +
			"It has been assuming three times a week for fifty minutes; I would rather know.",
		Ask: "Seminggu bisa latihan berapa kali, dan sekali latihan berapa lama?",
	},
	{
		Field: FieldInjuries,
		Because: "So I never suggest a movement that aggravates it, and so it is on the " +
			"record if a clinician ever reads this.",
		Ask: "Ada cedera lama atau bagian badan yang suka rewel? Lutut, bahu, pinggang.",
	},
	{
		Field:      FieldHeight,
		Detectable: true,
		Gate:       "only when a weight has just been logged",
		Because: "70 kg means something different at 165 cm and at 185. Height sits next " +
			"to your weight log so it reads properly.",
		Ask: "Tinggimu berapa? Angka berat lebih kebaca kalau ada tingginya.",
	},
	{
		Field:      FieldAge,
		Detectable: true,
		Because: "Recovery and pacing shift with age, so a session can suit yours — and a " +
			"doctor reading this record would expect it there.",
		Ask: "Umur berapa, kalau boleh tanya?",
	},
	{
		Field:   FieldDietPreference,
		Gate:    "only when food is being logged — vegetarian, non-vegetarian or vegan",
		Because: "So a suggestion is something you would actually eat.",
		Ask:     "Makannya ada aturan khusus? Vegetarian, vegan, atau makan semua?",
	},
	{
		Field: FieldAllergies,
		Gate:  "only when food is being logged",
		Because: "So it is flagged in your record, and I never suggest a food that would " +
			"hurt you.",
		Ask: "Sebelum catatan makannya makin panjang — ada alergi makanan?",
	},
	{
		Field:      FieldSex,
		Detectable: true,
		Gate: "prefer never asking — it usually surfaces on its own. Ask only when a " +
			"reference range or an exercise choice makes it concretely relevant",
		Because: "Strength references and some exercise choices differ. That is the whole use.",
		Ask: "Buat catatan aja — cowok atau cewek? Angka acuan sama beberapa pilihan " +
			"latihan emang beda.",
	},
}

// AskOrder returns the assessment questions in the order to put them.
func AskOrder() []Question {
	out := make([]Question, len(askOrder))
	copy(out, askOrder)
	return out
}

// Ask returns one question by field.
func Ask(f Field) (Question, bool) {
	for _, q := range askOrder {
		if q.Field == f {
			return q, true
		}
	}
	return Question{}, false
}

// Requirement is one field a tool needs.
type Requirement struct {
	Field Field

	// Because is the true reason, shown to the person. It says what this tool cannot do
	// without the field, not what the field is.
	Because string

	// Hard means refuse: a number would have to be invented. Soft means proceed AND name
	// the gap, which is a different failure from silence and keeps day-one friction low.
	Hard bool
}

// Capability is one tool and what it needs to work.
type Capability struct {
	Tool    string
	Needs   []Requirement
	Unlocks string
}

// known is the registry.
//
// Tools absent from this list need nothing and work from minute one: log_food, log_set,
// log_weight, log_cardio, find_foods, find_exercises, progress, get_profile. That is not
// an oversight — a health record that cannot be written to until a form is finished is a
// form, and only about one beginner in ten is still training at 52 weeks.
var known = []Capability{
	{
		Tool:    "estimate_energy",
		Unlocks: "an estimate of how much energy you use in a day",
		Needs: []Requirement{
			{Field: FieldAge, Hard: true,
				Because: "The equation is age-banded; without an age there is no coefficient to use."},
			{Field: FieldHeight, Hard: true,
				Because: "A weight on its own does not say what body it belongs to."},
			{Field: FieldSex, Hard: true,
				Because: "The published equations have separate forms, and there is no unisex one."},
			{Field: FieldWeightLog, Hard: true,
				Because: "The estimate is built on a real weight. Guessing one produces a number that reads exactly like a measured one."},
			{Field: FieldSchedule,
				Because: "How often you train carries most of the error in this estimate. Without it I assume three sessions a week, and I would rather you knew that than not."},
		},
	},
	{
		Tool:    "plan_session",
		Unlocks: "a session built for you rather than a generic one",
		Needs: []Requirement{
			{Field: FieldEquipment,
				Because: "Without it I can only suggest movements from what is already on your record."},
			{Field: FieldExperience,
				Because: "Without it the session is pitched at the middle, which is too much for some people and too little for others."},
			{Field: FieldGoal,
				Because: "Sets, reps and rest come from the goal. Without one I use the general prescription."},
			{Field: FieldInjuries,
				Because: "Without it I am assuming nothing hurts, and I would rather be told than assume."},
		},
	},
	{
		Tool:    "log_food",
		Unlocks: "a food log that can warn you",
		Needs: []Requirement{
			{Field: FieldAllergies,
				Because: "Without it nothing I suggest is checked against anything that would hurt you."},
			{Field: FieldDietPreference,
				Because: "Without it a suggestion may be something you do not eat."},
		},
	},
}

// Known returns the registry.
func Known() []Capability {
	out := make([]Capability, len(known))
	copy(out, known)
	return out
}
```

- [ ] **Step 4: Run it and watch it pass**

```
cd ~/codes/sehaty && go test ./internal/capability/ -v 2>&1 | tail -30
```

Expected: PASS, six tests.

- [ ] **Step 5: Commit**

```bash
cd ~/codes/sehaty
git add internal/capability/
git commit -m "feat(capability): tools declare what they need, with the reason attached"
```

---

## Task 2: Evaluating the registry

**Files:**
- Create: `internal/capability/evaluate.go`
- Test: `internal/capability/evaluate_test.go`

**Interfaces:**
- Consumes: `Field`, `Question`, `Requirement`, `Capability`, `AskOrder()`, `Known()`, `Ask()` from Task 1.
- Produces: `type Have map[Field]bool`; `type Asked map[Field]bool`; `func Blocked(tool string, have Have) ([]Requirement, bool)`; `func SoftGaps(tool string, have Have) []Requirement`; `func ToAsk(have Have, asked Asked) []Requirement`; `func Locked(have Have) []Capability`; `func Canonical(q string) (Field, bool)`; `func Names(in any) []string`.

- [ ] **Step 1: Write the failing test**

Create `internal/capability/evaluate_test.go`:

```go
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
```

- [ ] **Step 2: Run it and watch it fail**

```
cd ~/codes/sehaty && go test ./internal/capability/ 2>&1 | tail -20
```

Expected: `undefined: Blocked`, `undefined: Have`, and the rest.

- [ ] **Step 3: Write the evaluation**

Create `internal/capability/evaluate.go`:

```go
package capability

// Have is the set of fields whose value is actually on file.
//
// Have and Asked are DIFFERENT QUESTIONS and conflating them was the trap this pair
// exists to avoid. "Do I have an age?" decides whether estimate_energy can run. "Has the
// age question been put?" decides whether to ask again. Someone who declined to give
// their age has answered the question and still has no age, and only one of those two
// facts should stop the tool.
type Have map[Field]bool

// Asked is the set of questions already put, declines included.
type Asked map[Field]bool

// Blocked reports the unmet HARD requirements of a tool.
//
// A tool absent from the registry needs nothing and is never blocked. That is the safe
// default in both directions: a typo cannot gate a working tool, and a tool that needs
// something must say so here to get it enforced.
func Blocked(tool string, have Have) ([]Requirement, bool) {
	var unmet []Requirement
	for _, c := range known {
		if c.Tool != tool {
			continue
		}
		for _, n := range c.Needs {
			if n.Hard && !have[n.Field] {
				unmet = append(unmet, n)
			}
		}
	}
	return unmet, len(unmet) > 0
}

// SoftGaps reports the unmet SOFT requirements of a tool: what it is proceeding without,
// so it can say so rather than assume in silence.
func SoftGaps(tool string, have Have) []Requirement {
	var gaps []Requirement
	for _, c := range known {
		if c.Tool != tool {
			continue
		}
		for _, n := range c.Needs {
			if !n.Hard && !have[n.Field] {
				gaps = append(gaps, n)
			}
		}
	}
	return gaps
}

// ToAsk is what is still worth putting to this person: unmet, and not yet put.
//
// In ask order, not in unlock order. Unlock order says what would be most useful to know
// next; ask order says what a person will not mind being asked next, and the second one
// is what decides whether they answer at all. The brief leads with the energy inputs
// separately when the intake is unfinished, which is where unlock order gets its say.
//
// The Because carried here is the QUESTION's reason — what the field is for in general —
// rather than any one tool's, because the same field usually serves several.
func ToAsk(have Have, asked Asked) []Requirement {
	var out []Requirement
	for _, q := range askOrder {
		if have[q.Field] || asked[q.Field] {
			continue
		}
		out = append(out, Requirement{Field: q.Field, Because: q.Because})
	}
	return out
}

// Locked is what this person cannot do yet, and the unmet hard requirements that are the
// reason. Soft gaps are excluded: those do not lock anything.
func Locked(have Have) []Capability {
	var out []Capability
	for _, c := range known {
		unmet, blocked := Blocked(c.Tool, have)
		if !blocked {
			continue
		}
		out = append(out, Capability{Tool: c.Tool, Needs: unmet, Unlocks: c.Unlocks})
	}
	return out
}

// aliases map the finer-grained names a UI may use onto the question they answer.
//
// "Seminggu berapa kali, dan berapa lama?" is one question to a person and two sets of
// buttons on a phone.
var aliases = map[string]Field{
	"sessions per week": FieldSchedule,
	"session minutes":   FieldSchedule,
}

// Canonical resolves a question name to the field it answers.
func Canonical(q string) (Field, bool) {
	if f, ok := aliases[q]; ok {
		return f, true
	}
	for _, k := range askOrder {
		if string(k.Field) == q {
			return k.Field, true
		}
	}
	if q == string(FieldWeightLog) {
		return FieldWeightLog, true
	}
	return "", false
}

// Names lists the fields of a requirement or question slice, for error messages.
func Names(in any) []string {
	switch v := in.(type) {
	case []Requirement:
		out := make([]string, 0, len(v))
		for _, r := range v {
			out = append(out, string(r.Field))
		}
		return out
	case []Question:
		out := make([]string, 0, len(v))
		for _, q := range v {
			out = append(out, string(q.Field))
		}
		return out
	}
	return nil
}
```

- [ ] **Step 4: Run it and watch it pass**

```
cd ~/codes/sehaty && go test ./internal/capability/ -v 2>&1 | tail -40
```

Expected: PASS, fourteen tests.

- [ ] **Step 5: Commit**

```bash
cd ~/codes/sehaty
git add internal/capability/
git commit -m "feat(capability): refuse on hard needs, name soft ones, ask the rest"
```

---

## Task 3: The assessment derives from the registry

Deletes `storage.Assessment`, `questionAliases`, `Canonical` and `Gated`, and reimplements `Profile.Missing()` on top of the registry. Behaviour is unchanged and the existing storage tests are the proof.

**Files:**
- Modify: `internal/storage/profile.go` (the block from `var Assessment = []string{` through the end of `func Gated`)
- Modify: `internal/storage/assessment_test.go:44` (`len(Assessment)`)
- Modify: `internal/tools/tools.go:435-440` (the declined-question error)
- Modify: `internal/agent/brief_test.go:57` (`storage.Assessment...`)
- Test: `internal/storage/assessment_test.go`

**Interfaces:**
- Consumes: `capability.Field`, `capability.AskOrder()`, `capability.ToAsk()`, `capability.Canonical()`, `capability.Names()`, `capability.Have`, `capability.Asked`.
- Produces: `func (p Profile) Have(weighed bool) capability.Have`; `func (p Profile) Asked() capability.Asked`; `func (p Profile) Missing() []string` (unchanged signature); `func (p *Profile) MarkAnswered(questions ...string)` (unchanged signature).

- [ ] **Step 1: Write the failing test**

Append to `internal/storage/assessment_test.go`:

```go
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
```

Add the import `"github.com/rakasatria/sehaty/internal/capability"` to that file, and change line 44's `len(Assessment)` to `len(capability.AskOrder())`.

- [ ] **Step 2: Run it and watch it fail**

```
cd ~/codes/sehaty && go test ./internal/storage/ 2>&1 | tail -20
```

Expected: `p.Have undefined (type Profile has no field or method Have)`.

- [ ] **Step 3: Replace the block in `internal/storage/profile.go`**

Delete everything from `var Assessment = []string{` down to and including the closing brace of `func Gated`, and put this in its place:

```go
// Have is what is actually on file about this person, as the capability registry counts
// it. weighed comes from the weight log, which is a fact rather than an answer.
//
// Only four fields can be told from their own zero value; for the rest, a default that
// is also a valid answer makes the value useless as evidence, so having been asked is
// the only signal there is. See capability.Question.Detectable.
func (p Profile) Have(weighed bool) capability.Have {
	h := capability.Have{}
	if weighed {
		h[capability.FieldWeightLog] = true
	}
	asked := p.Asked()
	for _, q := range capability.AskOrder() {
		if !q.Detectable {
			h[q.Field] = asked[q.Field]
			continue
		}
		switch q.Field {
		case capability.FieldAge:
			h[q.Field] = p.Age > 0
		case capability.FieldHeight:
			h[q.Field] = p.HeightCm > 0
		case capability.FieldSex:
			h[q.Field] = p.Sex != ""
		}
	}
	return h
}

// Asked is every question that has been put to this person, declines included.
func (p Profile) Asked() capability.Asked {
	a := capability.Asked{}
	for _, q := range p.Answered {
		if f, ok := capability.Canonical(q); ok {
			a[f] = true
		}
	}
	return a
}

// Missing lists the questions still worth putting, in the order to put them.
//
// It reads the answered set, NOT the values, for every field whose default is also a
// valid answer. Inferring from those cannot work: someone who trains three times a week
// and someone who has never been asked both hold 3, so the value heuristic would ask the
// second question forever. A declined question counts as answered — being asked once is
// a question, being asked every day is nagging.
func (p Profile) Missing() []string {
	// weighed is false here deliberately: Missing answers "what should I ask", and
	// nobody is asked for a weight — log_weight settles it. Passing false cannot add
	// weight_log to the result because it is not one of the questions.
	out := []string{}
	for _, r := range capability.ToAsk(p.Have(false), p.Asked()) {
		out = append(out, string(r.Field))
	}
	return out
}

// MarkAnswered records that a question has been put and answered, declines included.
func (p *Profile) MarkAnswered(questions ...string) {
	have := make(map[string]bool, len(p.Answered))
	for _, a := range p.Answered {
		have[a] = true
	}
	for _, q := range questions {
		f, ok := capability.Canonical(q)
		if !ok {
			continue
		}
		if !have[string(f)] {
			p.Answered = append(p.Answered, string(f))
			have[string(f)] = true
		}
	}
}
```

Add `"github.com/rakasatria/sehaty/internal/capability"` to the file's imports.

- [ ] **Step 4: Fix the two call sites that referenced the deleted names**

In `internal/tools/tools.go`, replace the declined-question loop (around line 435) with:

```go
	// A question that was put and turned down is answered. Asking it again tomorrow is
	// how an assistant stops being useful and starts being a form that follows you around.
	for _, q := range a.Declined {
		if _, ok := capability.Canonical(q); !ok {
			return p, fmt.Errorf("declined %q is not one of: %s",
				q, strings.Join(capability.Names(capability.AskOrder()), ", "))
		}
		p.MarkAnswered(q)
	}
```

and add `"github.com/rakasatria/sehaty/internal/capability"` to that file's imports.

In `internal/agent/brief_test.go`, replace line 57:

```go
	done.MarkAnswered(capability.Names(capability.AskOrder())...)
```

and add `"github.com/rakasatria/sehaty/internal/capability"` to that file's imports.

- [ ] **Step 5: Run the whole suite and watch it pass**

```
cd ~/codes/sehaty && go build ./... && go vet ./... && go test ./... 2>&1 | tail -25
```

Expected: all 15 packages `ok`. In particular `TestAnAnswerEqualToTheDefaultStillCountsAsAnswered`, `TestDecliningCountsAsAnsweringAndIsIdempotent` and `TestMissingComesBackInTheOrderToAsk` pass unchanged — that is the proof the behaviour did not move.

- [ ] **Step 6: Prove the old names are gone**

```
cd ~/codes/sehaty && grep -rn "storage.Assessment\|storage.Canonical\|storage.Gated\|questionAliases" --include=*.go . ; echo "exit=$?"
```

Expected: no output, `exit=1`. If anything prints, fix that call site and re-run Step 5.

- [ ] **Step 7: Commit**

```bash
cd ~/codes/sehaty
git add internal/storage/ internal/tools/tools.go internal/agent/brief_test.go
git commit -m "refactor(storage): the assessment is derived from the registry, not kept beside it"
```

---

## Task 4: The tool refuses from the registry, and names what it assumed

`EstimateEnergy` stops carrying its own hand-written list of inputs and asks the registry instead. It also gains `Assumed` — the soft gaps it proceeded past — so "I assumed three sessions a week" stops being invisible. Two stale references to Mifflin-St Jeor are corrected: the equation is Henry/Oxford as of `e7a1b63`, and the tool description and the under-18 refusal still name the old one.

**Files:**
- Create: `internal/tools/capability.go`
- Modify: `internal/tools/energy.go:143-175` and the `EnergyEstimate` struct near line 44
- Modify: `internal/agent/registry.go` (the `estimate_energy` description)
- Test: `internal/tools/energy_test.go`

**Interfaces:**
- Consumes: `capability.Blocked`, `capability.SoftGaps`, `capability.Names`, `storage.Profile.Have`.
- Produces: `func Have(d Deps, p storage.Profile) capability.Have`; `EnergyEstimate.Assumed []string`.

- [ ] **Step 1: Write the failing test**

Append to `internal/tools/energy_test.go`:

```go
// The refusal must name every missing input at once. One at a time turns a single
// question into four rounds of conversation.
func TestTheRefusalNamesEveryMissingInputAtOnce(t *testing.T) {
	d := testDeps(t)
	p := mustProfile(t, d, storage.Profile{DisplayName: "Someone"})

	_, err := EstimateEnergy(d, p.ID)
	if err == nil {
		t.Fatal("estimated from a profile that knows nothing")
	}
	for _, want := range []string{"age", "height", "sex", "weight"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal does not mention %q: %v", want, err)
		}
	}
	if !strings.Contains(err.Error(), "ask for it rather than assuming") {
		t.Error("the refusal does not say what to do about it")
	}
}

// A soft gap does not refuse. It proceeds and says what it assumed — which is the whole
// point, because the training frequency carries most of the error in this estimate.
func TestAnAssumedTrainingFrequencyIsReportedNotHidden(t *testing.T) {
	d := testDeps(t)
	p := mustProfile(t, d, storage.Profile{
		DisplayName: "Someone", Age: 34, HeightCm: 173, Sex: "male",
	})
	if err := d.DB.LogWeight(p.ID, storage.WeightEntry{Date: today(), WeightKg: 72.3}); err != nil {
		t.Fatal(err)
	}

	got, err := EstimateEnergy(d, p.ID)
	if err != nil {
		t.Fatalf("refused despite having every hard input: %v", err)
	}
	if len(got.Assumed) == 0 {
		t.Fatal("proceeded on an assumed training frequency and did not say so")
	}
	if !strings.Contains(strings.Join(got.Assumed, " "), "three sessions a week") {
		t.Errorf("does not say what was assumed: %v", got.Assumed)
	}

	// Once the schedule has actually been answered there is nothing left to assume.
	p.SessionsPerWeek = 4
	p.MarkAnswered("training schedule")
	if err := d.DB.SaveProfile(p); err != nil {
		t.Fatal(err)
	}
	got, err = EstimateEnergy(d, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Assumed) != 0 {
		t.Errorf("still reporting an assumption after it was answered: %v", got.Assumed)
	}
}

// The equation changed at e7a1b63 and two places still named the old one. A refusal that
// cites an equation the code does not use is a refusal nobody can check.
func TestNothingStillClaimsMifflinStJeor(t *testing.T) {
	d := testDeps(t)
	p := mustProfile(t, d, storage.Profile{
		DisplayName: "Someone", Age: 15, HeightCm: 160, Sex: "female",
	})
	if err := d.DB.LogWeight(p.ID, storage.WeightEntry{Date: today(), WeightKg: 50.0}); err != nil {
		t.Fatal(err)
	}
	_, err := EstimateEnergy(d, p.ID)
	if err == nil {
		t.Fatal("estimated for someone under 18")
	}
	if strings.Contains(err.Error(), "Mifflin") {
		t.Errorf("the refusal still names an equation this code does not use: %v", err)
	}
	if !strings.Contains(err.Error(), "paediatric dietitian") {
		t.Errorf("the refusal no longer says who this belongs to: %v", err)
	}
}
```

If `testDeps` and `mustProfile` are not already helpers in `internal/tools`, use whatever the existing `energy_test.go` uses to build a `Deps` and a saved profile — check the top of that file first with `sed -n '1,40p' internal/tools/energy_test.go` and reuse it verbatim rather than adding a second helper.

- [ ] **Step 2: Run it and watch it fail**

```
cd ~/codes/sehaty && go test ./internal/tools/ -run 'TestTheRefusalNames|TestAnAssumedTraining|TestNothingStillClaims' 2>&1 | tail -20
```

Expected: `got.Assumed undefined` and the Mifflin assertion failing.

- [ ] **Step 3: Add the `Have` bridge**

Create `internal/tools/capability.go`:

```go
package tools

import (
	"github.com/rakasatria/sehaty/internal/capability"
	"github.com/rakasatria/sehaty/internal/storage"
)

// Have answers what is on file about someone, for the capability registry.
//
// This is the ONLY place that reads the weight log to answer that question. The log is
// the difference between a fact and an answer: a weight is measured, not volunteered, so
// it cannot come from the answered set and must not be inferred from the profile.
func Have(d Deps, p storage.Profile) capability.Have {
	weighed := false
	if d.DB != nil {
		if ws, err := d.DB.Weights(p.ID, 3650); err == nil && len(ws) > 0 {
			weighed = true
		}
	}
	return p.Have(weighed)
}
```

- [ ] **Step 4: Rewrite the head of `EstimateEnergy`**

In `internal/tools/energy.go`, add to the `EnergyEstimate` struct, next to `Provisional`:

```go
	// Assumed names what this estimate proceeded without, in the words the person gets.
	// An assumption nobody is told about is indistinguishable from a measurement, and
	// the training frequency carries most of the error in this figure.
	Assumed []string `json:"assumed,omitempty"`
```

Replace lines 149-173 (from `var missing []string` through the end of the under-18 refusal) with:

```go
	have := Have(d, p)
	if unmet, blocked := capability.Blocked("estimate_energy", have); blocked {
		return EnergyEstimate{}, fmt.Errorf(
			"cannot estimate without %s — ask for it rather than assuming; an estimate "+
				"built on a guessed input is not an estimate",
			joinAnd(readable(unmet)))
	}

	weight, _ := latestWeight(d, profileID)

	if p.Age < 18 {
		return EnergyEstimate{}, fmt.Errorf(
			"these equations are validated for adults; for someone under 18 this needs " +
				"a paediatric dietitian, not an equation")
	}
```

and just before the `return out, nil` at the end of the function, add:

```go
	for _, gap := range capability.SoftGaps("estimate_energy", have) {
		out.Assumed = append(out.Assumed, gap.Because)
	}
```

Add a small helper at the bottom of `energy.go`:

```go
// readable turns field names into the words a person would use. "weight_log" is a
// database concept; "a recorded weight" is the thing they are being asked for.
func readable(rs []capability.Requirement) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		switch r.Field {
		case capability.FieldWeightLog:
			out = append(out, "a recorded weight")
		case capability.FieldHeight:
			out = append(out, "height")
		default:
			out = append(out, string(r.Field))
		}
	}
	return out
}
```

Add `"github.com/rakasatria/sehaty/internal/capability"` to the imports of `energy.go`.

- [ ] **Step 5: Correct the tool description**

In `internal/agent/registry.go`, replace the `estimate_energy` description with:

```go
	r.add("estimate_energy",
		"Estimate how much energy they use in a day — from their age, height, sex, "+
			"latest weight and training frequency. Returns a RANGE with its uncertainty, "+
			"and says what it had to assume. You may NOT do this arithmetic yourself; "+
			"call this. It refuses when an input is missing, and the right response to "+
			"that is to ask for the missing thing, never to assume it.",
		obj(map[string]any{}),
		func(_ context.Context, p string, _ json.RawMessage) (any, error) {
			return tools.EstimateEnergy(r.deps, p)
		})
```

- [ ] **Step 6: Run the whole suite and watch it pass**

```
cd ~/codes/sehaty && go build ./... && go vet ./... && go test ./... 2>&1 | tail -25
```

Expected: all 15 packages `ok`.

- [ ] **Step 7: Prove no stale equation name survives**

```
cd ~/codes/sehaty && grep -rn "Mifflin" --include=*.go . ; echo "exit=$?"
```

Expected: no output, `exit=1`. If `energy.go` still names Mifflin-St Jeor in a doc comment describing history, that is fine only if the sentence says it is no longer used; otherwise remove it.

- [ ] **Step 8: Commit**

```bash
cd ~/codes/sehaty
git add internal/tools/ internal/agent/registry.go
git commit -m "feat(energy): refuse from the registry, and say out loud what was assumed"
```

---

## Task 5: The brief derives from the registry

The brief stops hand-listing the energy inputs and hand-writing the unknown list. Each unknown now arrives with the reason attached, so the model never composes one.

**Files:**
- Modify: `internal/agent/brief.go:66-130` (from `// The intake needs a weight` to the end)
- Test: `internal/agent/brief_test.go`

**Interfaces:**
- Consumes: `tools.Have`, `capability.Blocked`, `capability.ToAsk`, `capability.Ask`, `capability.Names`.
- Produces: no new exported names. `Brief(d tools.Deps, p storage.Profile) string` keeps its signature.

- [ ] **Step 1: Write the failing test**

Append to `internal/agent/brief_test.go`:

```go
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
```

- [ ] **Step 2: Run it and watch it fail**

```
cd ~/codes/sehaty && go test ./internal/agent/ -run 'TestEveryUnknownCarries|TestGatedQuestions|TestTheEnergyInputs' 2>&1 | tail -20
```

Expected: `the unknown list does not carry the reason "only uses things you actually have"`.

- [ ] **Step 3: Rewrite the tail of `Brief`**

In `internal/agent/brief.go`, replace everything from the comment `// The intake needs a weight` to the closing `}` of the function with:

```go
	have := tools.Have(d, p)
	unmet, blocked := capability.Blocked("estimate_energy", have)
	toAsk := capability.ToAsk(have, p.Asked())

	if !blocked && len(toAsk) == 0 {
		s.WriteString("\nCONSULTATION COMPLETE. You know everything you need. Do not ask " +
			"profile questions any more; just help, and log what they tell you.\n")
		return s.String()
	}

	// FIRST CONSULTATION. Different rules apply here from the ongoing relationship,
	// and the brief says which are in force so the prompt does not have to guess.
	s.WriteString("\nYOU ARE IN THE FIRST CONSULTATION — the intake is not finished.\n")

	if !blocked {
		s.WriteString("\nYou now have age, height, sex and a weight. Before asking anything " +
			"else, call estimate_energy and give them the result — that is what they have " +
			"been answering questions FOR, and it should arrive as soon as it can be earned " +
			"rather than at the end. Then carry on with what is still missing.\n")
	} else {
		need := []string{}
		for _, r := range unmet {
			if r.Field == capability.FieldWeightLog {
				need = append(need, "a current weight")
				continue
			}
			need = append(need, string(r.Field))
		}
		fmt.Fprintf(&s, "\nStill needed before you can estimate their energy: %s. "+
			"These come first — the estimate is the point of the intake.\n",
			strings.Join(need, ", "))
	}

	if len(toAsk) > 0 {
		s.WriteString("\nSTILL UNKNOWN, in the order to ask. The reason after each one is " +
			"the true reason — give it if they ask why, and never invent a different one:\n")
		for i, r := range toAsk {
			q, _ := capability.Ask(r.Field)
			if q.Gate != "" && blocked {
				fmt.Fprintf(&s, "  %d. %s — %s — %s\n", i+1, r.Field, q.Gate, r.Because)
				continue
			}
			fmt.Fprintf(&s, "  %d. %s — %s\n", i+1, r.Field, r.Because)
		}
		s.WriteString("During the first consultation the moment-gating above is relaxed: " +
			"they came to be assessed, so height and the rest may be asked directly.\n")
	}

	if blocked {
		for _, r := range unmet {
			if r.Field == capability.FieldWeightLog {
				s.WriteString("\nThey have never been weighed. Ask for a current weight and " +
					"log it with log_weight.\n")
				break
			}
		}
	}

	s.WriteString("\nAsk the NEXT question as soon as they answer the last one — do not wait " +
		"for something useful to do first, and do not pad between questions. One question " +
		"per message still, buttons where offer_choices has them, and skip is always a " +
		"complete answer. When the intake is done, offer to build their plan.\n")

	return s.String()
}
```

Add `"github.com/rakasatria/sehaty/internal/capability"` to the imports of `brief.go`. Remove the now-unused `weighed` variable and the `d.DB.Weights` block above it — `tools.Have` does that lookup now, in one place.

- [ ] **Step 4: Run the agent package and watch it pass**

```
cd ~/codes/sehaty && go test ./internal/agent/ -v 2>&1 | tail -40
```

Expected: PASS. `TestABareProfileIsAConsultation`, `TestTheUnknownListKeepsItsOrder`, `TestAnAnsweredProfileIsComplete` and `TestBriefOmitsFieldsRatherThanShowingThemEmpty` all pass unchanged — the mode logic did not move, only where its inputs come from.

- [ ] **Step 5: Run the whole suite**

```
cd ~/codes/sehaty && go build ./... && go vet ./... && go test ./... 2>&1 | tail -25
```

Expected: all 15 packages `ok`.

- [ ] **Step 6: Commit**

```bash
cd ~/codes/sehaty
git add internal/agent/brief.go internal/agent/brief_test.go
git commit -m "feat(brief): every question arrives with the reason the tool gives it"
```

---

## Task 6: The Mini App shows what is locked and why

The third reader. `/api/summary` gains a `locked` array; the app renders it. TypeScript computes nothing — the reasons arrive as strings from Go.

**Files:**
- Modify: `internal/dashboard/api.go` (the `summary` struct and `buildSummary`)
- Create: `web/src/components/Locked.tsx`
- Modify: `web/src/types.ts`, `web/src/App.tsx`
- Test: `internal/dashboard/api_test.go`

**Interfaces:**
- Consumes: `tools.Have`, `capability.Locked`, `capability.Ask`.
- Produces: `type lockedCapability struct{ Unlocks string; Needs []lockedNeed }`; `type lockedNeed struct{ Field, Because string }`; TS `LockedCapability` and `LockedNeed` on `Summary.locked`.

- [ ] **Step 1: Write the failing test**

Append to `internal/dashboard/api_test.go`:

```go
// The third reader of the capability registry. A person looking at their own record
// should be able to see what it cannot do yet and exactly why — not discover it by
// asking for something and being refused.
func TestTheSummaryReportsWhatIsLockedAndWhy(t *testing.T) {
	d := testDeps(t)
	p := mustProfile(t, d, storage.Profile{DisplayName: "Someone"})

	got, err := buildSummary(d, p.ID, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Locked) != 1 {
		t.Fatalf("locked = %d entries, want 1", len(got.Locked))
	}
	if got.Locked[0].Unlocks == "" {
		t.Error("the locked entry does not say what it would unlock")
	}
	if len(got.Locked[0].Needs) != 4 {
		t.Fatalf("locked on %d needs, want 4", len(got.Locked[0].Needs))
	}
	for _, n := range got.Locked[0].Needs {
		if n.Field == "" || n.Because == "" {
			t.Errorf("a lock with no field or no reason: %+v", n)
		}
	}
}

// Once every hard requirement is met, nothing is locked and the section disappears
// rather than rendering an empty box.
func TestNothingIsLockedOnceTheInputsExist(t *testing.T) {
	d := testDeps(t)
	p := mustProfile(t, d, storage.Profile{
		DisplayName: "Someone", Age: 34, HeightCm: 173, Sex: "male",
	})
	if err := d.DB.LogWeight(p.ID, storage.WeightEntry{
		Date: time.Now().Format("2006-01-02"), WeightKg: 72.3}); err != nil {
		t.Fatal(err)
	}

	got, err := buildSummary(d, p.ID, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Locked) != 0 {
		t.Errorf("still locked with every input on file: %+v", got.Locked)
	}
}
```

Reuse whatever `testDeps` / `mustProfile` helpers `internal/dashboard`'s existing tests use — check with `sed -n '1,40p' internal/dashboard/api_test.go` first and do not add a second helper.

- [ ] **Step 2: Run it and watch it fail**

```
cd ~/codes/sehaty && go test ./internal/dashboard/ -run 'TestTheSummaryReportsWhatIsLocked|TestNothingIsLocked' 2>&1 | tail -20
```

Expected: `got.Locked undefined (type summary has no field or method Locked)`.

- [ ] **Step 3: Add the field to the wire shape**

In `internal/dashboard/api.go`, add to the `summary` struct after `Limits`:

```go
	Locked    []lockedCapability `json:"locked"`
```

and after the `entry` type:

```go
// lockedCapability is something this record cannot do yet, and why.
//
// Shown rather than discovered: finding out by asking and being refused reads as the
// software being broken, while a named gap with a named reason reads as a record that
// knows what it is short of.
type lockedCapability struct {
	Unlocks string       `json:"unlocks"`
	Needs   []lockedNeed `json:"needs"`
}

type lockedNeed struct {
	Field   string `json:"field"`
	Because string `json:"because"`
}
```

In `buildSummary`, initialise `Locked: []lockedCapability{}` alongside `Recent: []entry{}`, and just before `return out, nil` add:

```go
	// The same registry the tools refuse from and the brief asks from. Three readers,
	// one declaration — a lock the person is shown cannot disagree with a refusal they
	// would get, because both are the same fact.
	for _, c := range capability.Locked(tools.Have(d.ToolDeps(), p)) {
		lc := lockedCapability{Unlocks: c.Unlocks}
		for _, n := range c.Needs {
			lc.Needs = append(lc.Needs, lockedNeed{
				Field: string(n.Field), Because: n.Because})
		}
		out.Locked = append(out.Locked, lc)
	}
```

`dashboard.Deps` does not currently carry a `tools.Deps`. Check what it holds with `grep -n "type Deps struct" -A 15 internal/dashboard/dashboard.go`. If it already has `DB` and nothing else, do NOT add a `ToolDeps()` shim — instead call `p.Have(weighed)` directly, computing `weighed` from the weights already fetched earlier in `buildSummary`:

```go
	weighed := false
	if ws, err := d.DB.Weights(profileID, 3650); err == nil && len(ws) > 0 {
		weighed = true
	}
	for _, c := range capability.Locked(p.Have(weighed)) {
		lc := lockedCapability{Unlocks: c.Unlocks}
		for _, n := range c.Needs {
			lc.Needs = append(lc.Needs, lockedNeed{
				Field: string(n.Field), Because: n.Because})
		}
		out.Locked = append(out.Locked, lc)
	}
```

Use the second form unless `dashboard.Deps` already carries a `tools.Deps`. Add `"github.com/rakasatria/sehaty/internal/capability"` to the imports. Delete the `var _ = storage.Profile{}` line at the bottom of the file if `storage` is now genuinely used.

- [ ] **Step 4: Run it and watch it pass**

```
cd ~/codes/sehaty && go test ./internal/dashboard/ 2>&1 | tail -20
```

Expected: `ok`.

- [ ] **Step 5: Add the TypeScript type**

In `web/src/types.ts`, add to `Summary` after `limitations`:

```ts
  locked: LockedCapability[] // empty when nothing is gated; never null
```

and at the bottom of the file:

```ts
/**
 * Something the record cannot do yet. Both strings arrive ready to render — the
 * client picks neither the wording nor which needs to show.
 */
export type LockedCapability = {
  unlocks: string
  needs: LockedNeed[]
}

export type LockedNeed = {
  field: string
  because: string
}
```

- [ ] **Step 6: Add the component**

Create `web/src/components/Locked.tsx`:

```tsx
import type { LockedCapability } from '../types'

/**
 * What this record cannot do yet, and why.
 *
 * Renders nothing at all when nothing is locked — an empty "everything is unlocked"
 * box is a box that exists only to be dismissed. Every string here comes from the Go
 * capability registry; this component chooses no wording and hides no need.
 */
export function Locked({ items }: { items: LockedCapability[] }) {
  if (items.length === 0) return null

  return (
    <section className="locked" aria-labelledby="locked-heading">
      <h2 id="locked-heading">Belum bisa</h2>
      {items.map((c) => (
        <article key={c.unlocks} className="locked-item">
          <h3>{c.unlocks}</h3>
          <ul>
            {c.needs.map((n) => (
              <li key={n.field}>
                <b>{n.field}</b>
                <span>{n.because}</span>
              </li>
            ))}
          </ul>
        </article>
      ))}
    </section>
  )
}
```

- [ ] **Step 7: Render it**

In `web/src/App.tsx`, import `{ Locked }` from `./components/Locked` and render `<Locked items={summary.locked} />` after the existing sections. Then add to `web/src/styles/app.css`:

```css
.locked {
  margin-top: var(--gap-lg, 2rem);
}
.locked-item {
  border-top: 1px solid var(--rule);
  padding-top: 0.75rem;
  margin-top: 0.75rem;
}
.locked-item h3 {
  font: inherit;
  font-weight: 600;
  margin: 0 0 0.4rem;
}
.locked-item ul {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.5rem;
}
.locked-item li {
  display: grid;
  gap: 0.15rem;
}
.locked-item b {
  font-weight: 600;
}
.locked-item span {
  color: var(--faint);
}
```

If `--gap-lg`, `--rule` or `--faint` are not in `web/src/styles/tokens.css`, use the token names that file actually defines — check it first and do not introduce new ones.

- [ ] **Step 8: Build the app and embed it**

```
cd ~/codes/sehaty/web && npm run build && cd .. && go build ./... && go test ./... 2>&1 | tail -25
```

Expected: the Vite build succeeds into `internal/dashboard/webdist`, and all 15 packages `ok`.

- [ ] **Step 9: Commit**

```bash
cd ~/codes/sehaty
git add internal/dashboard/ web/
git commit -m "feat(dashboard): show what the record cannot do yet, and why"
```

---

## Task 7: The soul split

The 377-line `SystemPrompt` constant becomes markdown files with provenance. This task changes NO text: the golden file captured in step 1 is the proof.

**Files:**
- Create: `internal/agent/testdata/prompt.golden`
- Create: `internal/agent/soul.go`, `internal/agent/soul/intent.md`, `internal/agent/soul/limits.md`, `internal/agent/soul/manner.md`, `internal/agent/soul/skills/*.md` (six)
- Modify: `internal/agent/agent.go` (delete the constant, add `var SystemPrompt`)
- Test: `internal/agent/soul_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `func soulText() string`; `func Compose() string`; `var SystemPrompt string`; `func strip(md string) string`.

- [ ] **Step 1: Capture the golden BEFORE touching anything**

This must happen first — it is the only record of what the prompt said. On hrmdev01:

```bash
cd ~/codes/sehaty
mkdir -p internal/agent/testdata
cat > /tmp/dumpgolden_test.go <<'EOF'
package agent

import "testing"
import "os"

func TestDumpGolden(t *testing.T) {
	if err := os.WriteFile("testdata/prompt.golden", []byte(SystemPrompt), 0o644); err != nil {
		t.Fatal(err)
	}
}
EOF
cp /tmp/dumpgolden_test.go internal/agent/dumpgolden_test.go
go test ./internal/agent/ -run TestDumpGolden
rm internal/agent/dumpgolden_test.go
wc -c internal/agent/testdata/prompt.golden
git add internal/agent/testdata/prompt.golden
git commit -m "test(agent): capture the prompt as it stands, so the split can be proved lossless"
```

Expected: a file of roughly 17,000 bytes, committed.

- [ ] **Step 2: Write the failing test**

Create `internal/agent/soul_test.go`:

```go
package agent

import (
	"os"
	"strings"
	"testing"
)

// The split moves text; it does not rewrite it. This is the proof, and it is the reason
// the golden file was committed before a single line was moved: a refactor of the most
// heavily-evidenced part of this system is only safe if a machine can tell you nothing
// changed.
func TestTheSplitChangedNoText(t *testing.T) {
	want, err := os.ReadFile("testdata/prompt.golden")
	if err != nil {
		t.Fatal(err)
	}
	got := soulText()
	if got != string(want) {
		t.Errorf("the composed soul differs from the prompt it replaced\n"+
			"got  %d bytes\nwant %d bytes\nfirst difference at byte %d",
			len(got), len(want), firstDiff(got, string(want)))
	}
}

// Every claim carries its source, and the model never sees one. A model that can read
// its own citations starts performing them — "research shows" in a health record is
// exactly the sentence this project must never produce.
func TestProvenanceIsStrippedBeforeTheModelSeesIt(t *testing.T) {
	if strings.Contains(SystemPrompt, "<!--") {
		t.Error("an HTML comment reached the prompt")
	}
	for _, tag := range []string{"PAPER:", "RULE:", "DATA:", "JUDGEMENT:"} {
		if strings.Contains(SystemPrompt, tag) {
			t.Errorf("the provenance tag %q reached the prompt", tag)
		}
	}
	if strings.Contains(SystemPrompt, "research shows") {
		t.Error("the prompt invites Sehaty to cite research")
	}
}

// The tags have to exist to be stripped. A soul with no provenance is a soul nobody can
// audit, and JUDGEMENT is the tag that earns its keep: it marks a rule as somebody's
// confident guess, and during this design five such guesses were checked and all five
// were wrong.
func TestTheSourceFilesActuallyCarryProvenance(t *testing.T) {
	files, err := soulFS.ReadDir("soul/skills")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 6 {
		t.Fatalf("expected six behaviour skills, found %d", len(files))
	}

	tagged, judgement := 0, 0
	for _, name := range []string{"soul/intent.md", "soul/limits.md", "soul/manner.md"} {
		b, err := soulFS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		tagged += strings.Count(string(b), "<!--")
		judgement += strings.Count(string(b), "JUDGEMENT:")
	}
	for _, f := range files {
		b, err := soulFS.ReadFile("soul/skills/" + f.Name())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "<!--") {
			t.Errorf("%s carries no provenance at all", f.Name())
		}
		tagged += strings.Count(string(b), "<!--")
		judgement += strings.Count(string(b), "JUDGEMENT:")
	}
	if tagged < 20 {
		t.Errorf("only %d claims carry a source; the soul is mostly unattributed", tagged)
	}
	if judgement == 0 {
		t.Error("nothing is marked JUDGEMENT, which means either perfect evidence or a dishonest tagger")
	}
}

// Stripping must remove the comment and leave the line, including its trailing content,
// without eating the newline that separates two rules.
func TestStripRemovesOnlyTheComment(t *testing.T) {
	in := "Praise the person.   <!-- PAPER: +0.33 -->\nNever a token.\n"
	want := "Praise the person.\nNever a token.\n"
	if got := strip(in); got != want {
		t.Errorf("strip(%q) = %q, want %q", in, got, want)
	}

	multi := "A rule.\n<!--\nPAPER: something\nover two lines\n-->\nAnother rule.\n"
	if got := strip(multi); strings.Contains(got, "PAPER") {
		t.Errorf("a multi-line comment survived: %q", got)
	}
}

func firstDiff(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
```

- [ ] **Step 3: Run it and watch it fail**

```
cd ~/codes/sehaty && go test ./internal/agent/ -run 'TestTheSplit|TestProvenance|TestTheSourceFiles|TestStrip' 2>&1 | tail -20
```

Expected: `undefined: soulText`, `undefined: soulFS`, `undefined: strip`.

- [ ] **Step 4: Write the composer**

Create `internal/agent/soul.go`:

```go
package agent

import (
	"embed"
	"regexp"
	"sort"
	"strings"
)

//go:embed soul
var soulFS embed.FS

// soulOrder is the order the files are composed in, and it is the order the prompt had.
//
// Not alphabetical and not discovered: the sequence is load-bearing. What Sehaty IS comes
// before what it will not do, which comes before how it sounds, and the behaviour skills
// come last because each one assumes all three.
var soulOrder = []string{
	"soul/intent.md",
	"soul/limits.md",
	"soul/skills/taking-a-food-entry.md",
	"soul/skills/asking-a-question.md",
	"soul/skills/declining-a-target.md",
	"soul/skills/after-a-gap.md",
	"soul/skills/when-they-push-back.md",
	"soul/manner.md",
	"soul/skills/noticing-progress.md",
}

// soulText is the soul with its provenance stripped: what the model is given, and
// nothing else.
//
// Every claim in those files carries its source in an HTML comment, visible to anyone
// reading the file and invisible here. Stripping saves tokens and prevents something
// worse: a model that can see its citations starts performing them, and "research shows"
// in a health record is a sentence this project must never produce. The papers decide
// what Sehaty does and stay out of what it says.
func soulText() string {
	var b strings.Builder
	for _, name := range soulOrder {
		raw, err := soulFS.ReadFile(name)
		if err != nil {
			// A missing file is a build-time mistake, not a runtime condition: the
			// files are embedded, so if one is absent the binary was built wrong.
			panic("soul: " + name + ": " + err.Error())
		}
		b.WriteString(strip(string(raw)))
	}
	return b.String()
}

// comment matches an HTML comment, including one spanning several lines.
var comment = regexp.MustCompile(`(?s)[ \t]*<!--.*?-->`)

// strip removes provenance comments and the whitespace that preceded them, leaving the
// claim and its newline exactly as they were.
func strip(md string) string {
	out := comment.ReplaceAllString(md, "")
	// A comment on a line of its own leaves a blank line behind. Collapse only that
	// case: real blank lines between sections are structure the prompt relies on.
	out = strings.ReplaceAll(out, "\n\n\n", "\n\n")
	return out
}

// Compose builds the system prompt: the soul, then the section generated from the
// capability registry.
//
// Two sources, and the boundary between them is the point. What Sehaty is and how it
// behaves is written by a person and reviewed by a person. What it can and cannot do,
// and why each question is asked, is generated from the tools — so it cannot drift from
// them, and nobody has to remember to update a paragraph when a tool gains an input.
func Compose() string {
	return soulText()
}

// SystemPrompt is what the model is given. Computed once at init from the embedded soul.
var SystemPrompt = Compose()

var _ = sort.Strings // retained for soulOrder maintenance; remove if unused
```

Delete the trailing `var _ = sort.Strings` line and the `"sort"` import — they are there only to name the temptation. Composition order is explicit, never sorted.

- [ ] **Step 5: Split the prompt into the files**

Open `internal/agent/agent.go` and cut the body of `SystemPrompt` (from the line after `const SystemPrompt = \`` to the closing backtick) into the files below. **Copy the text byte for byte.** Do not reword, do not fix typos, do not re-wrap. The only additions are the provenance comments.

| file | sections to move, in this order |
|---|---|
| `soul/intent.md` | `You are Sehaty, a personal health record…` opening line, `WHAT YOU ARE`, `LANGUAGE`, `WHAT YOU ARE FOR` |
| `soul/limits.md` | `THE RULE THAT MATTERS MOST`, `WHAT YOU WILL NOT SUGGEST` |
| `soul/skills/taking-a-food-entry.md` | `FOOD`, `FLAGGED FOODS` |
| `soul/skills/asking-a-question.md` | `THE FIRST CONSULTATION`, `AFTERWARDS`, `GETTING TO KNOW THEM`, `STARTING THE QUESTIONS OVER`, `WHY EACH ONE, IF THEY ASK`, `HOW TO ASK, AND HOW TO TAKE THE ANSWER` |
| `soul/skills/declining-a-target.md` | `ESTIMATING ENERGY` |
| `soul/skills/after-a-gap.md` | the return-as-continuation paragraphs, currently inside `WHAT YOU ARE FOR` — move ONLY the three paragraphs beginning `A missed day does not derail anything`, `What DOES predict someone drifting away` and `Treat a return as continuation` |
| `soul/skills/when-they-push-back.md` | `WHAT NOT TO DO, WHICH MATTERS MORE THAN WHAT TO DO` |
| `soul/manner.md` | `MANNER`, `STYLE` |
| `soul/skills/noticing-progress.md` | `ENCOURAGEMENT` |

Because `after-a-gap.md` and `noticing-progress.md` lift paragraphs out of the middle of larger sections, `soulOrder` above places them exactly where those paragraphs sat. **The golden test in Step 2 is what tells you whether you got the boundaries right** — run it after every file and read the byte offset it reports.

Each file keeps its section heading as the first line, unchanged (`WHAT YOU ARE`, not `## What you are`) — the headings are part of the text the golden covers.

Add provenance comments to the claims. At minimum these, each on the line it belongs to:

```markdown
Do not disagree, argue, correct, shame, criticise, or give advice nobody asked for.   <!-- PAPER: MI-nonadherent behaviours; Butler 2013 N=1,827 — OR 12.44 on recall, OR 1.12 (null) on behaviour -->
Do not do pros-and-cons with someone who is undecided.   <!-- PAPER: decisional balance reduces commitment in the ambivalent -->
You may say well done, and mean it.   <!-- PAPER: verbal praise d=+0.33; tangible tokens d=-0.34 -->
Never praise the act of LOGGING, though.   <!-- JUDGEMENT: untested; follows from the praise/token split but nobody has tested it on logging -->
a quiet day is never remarked on.   <!-- PAPER: Kirchner 2012, 1,001 lapses — guilt did NOT predict relapse; self-efficacy collapse did -->
Favour consistency over intensity.   <!-- PAPER: Burnet 2020 meta-analysis, -3.3% adherence per intensity increase -->
Treat a return as continuation, never resumption.   <!-- PAPER: Kirchner 2012; JUDGEMENT: the specific wording is untested -->
Three things reliably help someone take something on   <!-- PAPER: Deci et al. 1994 — rationale, acknowledgement, choice -->
make them feel CAPABLE before anything else   <!-- PAPER: competence is the strongest single SDT factor for maintenance -->
Give real choices rather than softened wording.   <!-- PAPER: choice helps measurably; phrasing alone does almost nothing -->
Never invent a number.   <!-- RULE: Raka, founding -->
No emoji, no exclamation marks.   <!-- RULE: Raka -->
No supplements and no medication, ever   <!-- RULE: Raka; the teaching/prescribing line -->
You are not a therapist and you do not do therapy.   <!-- PAPER: hypnotherapy safety data reassuring, efficacy data fail; DECISION: declined on efficacy, not on safety -->
It is a RANGE from a population equation, and an individual can sit twenty percent either side.   <!-- DATA: ±25% is the honest band for an estimated TDEE with no Indonesian validation; the prompt says twenty and the tool returns the real one -->
Ask the way a person would, in their language   <!-- RULE: Raka — Jakarta Indonesian, not textbook -->
```

Add more wherever a claim is making an empirical assertion. Anything you cannot source, tag `JUDGEMENT:` and say so honestly — an over-confident `PAPER:` tag is worse than an honest `JUDGEMENT:` one, because the fortnightly evidence job prioritises `JUDGEMENT` for checking and will never re-examine a claim you dressed up.

- [ ] **Step 6: Replace the constant**

In `internal/agent/agent.go`, delete `const SystemPrompt = \`…\`` entirely, including the doc comment above it, and leave nothing in its place — `SystemPrompt` now lives in `soul.go`. The `strings` import stays; check whether `fmt` is still used and remove it if not.

- [ ] **Step 7: Run and iterate until the golden matches**

```
cd ~/codes/sehaty && go test ./internal/agent/ -run TestTheSplitChangedNoText 2>&1 | tail -20
```

Expected once the boundaries are right: PASS. While it fails it reports the byte offset of the first difference — use it:

```
cd ~/codes/sehaty && go run ./cmd/... 2>/dev/null; head -c 400 internal/agent/testdata/prompt.golden | tail -c 200
```

More useful: add a temporary `os.WriteFile("/tmp/got.txt", []byte(soulText()), 0o644)` at the top of the golden test and `diff /tmp/got.txt internal/agent/testdata/prompt.golden | head -40`. Remove it once green.

- [ ] **Step 8: Run the whole suite**

```
cd ~/codes/sehaty && go build ./... && go vet ./... && go test ./... 2>&1 | tail -25
```

Expected: all 15 packages `ok`. `TestSystemPromptStatesTheRulesThatMatter`, `TestExpertiseDoesNotBecomePrescription`, `TestTheModelMayNotDoTheArithmeticItself` and `TestThePromptAsksOneThingAtATime` pass **unchanged** — they assert exact substrings, and the split preserved every one.

- [ ] **Step 9: Commit**

```bash
cd ~/codes/sehaty
git add internal/agent/
git commit -m "refactor(agent): the prompt becomes a soul, one behaviour per file, every claim sourced"
```

---

## Task 8: The question sections are generated, not maintained

The last hand-kept copy of the registry goes. `WHY EACH ONE, IF THEY ASK` and `HOW TO ASK, AND HOW TO TAKE THE ANSWER` are two lists of ten items that must agree with the ten questions — and nothing made them agree except somebody remembering. Now they are generated.

**Files:**
- Create: `internal/capability/describe.go`
- Modify: `internal/agent/soul/skills/asking-a-question.md` (delete the two sections)
- Modify: `internal/agent/soul.go` (`Compose` appends the generated section)
- Modify: `internal/agent/testdata/prompt.golden` (regenerate — the only task in this plan that legitimately moves it)
- Test: `internal/capability/describe_test.go`, `internal/agent/soul_test.go`

**Interfaces:**
- Consumes: `AskOrder()`, `Known()`, `Question`, `Capability`.
- Produces: `func Describe() string`.

- [ ] **Step 1: Write the failing test**

Create `internal/capability/describe_test.go`:

```go
package capability

import (
	"strings"
	"testing"
)

// The generated section must carry every question, its reason and its wording. It is
// replacing two hand-kept lists that had to agree with this registry and were only kept
// agreeing by somebody remembering.
func TestDescribeCarriesEveryQuestion(t *testing.T) {
	got := Describe()
	for _, q := range AskOrder() {
		if !strings.Contains(got, string(q.Field)) {
			t.Errorf("the generated section omits the question %q", q.Field)
		}
		if !strings.Contains(got, q.Because) {
			t.Errorf("the generated section omits the reason for %q", q.Field)
		}
		if !strings.Contains(got, q.Ask) {
			t.Errorf("the generated section omits the wording for %q", q.Field)
		}
	}
}

// What a tool needs, and what it gives back for it, both appear — so the model can
// answer "why are you asking" with the consequence rather than a plausible sentence.
func TestDescribeCarriesWhatEachToolUnlocks(t *testing.T) {
	got := Describe()
	for _, c := range Known() {
		if !strings.Contains(got, c.Unlocks) {
			t.Errorf("the generated section does not say %q unlocks %q", c.Tool, c.Unlocks)
		}
	}
}

// The closing rule survived the move: nothing here is calculated into a target.
func TestDescribeKeepsTheClosingRule(t *testing.T) {
	got := Describe()
	for _, want := range []string{
		"nothing is calculated from it",
		"everything still works",
		"personalise",
	} {
		if want == "personalise" {
			if strings.Contains(got, want) {
				t.Error("the generated section offers a reason nobody believes")
			}
			continue
		}
		if !strings.Contains(got, want) {
			t.Errorf("the generated section no longer says %q", want)
		}
	}
}
```

Append to `internal/agent/soul_test.go`:

```go
// The two question sections are generated now. If they are still in the file as well,
// there are two copies again and they will disagree.
func TestTheQuestionSectionsAreNotAlsoWrittenByHand(t *testing.T) {
	b, err := soulFS.ReadFile("soul/skills/asking-a-question.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"WHY EACH ONE, IF THEY ASK", "Biasa latihan pake apa"} {
		if strings.Contains(string(b), gone) {
			t.Errorf("%q is still hand-written in the soul as well as generated", gone)
		}
	}
}

// And they must still reach the model.
func TestTheGeneratedQuestionsReachThePrompt(t *testing.T) {
	for _, want := range []string{
		"Biasa latihan pake apa",
		"only uses things you actually have",
		"an estimate of how much energy you use in a day",
	} {
		if !strings.Contains(SystemPrompt, want) {
			t.Errorf("the prompt does not carry %q", want)
		}
	}
}
```

- [ ] **Step 2: Run them and watch them fail**

```
cd ~/codes/sehaty && go test ./internal/capability/ ./internal/agent/ 2>&1 | tail -20
```

Expected: `undefined: Describe`, and the soul test failing because the sections are still hand-written.

- [ ] **Step 3: Write the generator**

Create `internal/capability/describe.go`:

```go
package capability

import (
	"fmt"
	"strings"
)

// Describe writes the part of the system prompt that must never drift from the tools.
//
// It replaces two hand-kept lists — one of reasons, one of question wordings — that had
// to agree with this registry and stayed agreeing only because somebody remembered. A
// tool gaining an input used to require three edits in three files; now it requires one.
func Describe() string {
	var b strings.Builder

	b.WriteString("\nWHAT EACH ANSWER UNLOCKS\n")
	b.WriteString("These come from the tools themselves. If someone asks what a question is for, " +
		"this is the true answer, and the only one you may give.\n\n")
	for _, c := range known {
		fmt.Fprintf(&b, "  %s — %s\n", c.Tool, c.Unlocks)
		for _, n := range c.Needs {
			mark := "needs"
			if !n.Hard {
				mark = "assumes if unknown"
			}
			fmt.Fprintf(&b, "      %s %s: %s\n", mark, n.Field, n.Because)
		}
	}

	b.WriteString("\nWHY EACH ONE, IF THEY ASK\n")
	b.WriteString("Name a concrete consequence in the record, then say what it is not. Never say\n" +
		"\"to personalise your experience\" — that is not true here and they will smell it.\n\n")
	for _, q := range askOrder {
		fmt.Fprintf(&b, "  %-12s %s\n", q.Field, q.Because)
	}
	b.WriteString("\nThen close the answer the same way every time: nothing is calculated from it — no\n" +
		"calorie target, no macros; those come from their dietitian, not from you. And if they\n" +
		"would rather not say, everything still works.\n")

	b.WriteString("\nHOW TO ASK\n")
	b.WriteString("Ask the way a person would, in their language, in the register they are using.\n" +
		"Jakarta Indonesian, not textbook Indonesian: \"Btw, umurmu berapa?\" not \"Mohon\n" +
		"informasikan usia Anda.\" Some shapes that work:\n\n")
	for _, q := range askOrder {
		fmt.Fprintf(&b, "  %-12s %s\n", q.Field, q.Ask)
	}

	b.WriteString("\nSome must not be asked cold. Each has a moment that makes it obvious instead:\n\n")
	for _, q := range askOrder {
		if q.Gate == "" {
			continue
		}
		fmt.Fprintf(&b, "  %-12s %s\n", q.Field, q.Gate)
	}

	return b.String()
}
```

- [ ] **Step 4: Delete the hand-written sections and append the generated one**

Remove `WHY EACH ONE, IF THEY ASK` and the ten-line table under `HOW TO ASK, AND HOW TO TAKE THE ANSWER` from `internal/agent/soul/skills/asking-a-question.md`. **Keep** the paragraphs after that table — `Restate what you saved so a mistake is visible…` and `A decline gets two to six words…` — those are behaviour, not registry data. Rename the remaining heading from `HOW TO ASK, AND HOW TO TAKE THE ANSWER` to `HOW TO TAKE THE ANSWER`.

Then in `internal/agent/soul.go`:

```go
func Compose() string {
	return soulText() + capability.Describe()
}
```

and add `"github.com/rakasatria/sehaty/internal/capability"` to its imports.

- [ ] **Step 5: Update the golden test to cover only the hand-written half**

In `internal/agent/soul_test.go`, `TestTheSplitChangedNoText` compares `soulText()`, which is now smaller than the golden by exactly the two deleted sections. Regenerate the golden and **read the diff before accepting it**:

```bash
cd ~/codes/sehaty
cp internal/agent/testdata/prompt.golden /tmp/before.golden
cat > internal/agent/dumpgolden_test.go <<'EOF'
package agent

import "os"
import "testing"

func TestDumpGolden(t *testing.T) {
	if err := os.WriteFile("testdata/prompt.golden", []byte(soulText()), 0o644); err != nil {
		t.Fatal(err)
	}
}
EOF
go test ./internal/agent/ -run TestDumpGolden
rm internal/agent/dumpgolden_test.go
diff /tmp/before.golden internal/agent/testdata/prompt.golden
```

Expected in the diff: ONLY the two deleted sections and the heading rename. If anything else appears, a boundary in Task 7 was wrong — fix it rather than accepting the golden.

- [ ] **Step 6: Run the whole suite**

```
cd ~/codes/sehaty && go build ./... && go vet ./... && go test ./... 2>&1 | tail -25
```

Expected: all 15 packages `ok`, including the four original prompt tests — the generated text still contains `Biasa latihan pake apa` and the reasons, so nothing that asserted on them broke.

- [ ] **Step 7: Prove there is exactly one copy**

```
cd ~/codes/sehaty && grep -rn "Biasa latihan pake apa" --include=*.go --include=*.md . | wc -l
```

Expected: `1` — only `internal/capability/capability.go`.

- [ ] **Step 8: Commit**

```bash
cd ~/codes/sehaty
git add internal/capability/ internal/agent/
git commit -m "feat(capability): the reasons and the wordings are generated from the tools"
```

---

## Task 9: Deploy and verify against the running service

Nothing in this plan is finished until it runs on srvdev01. The service is `sehaty` on 10.254.1.105.

**Files:** none changed.

- [ ] **Step 1: Confirm the tree is green and clean**

```
cd ~/codes/sehaty && git status --short && go test ./... 2>&1 | tail -20
```

Expected: no output from `git status`, 15 `ok` lines.

- [ ] **Step 2: Build and deploy using the project's existing deploy path**

Find it first — do not invent one:

```
cd ~/codes/sehaty && ls Makefile scripts/ 2>/dev/null && grep -rn "srvdev01\|10.254.1.105" Makefile scripts/ docs/ 2>/dev/null | head
```

Use whatever that reveals. Do not hand-roll an scp.

- [ ] **Step 3: Verify the service came back**

```
ssh srvdev01 'systemctl is-active sehaty && journalctl -u sehaty -n 20 --no-pager'
```

Expected: `active`, and no panic in the log. A `panic: soul:` line means an embedded file is missing from the binary — check `//go:embed soul` matched the directory.

- [ ] **Step 4: Verify the third reader end to end**

Register a throwaway profile through Telegram, open the Mini App, and confirm the "Belum bisa" section lists the energy estimate and its four missing inputs. Then log a weight and give age, height and sex, and confirm the section disappears.

Report what you saw. Do not report this task complete on the basis that the code compiles.

- [ ] **Step 5: Write the vault note**

Append to `~/Notes/Projects/sehaty-state.md` on the Mac: what changed, what the three readers are, and that `storage.Assessment` is gone. Raka's standing rule — finished work goes into the vault, not just the chat.

---

## Self-Review

**Spec coverage.**

| spec requirement | task |
|---|---|
| §2 `Requirement` / `Capability` types | 1 |
| §2 the tool/hard/soft/unlocks table | 1 (minus `draft_programme`, deferred and flagged) |
| §2 three readers, one registry | 4 (tool), 5 (brief), 6 (Mini App) |
| §2 `storage.Assessment` deleted, `Missing()` = unmet union | 3 |
| §2 hard refuses, soft proceeds and names the gap | 2, 4 |
| §2 rationale structural, true by construction | 1, 8 |
| §4 `intent.md` / `limits.md` / `skills/*.md`, `go:embed` | 7 |
| §4 provenance tags, stripped before the model | 7 |
| §4 never says "research shows" | 7 |
| §4 one soul, not one per profile | 7 (single embedded FS; no per-profile path exists) |
| §4 generated capabilities | 8 |
| §1 model never produces a number | 4 (arithmetic stays in Go; the tool description keeps the prohibition) |
| §1 `null` is not zero | 6 (`locked` serialises as `[]`, never null) |

Not covered here, by design: §3 programme, §5 evidence refresh, §6 administration, §7 scheduler. Those are order-of-work items 3–6 and get their own plans.

**Placeholders.** None. Every code step carries the code. Three steps say "check the existing helper first" rather than inventing a fixture name — that is a real instruction with a real command, not a deferral.

**Type consistency.** `Field`, `Question`, `Requirement`, `Capability`, `Have`, `Asked` are defined in Task 1–2 and used with those exact names in 3, 4, 5, 6 and 8. `Have(d Deps, p storage.Profile)` in `tools` and `(p Profile) Have(weighed bool)` in `storage` are different functions with the same name in different packages; the first calls the second, and Task 4 defines both in one step so the pairing is visible. `soulText()` is compared against the golden; `Compose()` is `soulText() + Describe()`; `SystemPrompt` is `Compose()`. Task 7's golden covers `soulText()`, which is why Task 8 can change `Compose()` without invalidating it.

**One risk worth naming.** Task 7 is a 377-line hand-split with a byte-exact test. It will take several iterations to land, and the temptation when the diff will not close is to edit the golden. Do not. The golden is the only evidence that the most heavily-evidenced part of this system survived the move intact.
