package tools

import (
	"strings"
	"testing"

	"github.com/rakasatria/sehaty/internal/storage"
)

const pass = "test-passphrase"

func TestRegisterGeneratesAnUnguessableID(t *testing.T) {
	d := testDeps(t)
	p, err := Register(d, "telegram", "8412", "Raka", pass, pass)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ID) != storage.ProfileIDLength {
		t.Fatalf("id %q is %d chars, want %d", p.ID, len(p.ID), storage.ProfileIDLength)
	}
	if strings.EqualFold(p.ID, "raka") {
		t.Fatal("the id is the person's name; that is guessable, which is the whole problem")
	}
	if p.DisplayName != "Raka" {
		t.Errorf("display name = %q", p.DisplayName)
	}
}

// A retry, or the same person registering twice, must not mint a second profile and
// orphan the first one's history.
func TestRegisterIsIdempotentPerAccount(t *testing.T) {
	d := testDeps(t)
	first, err := Register(d, "telegram", "8412", "Raka", pass, pass)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Register(d, "telegram", "8412", "Raka Again", pass, pass)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID {
		t.Fatalf("same account produced two profiles: %s and %s", first.ID, second.ID)
	}
	all, err := d.DB.ListProfiles()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 {
		t.Fatalf("%d profiles exist after registering one account twice", len(all))
	}
}

// Different people on the same channel are different profiles.
func TestRegisterSeparatesDifferentAccounts(t *testing.T) {
	d := testDeps(t)
	a, _ := Register(d, "telegram", "8412", "Raka", pass, pass)
	b, err := Register(d, "telegram", "9999", "Dina", pass, pass)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID == b.ID {
		t.Fatal("two people share one profile")
	}
}

func TestRegisterRefusesAWrongPassphrase(t *testing.T) {
	d := testDeps(t)
	if _, err := Register(d, "telegram", "8412", "Raka", "wrong", pass); err == nil {
		t.Fatal("registered with the wrong passphrase")
	}
	all, _ := d.DB.ListProfiles()
	if len(all) != 0 {
		t.Fatal("a failed registration still created a profile")
	}
}

func TestRegisterRefusesWhenRegistrationIsClosed(t *testing.T) {
	d := testDeps(t)
	if _, err := Register(d, "telegram", "8412", "Raka", "anything", ""); err == nil {
		t.Fatal("registered while registration was closed")
	}
}

func TestRegisterRequiresChannelAndAccount(t *testing.T) {
	d := testDeps(t)
	for _, tc := range [][2]string{{"", "8412"}, {"telegram", ""}, {"", ""}} {
		if _, err := Register(d, tc[0], tc[1], "Raka", pass, pass); err == nil {
			t.Errorf("accepted channel=%q external_id=%q", tc[0], tc[1])
		}
	}
}
