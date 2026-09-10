package tools

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rakasatria/sehaty/internal/storage"
)

func authDB(t *testing.T) (*storage.DB, string, string) {
	t.Helper()
	d := testDeps(t)
	a, err := Register(d, "mcp", "acct-a", "Raka", pass, pass)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Register(d, "mcp", "acct-b", "Dina", pass, pass)
	if err != nil {
		t.Fatal(err)
	}
	return d.DB, a.ID, b.ID
}

func callBody(profile string) []byte {
	b, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "progress",
			"arguments": map[string]any{"profile": profile, "days": 7}},
	})
	return b
}

func do(h http.Handler, token string, body []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/mcp", bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// The whole point: a token bound to one person cannot touch another's record.
func TestProfileTokenCannotActOnAnotherProfile(t *testing.T) {
	db, a, b := authDB(t)
	secret, _, err := db.IssueToken(a, "raka's agent")
	if err != nil {
		t.Fatal(err)
	}
	h := Authenticate(okHandler(), db, "")

	if rec := do(h, secret, callBody(a)); rec.Code != http.StatusOK {
		t.Errorf("own profile got %d, want 200", rec.Code)
	}
	if rec := do(h, secret, callBody(b)); rec.Code != http.StatusForbidden {
		t.Errorf("another profile got %d, want 403", rec.Code)
	}
}

// A 403 must not reveal which profile the token owns, or any token holder could enumerate
// the others by trial.
func TestForbiddenDoesNotNameTheTokensProfile(t *testing.T) {
	db, a, b := authDB(t)
	secret, _, _ := db.IssueToken(a, "x")
	rec := do(Authenticate(okHandler(), db, ""), secret, callBody(b))
	if bytes.Contains(rec.Body.Bytes(), []byte(a)) {
		t.Errorf("the 403 body leaks the token's own profile: %q", rec.Body.String())
	}
}

func TestUnknownAndRevokedTokensAreBothRejected(t *testing.T) {
	db, a, _ := authDB(t)
	secret, tok, _ := db.IssueToken(a, "x")
	h := Authenticate(okHandler(), db, "")
	if rec := do(h, "sht_nonsense", callBody(a)); rec.Code != http.StatusUnauthorized {
		t.Errorf("unknown token got %d", rec.Code)
	}
	if err := db.RevokeToken(tok.ID); err != nil {
		t.Fatal(err)
	}
	if rec := do(h, secret, callBody(a)); rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked token got %d, want 401", rec.Code)
	}
}

// Registration has to work before any profile exists, so an admin credential is required.
func TestAdminTokenMayActOnAnyProfile(t *testing.T) {
	db, a, b := authDB(t)
	secret, _, err := db.IssueToken("", "bootstrap")
	if err != nil {
		t.Fatal(err)
	}
	h := Authenticate(okHandler(), db, "")
	for _, p := range []string{a, b} {
		if rec := do(h, secret, callBody(p)); rec.Code != http.StatusOK {
			t.Errorf("admin token on %s got %d", p, rec.Code)
		}
	}
}

func TestEnvAdminTokenStillWorks(t *testing.T) {
	db, a, _ := authDB(t)
	h := Authenticate(okHandler(), db, "env-admin-token")
	if rec := do(h, "env-admin-token", callBody(a)); rec.Code != http.StatusOK {
		t.Errorf("env admin token got %d", rec.Code)
	}
	if rec := do(h, "env-admin-toke", callBody(a)); rec.Code != http.StatusUnauthorized {
		t.Errorf("a near-miss on the env token got %d, want 401", rec.Code)
	}
}

// A body that names no profile (initialize, tools/list) must pass a valid profile token.
func TestNonToolCallsPassThroughForAValidToken(t *testing.T) {
	db, a, _ := authDB(t)
	secret, _, _ := db.IssueToken(a, "x")
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/list"})
	if rec := do(Authenticate(okHandler(), db, ""), secret, body); rec.Code != http.StatusOK {
		t.Errorf("tools/list got %d, want 200", rec.Code)
	}
}

// The handler downstream must still receive the body the client sent.
func TestBodyIsRestoredForTheNextHandler(t *testing.T) {
	db, a, _ := authDB(t)
	secret, _, _ := db.IssueToken(a, "x")
	var seen []byte
	echo := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = readAll(r)
		w.WriteHeader(http.StatusOK)
	})
	body := callBody(a)
	do(Authenticate(echo, db, ""), secret, body)
	if !bytes.Equal(seen, body) {
		t.Errorf("downstream saw %d bytes, sent %d", len(seen), len(body))
	}
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("reached"))
	})
}
