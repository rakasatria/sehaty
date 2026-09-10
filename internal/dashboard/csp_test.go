package dashboard

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The policy that protects the page must not also break it.
//
// This exists because it happened: the CSP named telegram.org and forgot the
// app's own origin, so the browser fetched the HTML, refused the bundle it
// pointed at, and rendered nothing. curl called that a 200. A blank screen on a
// phone is the most expensive kind of bug to find by hand, and the cheapest to
// catch here.
func TestTheCspAllowsTheAppItServes(t *testing.T) {
	d, _ := setup(t)
	d.WebApp = nil
	rr := httptest.NewRecorder()
	Handler(d).ServeHTTP(rr, httptest.NewRequest("GET", "/app", nil))

	if rr.Code != http.StatusOK {
		t.Fatalf("/app answered %d", rr.Code)
	}
	policy := rr.Header().Get("Content-Security-Policy")
	if policy == "" {
		t.Fatal("no policy at all")
	}

	directive := func(name string) string {
		for _, part := range strings.Split(policy, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, name+" ") {
				return part
			}
		}
		return ""
	}
	for _, need := range []struct{ name, token, why string }{
		{"script-src", "'self'", "the bundle is same-origin and would be refused"},
		{"script-src", "https://telegram.org", "initData is only reachable through Telegram's script"},
		{"style-src", "'self'", "the stylesheet is a same-origin <link>"},
		{"connect-src", "'self'", "the client fetches /api/summary from this origin"},
	} {
		if !strings.Contains(directive(need.name), need.token) {
			t.Errorf("%s lacks %s — %s\n  policy: %s", need.name, need.token, need.why, policy)
		}
	}

	// And it must still be shut to everything else.
	if !strings.HasPrefix(policy, "default-src 'none'") {
		t.Error("the policy no longer starts closed")
	}
	for _, banned := range []string{"'unsafe-eval'", "*", "http:"} {
		if strings.Contains(policy, banned) {
			t.Errorf("the policy has been opened up with %q", banned)
		}
	}
}

// The page the browser is given must actually point at assets the policy admits.
func TestTheServedPageReferencesOnlyItsOwnAssets(t *testing.T) {
	d, _ := setup(t)
	rr := httptest.NewRecorder()
	Handler(d).ServeHTTP(rr, httptest.NewRequest("GET", "/app", nil))
	body := rr.Body.String()

	if !strings.Contains(body, "/app/assets/") {
		t.Error("the page does not reference the bundle at the path it is served from")
	}
	for _, ref := range []string{"http://", "//cdn", "unpkg", "jsdelivr"} {
		if strings.Contains(body, ref) {
			t.Errorf("the page reaches for %q", ref)
		}
	}
}
