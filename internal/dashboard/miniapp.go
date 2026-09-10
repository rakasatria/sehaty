package dashboard

import (
	"net/http"
	"time"
)

// WebAppResolver turns a Telegram Mini App launch string into a profile id, or refuses.
//
// Injected rather than imported so this package stays free of the Telegram client: the
// dashboard renders records, it does not talk to Telegram. It also means a test can prove
// the routing refuses what the verifier refuses, without a bot token.
type WebAppResolver func(initData string, now time.Time) (profileID string, err error)

// miniApp registers the Mini App routes.
//
// Two of them, and the split is deliberate. GET /app serves a shell that knows nothing and
// contains nobody's data. The launch string only exists inside Telegram's webview, so the
// shell asks Telegram's own script for it and POSTs it back; the server validates and only
// then renders a record. Nothing identifying is ever placed in a URL, which is what makes
// this better than the link it replaces — there is no address to forward.
func (d Deps) miniApp(mux *http.ServeMux) {
	if d.WebApp == nil {
		return
	}

	mux.HandleFunc("GET /app", func(w http.ResponseWriter, _ *http.Request) {
		secure(w)
		_, _ = w.Write([]byte(shell))
	})

	mux.HandleFunc("POST /app/enter", func(w http.ResponseWriter, r *http.Request) {
		// Bounded: this field is a query string of a few hundred bytes, and an unbounded
		// read from an unauthenticated endpoint is a denial of service waiting to happen.
		if err := r.ParseForm(); err != nil {
			errorPage(w, http.StatusUnauthorized, "gone")
			return
		}
		initData := r.PostFormValue("initData")
		if len(initData) == 0 || len(initData) > 8192 {
			errorPage(w, http.StatusUnauthorized, "gone")
			return
		}
		profileID, err := d.WebApp(initData, time.Now())
		if err != nil {
			// One message for every failure — unverified, expired, or verified but never
			// registered here. Distinguishing them would confirm to whoever holds a
			// captured launch string that its signature was good.
			errorPage(w, http.StatusUnauthorized, "gone")
			return
		}
		v, err := build(d, profileID)
		if err != nil {
			errorPage(w, http.StatusInternalServerError, "servererror")
			return
		}
		// This document is a fresh navigation, so it does not inherit the shell's
		// Telegram context. Without loading the script again it would render with none
		// of Telegram's theme variables and silently fall back to the browser palette.
		v.InTelegram = true
		secure(w)
		_ = tpl.ExecuteTemplate(w, "page", v)
	})
}

// shell is what Telegram loads. It holds no data and names nobody.
//
// telegram.org's script is the one external resource on any Sehaty page, and it has to be:
// initData exists only inside the webview and only that script exposes it. The CSP admits
// that origin and nothing else, so a compromised dependency has nowhere to send anything.
const shell = `<!doctype html>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>Sehaty</title>
<script src="https://telegram.org/js/telegram-web-app.js"></script>
<style>
  html { color-scheme: light dark; }
  body {
    margin: 0; min-height: 100dvh;
    display: grid; place-items: center;
    font: 15px/1.5 ui-sans-serif, -apple-system, "Segoe UI", Roboto, sans-serif;
    background: var(--tg-theme-bg-color, Canvas);
    color: var(--tg-theme-hint-color, GrayText);
  }
  p { margin: 0; padding: 0 2rem; text-align: center; }
</style>
<form id="f" method="post" action="/app/enter" hidden>
  <input type="hidden" name="initData" id="d">
</form>
<p id="m">Membuka catatanmu…</p>
<script>
  (function () {
    var tg = window.Telegram && window.Telegram.WebApp;
    if (!tg || !tg.initData) {
      document.getElementById("m").textContent =
        "Halaman ini cuma bisa dibuka dari dalam Telegram.";
      return;
    }
    tg.ready();
    tg.expand();
    document.getElementById("d").value = tg.initData;
    document.getElementById("f").submit();
  })();
</script>
`
