package dashboard

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func miniHandler(t *testing.T, resolve WebAppResolver) http.Handler {
	t.Helper()
	d, _ := setup(t)
	d.WebApp = resolve
	return Handler(d)
}

// The shell is served to anyone. It must therefore contain nobody's data and name nobody.
func TestTheShellRevealsNothing(t *testing.T) {
	rr := httptest.NewRecorder()
	miniHandler(t, func(string, time.Time) (string, error) {
		t.Error("the shell verified something; it should ask for nothing")
		return "", nil
	}).ServeHTTP(rr, httptest.NewRequest("GET", "/app", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("the shell returned %d", rr.Code)
	}
	body := rr.Body.String()
	// Markers of a rendered record, not bare substrings — "background" contains "kg",
	// which is how a leak test starts failing for reasons that teach you nothing.
	for _, leak := range []string{" kg", "kcal", "Raka", "fat_loss", "dumbbell"} {
		if strings.Contains(body, leak) {
			t.Errorf("the shell leaks %q before anyone is verified", leak)
		}
	}
	// And nothing that looks like a profile id: they are unguessable precisely so that
	// knowing one is knowing too much.
	if _, seeded := setup(t); strings.Contains(body, seeded) {
		t.Error("the shell contains a profile id")
	}
	if !strings.Contains(body, "telegram-web-app.js") {
		t.Error("the shell cannot obtain a launch string")
	}
	if rr.Header().Get("Cache-Control") != "no-store, private" {
		t.Error("the shell is cacheable")
	}
}

// Every refusal must look the same. Expired, forged, and "verified but never registered
// here" are different facts, and telling them apart would confirm to whoever holds a
// captured launch string that its signature was good.
func TestEveryRefusalIsByteIdentical(t *testing.T) {
	var bodies []string
	for _, reason := range []string{
		"this launch did not come from Telegram",
		"this launch has expired",
		"no record for this account",
	} {
		reason := reason
		h := miniHandler(t, func(string, time.Time) (string, error) {
			return "", fmt.Errorf("%s", reason)
		})
		rr := httptest.NewRecorder()
		rr.Body.Reset()
		req := httptest.NewRequest("POST", "/app/enter",
			strings.NewReader("initData=user%3D%7B%7D%26hash%3Dx"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%q returned %d, want 401", reason, rr.Code)
		}
		bodies = append(bodies, rr.Body.String())
		if strings.Contains(rr.Body.String(), reason) {
			t.Errorf("the page repeats the internal reason %q back to the caller", reason)
		}
	}
	for i := 1; i < len(bodies); i++ {
		if bodies[i] != bodies[0] {
			t.Error("two different failures produced two different pages")
		}
	}
}

// An oversized or empty launch string must be refused before it reaches the verifier —
// this endpoint is unauthenticated, and an unbounded read from one is a way to be knocked
// over by anybody who can reach the tunnel.
func TestOversizedAndEmptyLaunchesAreRefusedWithoutVerifying(t *testing.T) {
	called := false
	h := miniHandler(t, func(string, time.Time) (string, error) {
		called = true
		return "someone", nil
	})
	for _, body := range []string{"initData=", "initData=" + strings.Repeat("x", 9000)} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("POST", "/app/enter", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		h.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("a %d-byte launch returned %d", len(body), rr.Code)
		}
	}
	if called {
		t.Error("an empty or oversized launch reached the verifier")
	}
}

// Without a resolver the routes must not exist at all, rather than existing and refusing.
func TestTheRoutesAreAbsentWhenTelegramIsNotConfigured(t *testing.T) {
	d, _ := setup(t)
	h := Handler(d)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/app", nil))
	if rr.Code != http.StatusNotFound {
		t.Errorf("/app answered %d on a server with no bot", rr.Code)
	}
}
