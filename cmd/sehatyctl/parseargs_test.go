package main

import "testing"

func TestParseDeleteAcceptsTheFlagInAnyPosition(t *testing.T) {
	for _, args := range [][]string{
		{"raka", "--yes-really-delete"},
		{"--yes-really-delete", "raka"},
		{"-yes-really-delete", "raka"},
	} {
		id, confirm := parseDelete(args)
		if id != "raka" {
			t.Errorf("%v -> id %q, want raka", args, id)
		}
		if !confirm {
			t.Errorf("%v -> confirm false; the confirmation was silently dropped", args)
		}
	}
}

func TestParseDeleteWithoutConfirmationDoesNotConfirm(t *testing.T) {
	id, confirm := parseDelete([]string{"raka"})
	if id != "raka" || confirm {
		t.Fatalf("id=%q confirm=%v; a bare id must never imply consent", id, confirm)
	}
}

func TestParseDeleteIgnoresUnknownFlags(t *testing.T) {
	id, confirm := parseDelete([]string{"--verbose", "raka"})
	if id != "raka" {
		t.Errorf("id = %q; an unknown flag was taken as the profile id", id)
	}
	if confirm {
		t.Error("an unknown flag implied confirmation")
	}
}
