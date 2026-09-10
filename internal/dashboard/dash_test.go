package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rakasatria/sehaty/internal/dashlink"
	"github.com/rakasatria/sehaty/internal/storage"
)

const testKey = "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"

var signKey = []byte("0123456789abcdef0123456789abcdef")

func setup(t *testing.T) (Deps, string) {
	t.Helper()
	db, err := storage.Open(t.TempDir()+"/t.db", testKey)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	id, _ := storage.NewProfileID()
	if err := db.SaveProfile(storage.Profile{ID: id, DisplayName: "Raka",
		Goal: "fat_loss", Equipment: []string{"body weight", "dumbbell"}}); err != nil {
		t.Fatal(err)
	}
	if err := db.LogWeight(id, storage.WeightEntry{Date: "2026-09-10", WeightKg: 87.4}); err != nil {
		t.Fatal(err)
	}
	if err := db.LogSet(id, storage.SetEntry{Date: "2026-09-10", Exercise: "push-up",
		Sets: 3, Reps: 12}); err != nil {
		t.Fatal(err)
	}
	return Deps{DB: db, Key: signKey}, id
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

// A valid link opens the application — and the application ONLY. The record used
// to be baked into this response; it now arrives separately through
// /api/summary, which re-verifies the token on its own account. So the thing to
// assert here changed: the page must open, and it must carry nothing.
func TestValidLinkOpensTheAppAndCarriesNoRecord(t *testing.T) {
	d, id := setup(t)
	tok, _ := dashlink.Mint(signKey, id, time.Now())
	rec := get(Handler(d), "/d/"+tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("a valid link got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "/app/assets/") {
		t.Error("a valid link did not serve the application")
	}
	for _, leak := range []string{"Raka", "87.4", "push-up", "fat_loss"} {
		if strings.Contains(body, leak) {
			t.Errorf("the shell carries %q before anything was verified", leak)
		}
	}
}

// And the record it can then obtain is its own, and no one else's.
func TestALinkCanOnlyFetchItsOwnRecord(t *testing.T) {
	d, mine := setup(t)

	other, _ := storage.NewProfileID()
	if err := d.DB.SaveProfile(storage.Profile{ID: other, DisplayName: "Someone Else",
		Goal: "strength"}); err != nil {
		t.Fatal(err)
	}

	tok, _ := dashlink.Mint(signKey, mine, time.Now())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/summary?days=30", nil)
	req.Header.Set("X-Dashboard-Token", tok)
	Handler(d).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("a valid token was refused: %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Raka") {
		t.Error("the token did not return its own record")
	}
	if strings.Contains(body, "Someone Else") || strings.Contains(body, other) {
		t.Error("a token reached another person's record")
	}
}

func TestExpiredLinkIsRefused(t *testing.T) {
	d, id := setup(t)
	tok, _ := dashlink.Mint(signKey, id, time.Now().Add(-2*time.Hour))
	rec := get(Handler(d), "/d/"+tok)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expired link got %d, want 401", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "Raka") {
		t.Error("the refusal page leaked profile content")
	}
}

// A forged link and an expired one must be indistinguishable, or a forger learns that the
// profile id they guessed was real.
func TestForgedAndExpiredLookIdentical(t *testing.T) {
	d, id := setup(t)
	expired, _ := dashlink.Mint(signKey, id, time.Now().Add(-2*time.Hour))
	forged, _ := dashlink.Mint([]byte("ffffffffffffffffffffffffffffffff"), id, time.Now())
	a := get(Handler(d), "/d/"+expired)
	b := get(Handler(d), "/d/"+forged)
	if a.Code != b.Code || a.Body.String() != b.Body.String() {
		t.Error("an expired link is distinguishable from a forged one")
	}
}

// There must be no way in without a link.
func TestThereIsNoIndexOrProfileListing(t *testing.T) {
	d, _ := setup(t)
	h := Handler(d)
	for _, p := range []string{"/", "/d/", "/profiles", "/index.html"} {
		rec := get(h, p)
		if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), "Raka") {
			t.Errorf("%s exposed profile content without a link", p)
		}
	}
}

// A dashboard link must not be cached by a proxy or synced into a browser history.
func TestResponseIsNotCacheable(t *testing.T) {
	d, id := setup(t)
	tok, _ := dashlink.Mint(signKey, id, time.Now())
	rec := get(Handler(d), "/d/"+tok)
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Errorf("Cache-Control = %q", cc)
	}
	if rec.Header().Get("Referrer-Policy") != "no-referrer" {
		t.Error("no Referrer-Policy: the token would leak in the Referer header")
	}
}

// A link for one person must never fetch another's record. The page is now the
// same shell for everybody, so the question moved to the API — which is where it
// was always really being decided.
func TestLinkOnlyOpensItsOwnProfile(t *testing.T) {
	d, _ := setup(t)
	other, _ := storage.NewProfileID()
	if err := d.DB.SaveProfile(storage.Profile{ID: other, DisplayName: "Dina",
		Goal: "general", Equipment: []string{"body weight"}}); err != nil {
		t.Fatal(err)
	}
	tok, _ := dashlink.Mint(signKey, other, time.Now())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/summary?days=30", nil)
	req.Header.Set("X-Dashboard-Token", tok)
	Handler(d).ServeHTTP(rec, req)

	body := rec.Body.String()
	if strings.Contains(body, "Raka") {
		t.Error("Dina's link returned Raka's record")
	}
	if !strings.Contains(body, "Dina") {
		t.Error("Dina's link did not return Dina's record")
	}
}

func TestHealthzNeedsNoLink(t *testing.T) {
	d, _ := setup(t)
	if rec := get(Handler(d), "/healthz"); rec.Code != http.StatusOK {
		t.Fatalf("healthz got %d", rec.Code)
	}
}
