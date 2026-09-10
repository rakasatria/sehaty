package tools

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// RequireToken wraps the MCP handler so only a caller holding the shared token gets
// through.
//
// WHY THIS EXISTS. Every tool takes a `profile` argument, and the MCP security guidance is
// explicit that a server must not treat a client-supplied handle as authentication. Until
// now the only thing standing between one agent and another person's health record was an
// unguessable profile id and a LAN-only port. Neither is a credential.
//
// This is a SHARED SECRET, not per-user identity: it answers "may this client call Sehaty
// at all", not "who is this". Profile isolation inside the tools remains the boundary
// between two people; this is the boundary between Sehaty and everything else on the
// network.
//
// If no token is configured the server runs OPEN and says so loudly at startup. That is
// deliberate — a community user trying Sehaty on a laptop should not be locked out by a
// credential they never set, but nobody should be able to run it exposed without knowing.
func RequireToken(next http.Handler, token string) http.Handler {
	if token == "" {
		return next
	}
	want := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := bearer(r)
		// Constant time: a byte-by-byte comparison leaks the token through timing to
		// anyone who can make repeated requests, which is anyone who can reach the port.
		if len(got) != len(want) || subtle.ConstantTimeCompare(got, want) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="sehaty"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// bearer pulls the credential from Authorization, falling back to X-Api-Key for clients
// that cannot set an Authorization header.
func bearer(r *http.Request) []byte {
	if h := r.Header.Get("Authorization"); h != "" {
		if v, ok := strings.CutPrefix(h, "Bearer "); ok {
			return []byte(strings.TrimSpace(v))
		}
		if v, ok := strings.CutPrefix(h, "bearer "); ok {
			return []byte(strings.TrimSpace(v))
		}
	}
	return []byte(strings.TrimSpace(r.Header.Get("X-Api-Key")))
}
