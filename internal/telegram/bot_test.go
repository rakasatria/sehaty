package telegram

import (
	"testing"

	"github.com/mymmrac/telego"

	"github.com/rakasatria/sehaty/internal/agent"
	"github.com/rakasatria/sehaty/internal/storage"
	"github.com/rakasatria/sehaty/internal/tools"
)

func TestBestPhotoTakesTheLargestRendition(t *testing.T) {
	got := bestPhoto([]telego.PhotoSize{
		{FileID: "thumb", FileSize: 1200},
		{FileID: "full", FileSize: 480000},
		{FileID: "mid", FileSize: 34000},
	})
	if got.FileID != "full" {
		t.Fatalf("chose %+v, want the largest", got)
	}
}

func TestDisabledWithoutAToken(t *testing.T) {
	if (&Bot{}).Enabled() {
		t.Fatal("reported enabled with no token")
	}
	if !(&Bot{Token: "123:abc"}).Enabled() {
		t.Fatal("reported disabled with a token")
	}
}

// The bot answers in whatever language it is written to, but this copy is static
// and reaches someone who has so far sent nothing but a passphrase. Indonesian is
// the right default here, and it must stay consistent with the command menu, the
// assessment questions and the Mini App — all of which are Indonesian.
func TestHelpCopyIsIndonesianAndPromisesNoGuessing(t *testing.T) {
	h := help(profileFixture())
	for _, want := range []string{"Voice note", "foto makanan", "/dash", "aku nggak nebak"} {
		if !contains(h, want) {
			t.Errorf("help does not mention %q", want)
		}
	}
	// Two screens of text is a poor first thing to say to someone.
	if len(h) > 700 {
		t.Errorf("help is %d bytes — too long to read on arrival", len(h))
	}
}

// Without a model the bot must say it cannot converse, not imitate one badly.
func TestCannotConverseIsHonestAndShort(t *testing.T) {
	u := cannotConverse("something long that the bot cannot possibly understand at all")
	if !contains(u, "no language model is configured") {
		t.Error("the fallback hides why it cannot answer")
	}
	if !contains(u, "berat") {
		t.Error("the fallback does not say what still works")
	}
	if len(u) > 400 {
		t.Error("the fallback is a wall of text")
	}
}

// The model may offer buttons for any question in agent.ChoiceQuestions. If the catalogue
// does not have that question, the offer silently does nothing and the person is left
// looking at a question with no way to answer it except typing an option they were never
// shown — because the prompt told the model NOT to list them.
func TestEveryOfferableQuestionHasButtons(t *testing.T) {
	for _, q := range agent.ChoiceQuestions {
		cs, ok := choices[q]
		if !ok {
			t.Errorf("the model may offer %q but there are no buttons for it", q)
			continue
		}
		if len(cs.Choices) == 0 {
			t.Errorf("%q has an empty button set", q)
		}
		for _, c := range cs.Choices {
			if c.Label == "" {
				t.Errorf("%q has an unlabelled button", q)
			}
			if c.apply == nil {
				t.Errorf("%q button %q does nothing when tapped", q, c.Label)
			}
		}
	}
}

// A button writes a value straight into UpdateProfile without a model in between, so a
// typo in the catalogue is not a bad answer — it is a validation error the person sees
// after tapping a button the bot itself drew. Every one of them is exercised here.
func TestEveryButtonWritesAValueTheProfileAccepts(t *testing.T) {
	d := testDeps(t)
	p, err := tools.Register(d, "probe", "buttons", "Probe", "pw", "pw")
	if err != nil {
		t.Fatal(err)
	}
	for q, cs := range choices {
		for _, c := range cs.Choices {
			args := tools.UpdateProfileArgs{Profile: p.ID}
			c.apply(&args)
			if _, err := tools.UpdateProfile(d, args); err != nil {
				t.Errorf("%s / %q is rejected by update_profile: %v", q, c.Label, err)
			}
		}
	}
}

// Tapping an answer must retire the question, or the buttons come back tomorrow.
func TestTappingAnAnswerRetiresTheQuestion(t *testing.T) {
	d := testDeps(t)
	p, err := tools.Register(d, "probe", "retire", "Probe", "pw", "pw")
	if err != nil {
		t.Fatal(err)
	}
	for q, cs := range choices {
		args := tools.UpdateProfileArgs{Profile: p.ID}
		cs.Choices[0].apply(&args)
		updated, err := tools.UpdateProfile(d, args)
		if err != nil {
			t.Fatalf("%s: %v", q, err)
		}
		if inList(updated.Missing(), q) {
			t.Errorf("%q is still unanswered after its button was tapped", q)
		}
	}
}

// And Skip must retire it just as firmly. A decline that is not recorded is a question
// that follows someone around.
func TestSkipRetiresTheQuestionToo(t *testing.T) {
	d := testDeps(t)
	p, err := tools.Register(d, "probe", "skip", "Probe", "pw", "pw")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range agent.ChoiceQuestions {
		updated, err := tools.UpdateProfile(d, tools.UpdateProfileArgs{
			Profile: p.ID, Declined: []string{q}})
		if err != nil {
			t.Fatalf("declining %q was rejected: %v", q, err)
		}
		if inList(updated.Missing(), q) {
			t.Errorf("%q came back after Skip", q)
		}
	}
}

func inList(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

var _ = storage.Profile{}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
