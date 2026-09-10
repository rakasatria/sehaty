package telegram

import "testing"

// Telegram sends several renditions, smallest first. A meal photo is stored once and
// looked at later; a thumbnail cannot be un-shrunk.
func TestBestPhotoTakesTheLargestRendition(t *testing.T) {
	m := &Message{Photo: []File{
		{FileID: "thumb", FileSize: 1200},
		{FileID: "full", FileSize: 480000},
		{FileID: "mid", FileSize: 34000},
	}}
	got := m.BestPhoto()
	if got == nil || got.FileID != "full" {
		t.Fatalf("chose %+v, want the largest", got)
	}
	if (&Message{}).BestPhoto() != nil {
		t.Error("a message with no photo returned one")
	}
}

func TestDisabledWithoutAToken(t *testing.T) {
	if New("").Enabled() {
		t.Fatal("reported enabled with no token")
	}
	if !New("123:abc").Enabled() {
		t.Fatal("reported disabled with a token")
	}
}

// The unregistered reply must not imply a way in for someone who has no passphrase.
func TestHelpAndRefusalCopyDoNotOverpromise(t *testing.T) {
	h := help(profileFixture())
	for _, want := range []string{"voice note", "photo", "/dash", "do not guess"} {
		if !contains(h, want) {
			t.Errorf("help does not mention %q", want)
		}
	}
	u := unrecognised("something long that the bot cannot possibly understand at all")
	if !contains(u, "simple interface") {
		t.Error("the fallback pretends to be a conversation")
	}
	if len(u) > 400 {
		t.Error("the fallback is a wall of text")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
