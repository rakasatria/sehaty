package dashboard

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rakasatria/sehaty/internal/dashlink"
)

func TestErrorPagesAreRealPagesNotBareText(t *testing.T) {
	d, id := setup(t)
	h := Handler(d)
	expired, _ := dashlink.Mint(signKey, id, time.Now().Add(-2*time.Hour))

	for _, tc := range []struct {
		name, path string
		code       int
		wants      []string
	}{
		{"expired link", "/d/" + expired, http.StatusUnauthorized,
			[]string{"<!doctype html>", "expired", "one", "hour", "<svg"}},
		{"unknown path", "/nothing", http.StatusNotFound,
			[]string{"<!doctype html>", "Nothing here", "<svg"}},
	} {
		rec := get(h, tc.path)
		if rec.Code != tc.code {
			t.Errorf("%s: got %d, want %d", tc.name, rec.Code, tc.code)
		}
		body := rec.Body.String()
		for _, w := range tc.wants {
			if !strings.Contains(strings.ToLower(body), strings.ToLower(w)) {
				t.Errorf("%s: page does not contain %q", tc.name, w)
			}
		}
		if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
			t.Errorf("%s: content-type = %q", tc.name, ct)
		}
		if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
			t.Errorf("%s: cache-control = %q", tc.name, cc)
		}
		// Both themes must be defined, or one of them renders unreadable.
		if !strings.Contains(body, "prefers-color-scheme") {
			t.Errorf("%s: no dark theme", tc.name)
		}
	}
}

// The 404 says there is no directory; the 401 must still not distinguish expired from
// forged. Those are different guarantees and both have to hold.
func TestNotFoundNeverNamesAnyone(t *testing.T) {
	d, _ := setup(t)
	body := get(Handler(d), "/nothing").Body.String()
	if strings.Contains(body, "Raka") {
		t.Error("the 404 page names a profile")
	}
}
