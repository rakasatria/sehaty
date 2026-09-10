package tools

import (
	"strings"
	"testing"
)

// The profile argument is a privacy boundary supplied by a language model. An opaque
// failure invites a retry with the same bad value; naming the real profiles turns it into
// a one-step self-correction.
func TestUnknownProfileErrorNamesTheRealOnes(t *testing.T) {
	d := foodDeps(t) // registers "raka"

	for _, call := range []struct {
		name string
		run  func() error
	}{
		{"log_food", func() error {
			_, err := LogFood(d, LogFoodArgs{Profile: "rakka", Food: "AR001", Grams: 100})
			return err
		}},
		{"put_document", func() error {
			_, err := PutDocument(d, PutDocumentArgs{Profile: "rakka", Key: "diet",
				Body: "x", CreateNew: true})
			return err
		}},
		{"progress", func() error {
			_, err := Progress(d, "rakka", 7)
			return err
		}},
		{"plan_session", func() error {
			_, err := PlanSession(d, "rakka", "full", 45)
			return err
		}},
	} {
		err := call.run()
		if err == nil {
			t.Errorf("%s accepted an unregistered profile", call.name)
			continue
		}
		if !strings.Contains(err.Error(), "raka") {
			t.Errorf("%s error does not name the registered profiles: %v", call.name, err)
		}
	}
}

// Nothing may auto-create a profile from an unknown id — that is how a typo silently
// becomes a second person's empty record.
func TestNoToolAutoCreatesAProfile(t *testing.T) {
	d := foodDeps(t)
	_, _ = LogFood(d, LogFoodArgs{Profile: "ghost", Food: "AR001", Grams: 100})
	_, _ = PutDocument(d, PutDocumentArgs{Profile: "ghost", Key: "diet", Body: "x", CreateNew: true})

	all, err := d.DB.ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range all {
		if p.ID == "ghost" {
			t.Fatal("a tool created a profile from an unknown id")
		}
	}
}
