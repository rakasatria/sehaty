package storage

import (
	"errors"
	"testing"
)

func TestLinkAndResolveIdentity(t *testing.T) {
	db := testDB(t)
	seed(t, db, "raka")
	must(t, db.LinkIdentity("telegram", "8412", "raka"))
	got, err := db.ResolveIdentity("telegram", "8412")
	must(t, err)
	if got != "raka" {
		t.Errorf("got %q, want raka", got)
	}
}

func TestResolveUnknownIdentity(t *testing.T) {
	db := testDB(t)
	if _, err := db.ResolveIdentity("telegram", "9999"); !errors.Is(err, ErrNoIdentity) {
		t.Errorf("err = %v, want ErrNoIdentity", err)
	}
}

// Same external id on two channels must not collide — a Telegram user 8412 and a
// Cloudflare Access identity 8412 are unrelated namespaces.
func TestChannelsAreSeparateNamespaces(t *testing.T) {
	db := testDB(t)
	seed(t, db, "raka", "other")
	must(t, db.LinkIdentity("telegram", "8412", "raka"))
	must(t, db.LinkIdentity("access", "8412", "other"))
	a, _ := db.ResolveIdentity("telegram", "8412")
	b, _ := db.ResolveIdentity("access", "8412")
	if a != "raka" || b != "other" {
		t.Errorf("telegram=%q access=%q", a, b)
	}
}

func TestLinkRejectsUnknownProfile(t *testing.T) {
	db := testDB(t)
	if err := db.LinkIdentity("telegram", "1", "ghost"); err == nil {
		t.Error("linking to a nonexistent profile should fail")
	}
}

// Re-linking the same external id moves it rather than duplicating — a person who
// re-registers must not end up with two profiles quietly diverging.
func TestRelinkMovesIdentity(t *testing.T) {
	db := testDB(t)
	seed(t, db, "raka", "other")
	must(t, db.LinkIdentity("telegram", "8412", "raka"))
	must(t, db.LinkIdentity("telegram", "8412", "other"))
	got, _ := db.ResolveIdentity("telegram", "8412")
	if got != "other" {
		t.Errorf("got %q, want other", got)
	}
}
