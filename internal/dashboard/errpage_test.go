package dashboard

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rakasatria/sehaty/internal/dashlink"
)

// These assert PROPERTIES, not wording. An earlier version pinned the exact headline and
// broke the moment the page was redesigned — which taught nobody anything about whether
// the page still worked.
func TestErrorPagesHoldTheirGuarantees(t *testing.T) {
	d, id := setup(t)
	h := Handler(d)
	expired, _ := dashlink.Mint(signKey, id, time.Now().Add(-2*time.Hour))

	for _, tc := range []struct {
		name, path string
		code       int
	}{
		{"expired link", "/d/" + expired, http.StatusUnauthorized},
		{"unknown path", "/nothing", http.StatusNotFound},
	} {
		rec := get(h, tc.path)
		body := rec.Body.String()
		low := strings.ToLower(body)

		if rec.Code != tc.code {
			t.Errorf("%s: got %d, want %d", tc.name, rec.Code, tc.code)
		}
		if !strings.Contains(low, "<!doctype html>") {
			t.Errorf("%s: not a complete document", tc.name)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Errorf("%s: content-type = %q", tc.name, ct)
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
			t.Errorf("%s: cache-control = %q", tc.name, cc)
		}
		// Readable in both themes. A page that renders white-on-white at night is a page
		// nobody can read at the moment they need it.
		if !strings.Contains(body, "prefers-color-scheme") {
			t.Errorf("%s: no dark theme", tc.name)
		}
		// Self-contained: an error page on a privacy product must not phone anyone.
		for _, bad := range []string{"http://", "https://", "<script"} {
			if strings.Contains(low, strings.ToLower(bad)) {
				t.Errorf("%s: contains %q — must be fully self-contained", tc.name, bad)
			}
		}
		if strings.Contains(body, "Raka") || strings.Contains(body, id) {
			t.Errorf("%s: leaks profile information", tc.name)
		}
		// Motion, if any, must be defeatable.
		if strings.Contains(body, "animation") && !strings.Contains(body, "prefers-reduced-motion") {
			t.Errorf("%s: animates without honouring prefers-reduced-motion", tc.name)
		}
	}
}

// The 401 must say nothing that is false when the link was never valid in the first place.
// "Expired" implies the link once worked; a forged link never did.
func TestExpiredPageDoesNotAssertTheLinkWasEverValid(t *testing.T) {
	d, id := setup(t)
	expired, _ := dashlink.Mint(signKey, id, time.Now().Add(-2*time.Hour))
	body := strings.ToLower(get(Handler(d), "/d/"+expired).Body.String())
	for _, claim := range []string{"was valid", "your link worked", "previously granted"} {
		if strings.Contains(body, claim) {
			t.Errorf("the 401 asserts %q, which is false for a forged link", claim)
		}
	}
}

func TestServerErrorReassuresDataIsIntact(t *testing.T) {
	d, id := setup(t)
	tok, _ := dashlink.Mint(signKey, id, time.Now())
	if err := d.DB.DeleteProfile(id); err != nil {
		t.Fatal(err)
	}
	rec := get(Handler(d), "/d/"+tok)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", rec.Code)
	}
	low := strings.ToLower(rec.Body.String())
	if !strings.Contains(low, "untouched") && !strings.Contains(low, "nothing was lost") &&
		!strings.Contains(low, "not been lost") {
		t.Error("the 500 does not reassure that records are intact")
	}
	if strings.Contains(rec.Body.String(), id) {
		t.Error("the 500 leaks the profile id")
	}
}
