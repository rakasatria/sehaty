package tools

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("reached"))
	})
}

func TestRequireTokenRefusesWithoutTheCredential(t *testing.T) {
	h := RequireToken(okHandler(), "s3cret-token")
	for _, tc := range []struct{ name, header, value string }{
		{"no header", "", ""},
		{"wrong token", "Authorization", "Bearer wrong"},
		{"empty bearer", "Authorization", "Bearer "},
		{"token as plain header value", "Authorization", "s3cret-token"},
		{"prefix of the real token", "Authorization", "Bearer s3cret"},
	} {
		req := httptest.NewRequest("POST", "/mcp", nil)
		if tc.header != "" {
			req.Header.Set(tc.header, tc.value)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: got %d, want 401", tc.name, rec.Code)
		}
	}
}

func TestRequireTokenAcceptsTheCredential(t *testing.T) {
	h := RequireToken(okHandler(), "s3cret-token")
	for _, tc := range [][2]string{
		{"Authorization", "Bearer s3cret-token"},
		{"Authorization", "bearer s3cret-token"},
		{"X-Api-Key", "s3cret-token"},
	} {
		req := httptest.NewRequest("POST", "/mcp", nil)
		req.Header.Set(tc[0], tc[1])
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("%s: %s -> %d, want 200", tc[0], tc[1], rec.Code)
		}
	}
}

// A community user on a laptop should not be locked out by a credential they never set —
// but the startup log says so loudly. See main.go.
func TestRequireTokenIsOpenWhenUnconfigured(t *testing.T) {
	h := RequireToken(okHandler(), "")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("POST", "/mcp", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d with no token configured, want 200", rec.Code)
	}
}

// A 401 must not hint at how close the attempt was.
func TestUnauthorizedRevealsNothing(t *testing.T) {
	h := RequireToken(okHandler(), "s3cret-token")
	req := httptest.NewRequest("POST", "/mcp", nil)
	req.Header.Set("Authorization", "Bearer s3cret-toke")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if body := rec.Body.String(); len(body) > 20 {
		t.Errorf("401 body is chatty: %q", body)
	}
}
