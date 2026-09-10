package tools

import (
	"bytes"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/rakasatria/sehaty/internal/storage"
)

// Authenticate identifies the caller and confines it to its own profile.
//
// Two kinds of credential:
//
//	profile token   bound to one person. Every tool call it makes must name that profile.
//	admin token     bound to none. Needed because registration creates the first profile,
//	                so something has to be able to call it before any profile token exists.
//
// The confinement happens HERE, in one place, rather than in each of the twenty-one tool
// handlers. A check repeated twenty-one times is a check that will eventually be forgotten
// once — and the thing being protected is one person's health record from another's.
//
// With no credentials configured at all the server runs OPEN and says so loudly at
// startup: a community user trying Sehaty on a laptop should not be locked out by a
// credential they never set.
func Authenticate(next http.Handler, db *storage.DB, adminToken string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		secret := string(bearer(r))

		// The environment token is an admin credential — the bootstrap path, and what
		// sehatyctl and the dashboard will use.
		if adminToken != "" && len(secret) == len(adminToken) &&
			subtle.ConstantTimeCompare([]byte(secret), []byte(adminToken)) == 1 {
			next.ServeHTTP(w, r)
			return
		}

		tok, err := db.ResolveToken(secret)
		if err != nil {
			unauthorized(w)
			return
		}
		if tok.Admin() {
			next.ServeHTTP(w, r)
			return
		}

		// A profile token may only act on its own profile.
		body, err := io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if named, ok := profileArgument(body); ok && named != tok.ProfileID {
			// Deliberately does not say which profile the token owns: that would let a
			// holder of any token enumerate the others.
			http.Error(w, "this token may not act on that profile", http.StatusForbidden)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		next.ServeHTTP(w, r)
	})
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="sehaty"`)
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

// profileArgument pulls params.arguments.profile out of a tools/call request.
//
// Only the profile field is decoded; the rest of arguments stays as raw bytes, so a 25 MB
// base64 photo is not turned into a second copy in memory just to read one string.
func profileArgument(body []byte) (string, bool) {
	var req struct {
		Method string `json:"method"`
		Params struct {
			Arguments map[string]json.RawMessage `json:"arguments"`
		} `json:"params"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return "", false
	}
	if req.Method != "tools/call" {
		return "", false
	}
	raw, ok := req.Params.Arguments["profile"]
	if !ok {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return strings.TrimSpace(s), s != ""
}

// bearer pulls the credential from Authorization, falling back to X-Api-Key for clients
// that cannot set an Authorization header.
func bearer(r *http.Request) []byte {
	if h := r.Header.Get("Authorization"); h != "" {
		for _, p := range []string{"Bearer ", "bearer "} {
			if v, ok := strings.CutPrefix(h, p); ok {
				return []byte(strings.TrimSpace(v))
			}
		}
	}
	return []byte(strings.TrimSpace(r.Header.Get("X-Api-Key")))
}
