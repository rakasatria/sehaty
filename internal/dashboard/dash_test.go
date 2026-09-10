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

func TestValidLinkShowsTheProfile(t *testing.T) {
	d, id := setup(t)
	tok, _ := dashlink.Mint(signKey, id, time.Now())
	rec := get(Handler(d), "/d/"+tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"Raka", "87.4", "push-up", "fat loss"} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not mention %q", want)
		}
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

// A link for one person must never render another's record.
func TestLinkOnlyOpensItsOwnProfile(t *testing.T) {
	d, _ := setup(t)
	other, _ := storage.NewProfileID()
	if err := d.DB.SaveProfile(storage.Profile{ID: other, DisplayName: "Dina",
		Goal: "general", Equipment: []string{"body weight"}}); err != nil {
		t.Fatal(err)
	}
	tok, _ := dashlink.Mint(signKey, other, time.Now())
	rec := get(Handler(d), "/d/"+tok)
	if strings.Contains(rec.Body.String(), "Raka") {
		t.Error("Dina's link rendered Raka's record")
	}
	if !strings.Contains(rec.Body.String(), "Dina") {
		t.Error("Dina's link did not render Dina")
	}
}

func TestHealthzNeedsNoLink(t *testing.T) {
	d, _ := setup(t)
	if rec := get(Handler(d), "/healthz"); rec.Code != http.StatusOK {
		t.Fatalf("healthz got %d", rec.Code)
	}
}
