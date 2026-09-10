package storage

import (
	"errors"
	"strings"
	"testing"
)

func tokenDB(t *testing.T) (*DB, string) {
	t.Helper()
	db, err := Open(t.TempDir()+"/t.db", testKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	id, _ := NewProfileID()
	if err := db.SaveProfile(Profile{ID: id, DisplayName: "Raka",
		Equipment: []string{"body weight"}}); err != nil {
		t.Fatal(err)
	}
	return db, id
}

func TestIssuedTokenResolvesToItsProfile(t *testing.T) {
	db, pid := tokenDB(t)
	secret, tok, err := db.IssueToken(pid, "hermes gym agent")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(secret, "sht_") || len(secret) < 60 {
		t.Fatalf("secret looks wrong: %q", secret)
	}
	got, err := db.ResolveToken(secret)
	if err != nil {
		t.Fatal(err)
	}
	if got.ProfileID != pid {
		t.Errorf("resolved to %q, want %q", got.ProfileID, pid)
	}
	if got.ID != tok.ID {
		t.Errorf("id mismatch")
	}
	if got.Admin() {
		t.Error("a profile token reported itself as admin")
	}
}

// The plaintext must never be recoverable from the database.
func TestTokenPlaintextIsNotStored(t *testing.T) {
	db, pid := tokenDB(t)
	secret, _, err := db.IssueToken(pid, "probe")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT id, token_hash, profile_id, label FROM api_token`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var a, b, c, d string
		if err := rows.Scan(&a, &b, &c, &d); err != nil {
			t.Fatal(err)
		}
		for _, v := range []string{a, b, c, d} {
			if strings.Contains(v, secret) {
				t.Fatal("the token plaintext is stored in the database")
			}
		}
	}
}

// A revoked token must be indistinguishable from one that never existed — the reply must
// not tell an attacker that a credential once worked.
func TestRevokedTokenIsRejectedLikeAnUnknownOne(t *testing.T) {
	db, pid := tokenDB(t)
	secret, tok, _ := db.IssueToken(pid, "old")
	if err := db.RevokeToken(tok.ID); err != nil {
		t.Fatal(err)
	}
	_, errRevoked := db.ResolveToken(secret)
	_, errUnknown := db.ResolveToken("sht_deadbeef")
	if !errors.Is(errRevoked, ErrNoToken) || !errors.Is(errUnknown, ErrNoToken) {
		t.Fatalf("revoked=%v unknown=%v — both should be ErrNoToken", errRevoked, errUnknown)
	}
	if errRevoked.Error() != errUnknown.Error() {
		t.Errorf("a revoked token reports differently from an unknown one: %q vs %q",
			errRevoked, errUnknown)
	}
}

// "Reset my access" — every credential for one person, at once.
func TestRevokeProfileTokensResetsAllOfThem(t *testing.T) {
	db, pid := tokenDB(t)
	other, _ := NewProfileID()
	if err := db.SaveProfile(Profile{ID: other, DisplayName: "Dina",
		Equipment: []string{"body weight"}}); err != nil {
		t.Fatal(err)
	}
	a, _, _ := db.IssueToken(pid, "phone")
	b, _, _ := db.IssueToken(pid, "laptop")
	keep, _, _ := db.IssueToken(other, "hers")

	n, err := db.RevokeProfileTokens(pid)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("revoked %d, want 2", n)
	}
	for _, s := range []string{a, b} {
		if _, err := db.ResolveToken(s); !errors.Is(err, ErrNoToken) {
			t.Error("a token survived the reset")
		}
	}
	// Another person's credential must be untouched.
	if _, err := db.ResolveToken(keep); err != nil {
		t.Errorf("resetting one profile revoked another's token: %v", err)
	}
}

func TestAdminTokenHasNoProfile(t *testing.T) {
	db, _ := tokenDB(t)
	secret, _, err := db.IssueToken("", "bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	got, err := db.ResolveToken(secret)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Admin() {
		t.Error("a token with no profile is not reporting as admin")
	}
}

func TestIssueTokenRefusesAnUnknownProfile(t *testing.T) {
	db, _ := tokenDB(t)
	if _, _, err := db.IssueToken("never-existed", "x"); !errors.Is(err, ErrNoProfile) {
		t.Fatalf("err = %v, want ErrNoProfile", err)
	}
}

// Revoked rows are kept so the record of what once had access survives.
func TestListTokensIncludesRevokedOnes(t *testing.T) {
	db, pid := tokenDB(t)
	_, tok, _ := db.IssueToken(pid, "gone")
	if err := db.RevokeToken(tok.ID); err != nil {
		t.Fatal(err)
	}
	all, err := db.ListTokens()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1 || !all[0].Revoked() {
		t.Fatalf("listing lost the revoked token: %+v", all)
	}
}
