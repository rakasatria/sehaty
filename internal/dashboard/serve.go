package dashboard

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/rakasatria/sehaty/internal/dashlink"
)

// webdist holds the built Mini App. Embedded so the whole thing — server,
// database driver, food table and front end — remains one binary to copy and
// one service to run, which is the point of self-hosting it.
//
//go:embed all:webdist
var webdist embed.FS

// WebAppResolver turns a Telegram Mini App launch string into a profile id, or
// refuses.
//
// Injected rather than imported so this package stays free of the Telegram
// client: the dashboard renders records, it does not talk to Telegram.
type WebAppResolver func(initData string, now time.Time) (profileID string, err error)

// authenticate answers one question: whose record is this request for?
//
// Two credentials are accepted, and they correspond to the two doors. Inside
// Telegram it is the signed launch string, which proves both who is asking and
// that Telegram vouched for them. In a browser it is the token from a one-hour
// dashboard link. Both are checked; neither is inferred.
//
// One refusal for every failure. Whether a credential was forged, expired, or
// perfectly valid for an account that was never registered here are different
// facts, and telling them apart would confirm to whoever holds a captured
// credential that its signature was good.
func (d Deps) authenticate(r *http.Request) (profileID string, ok bool) {
	if init := r.Header.Get("X-Telegram-Init-Data"); init != "" && d.WebApp != nil {
		if len(init) > 8192 {
			return "", false
		}
		if id, err := d.WebApp(init, time.Now()); err == nil {
			return id, true
		}
		return "", false
	}
	if tok := r.Header.Get("X-Dashboard-Token"); tok != "" {
		if id, err := dashlink.Verify(d.Key, tok, time.Now()); err == nil {
			return id, true
		}
	}
	return "", false
}

// app serves the Mini App bundle.
//
// The bundle itself is NOT protected, and that is deliberate rather than an
// oversight: it contains no record, names nobody, and cannot obtain one without
// a credential the browser must supply to /api/summary. Gating the HTML would
// buy nothing and would break the Telegram flow, which loads the page before it
// can prove anything.
func (d Deps) app(mux *http.ServeMux) {
	sub, err := fs.Sub(webdist, "webdist")
	if err != nil {
		return
	}
	files := http.FileServer(http.FS(sub))

	mux.HandleFunc("GET /app", func(w http.ResponseWriter, _ *http.Request) { d.serveApp(w) })
	// Every in-app route resolves to the same document; the client owns routing.
	mux.HandleFunc("GET /app/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/app/assets/") {
			// Hashed filenames, so these may be cached hard. They contain no
			// record — only the program that asks for one.
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			http.StripPrefix("/app/", files).ServeHTTP(w, r)
			return
		}
		d.serveApp(w)
	})

	mux.HandleFunc("GET /api/summary", d.apiSummary)
}

// serveApp writes the application shell. It holds no record and names nobody;
// what it can obtain depends entirely on the credential the browser then
// presents to /api/summary.
func (d Deps) serveApp(w http.ResponseWriter) {
	sub, err := fs.Sub(webdist, "webdist")
	if err != nil {
		errorPage(w, http.StatusInternalServerError, "servererror")
		return
	}
	index, err := fs.ReadFile(sub, "index.html")
	if err != nil {
		errorPage(w, http.StatusInternalServerError, "servererror")
		return
	}
	secure(w)
	_, _ = w.Write(index)
}
