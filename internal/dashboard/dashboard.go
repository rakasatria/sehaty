// Package dashboard serves one person's health record behind a time-limited link.
//
// It listens on its OWN PORT, separate from MCP. The MCP surface is tool execution —
// anything that reaches it can write to a health record — and must never be published.
// This is read-only and safe to put behind a tunnel, so the two must not share a listener.
package dashboard

import (
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/rakasatria/sehaty/internal/dashlink"
	"github.com/rakasatria/sehaty/internal/storage"
)

type Deps struct {
	DB  *storage.DB
	Key []byte
}

type view struct {
	Name      string
	Goal      string
	Equipment string
	Expires   string
	// Weight stays a POINTER so "never weighed" is distinguishable from 0 kg — the same
	// distinction the tools make. WeightStr is the formatted value, because Go templates
	// do not dereference a pointer for printf and would render the address instead.
	Weight    *float64
	WeightStr string
	WeightAgo string
	Sets      int
	Sessions  int
	Cardio    int
	FoodDays  int
	Kcal      float64
	Protein   float64
	Recent    []line
	Limits    []string
}

type line struct{ Date, What, Detail string }

// Handler serves /d/{token} and nothing else. There is no index, no login and no way in
// without a link — an unauthenticated route that lists profiles would undo the point.
func Handler(d Deps) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	mux.HandleFunc("/d/", func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.URL.Path, "/d/")
		profileID, err := dashlink.Verify(d.Key, token, time.Now())
		if err != nil {
			// One message for every failure. Distinguishing "expired" from "forged"
			// would confirm to a forger that the profile id inside was real.
			errorPage(w, http.StatusUnauthorized, "gone")
			return
		}
		v, err := build(d, profileID)
		if err != nil {
			// A health app that fails to render must not look like a health app that
			// lost your data. The page says so; this is only the plumbing.
			errorPage(w, http.StatusInternalServerError, "servererror")
			return
		}
		// A dashboard link should never be cached by a proxy or a browser history sync.
		w.Header().Set("Cache-Control", "no-store, private")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = tpl.ExecuteTemplate(w, "page", v)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// A different situation from a dead link, and safe to describe plainly: there is
		// simply no page here. Saying so reveals nothing, because there is nothing to
		// reveal — no index, no directory, no names.
		errorPage(w, http.StatusNotFound, "notfound")
	})
	return mux
}

// errorPage renders a refusal. Every 401 is byte-identical whatever caused it — telling
// someone their forged link is merely "expired" would confirm the profile id inside it was
// real.
func errorPage(w http.ResponseWriter, code int, name string) {
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	_ = tpl.ExecuteTemplate(w, name, nil)
}

func build(d Deps, profileID string) (view, error) {
	p, err := d.DB.GetProfile(profileID)
	if err != nil {
		return view{}, err
	}
	v := view{
		Name: p.DisplayName, Goal: strings.ReplaceAll(p.Goal, "_", " "),
		Equipment: strings.Join(p.Equipment, " · "),
		Limits:    p.Limitations,
		Expires:   time.Now().Add(dashlink.TTL).Format("15:04"),
	}
	if v.Name == "" {
		v.Name = p.ID
	}

	const window = 30
	if ws, err := d.DB.Weights(profileID, window); err == nil && len(ws) > 0 {
		last := ws[len(ws)-1]
		kg := last.WeightKg
		v.Weight = &kg
		v.WeightStr = fmt.Sprintf("%.1f", kg)
		v.WeightAgo = last.Date
	}
	sets, _ := d.DB.Sets(profileID, window)
	days := map[string]bool{}
	for _, s := range sets {
		days[s.Date] = true
		v.Recent = append(v.Recent, line{s.Date, s.Exercise,
			fmt.Sprintf("%d×%d", s.Sets, s.Reps)})
	}
	v.Sets, v.Sessions = len(sets), len(days)

	cardio, _ := d.DB.Cardio(profileID, window)
	for _, c := range cardio {
		v.Cardio += int(c.Minutes)
		v.Recent = append(v.Recent, line{c.Date, "cardio",
			fmt.Sprintf("%.0f min", c.Minutes)})
	}
	foods, _ := d.DB.Foods(profileID, window)
	fdays := map[string]bool{}
	for _, f := range foods {
		fdays[f.Date] = true
		v.Kcal += f.Kcal
		v.Protein += f.ProteinG
		v.Recent = append(v.Recent, line{f.Date, f.Item,
			fmt.Sprintf("%.0f g · %.0f kcal", f.Grams, f.Kcal)})
	}
	v.FoodDays = len(fdays)

	// Newest first, and bounded: a dashboard is a summary, not an export.
	for i, j := 0, len(v.Recent)-1; i < j; i, j = i+1, j-1 {
		v.Recent[i], v.Recent[j] = v.Recent[j], v.Recent[i]
	}
	if len(v.Recent) > 20 {
		v.Recent = v.Recent[:20]
	}
	return v, nil
}

var tpl = template.Must(template.New("t").Parse(`
{{define "head"}}<meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<meta name="robots" content="noindex,nofollow"><title>Sehaty</title><style>
:root{--ink:#16211f;--soft:#41514c;--faint:#6d7e78;--paper:#f4f6f4;--card:#fff;--rule:#d6dcd7;--accent:#1f6f5c}
@media(prefers-color-scheme:dark){:root{--ink:#dfe5e0;--soft:#a8b4ae;--faint:#7d8a85;--paper:#101614;--card:#18211e;--rule:#26312d;--accent:#4fae93}}
*{box-sizing:border-box}body{margin:0;background:var(--paper);color:var(--ink);
font:16px/1.55 ui-serif,Georgia,serif;-webkit-font-smoothing:antialiased}
.wrap{max-width:52rem;margin:0 auto;padding:32px 20px 64px}
h1{font-family:ui-sans-serif,system-ui,sans-serif;font-size:1.9rem;margin:0 0 4px;letter-spacing:-.01em}
.sub{color:var(--faint);font-size:.85rem;margin:0 0 28px;font-family:ui-monospace,monospace}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(140px,1fr));gap:12px;margin-bottom:28px}
.card{background:var(--card);border:1px solid var(--rule);border-radius:4px;padding:14px 16px}
.k{font-family:ui-sans-serif,system-ui,sans-serif;font-size:.66rem;letter-spacing:.13em;
text-transform:uppercase;color:var(--faint);margin-bottom:6px}
.v{font-size:1.5rem;font-variant-numeric:tabular-nums;font-family:ui-sans-serif,system-ui,sans-serif}
.v small{font-size:.8rem;color:var(--faint)}
h2{font-family:ui-sans-serif,system-ui,sans-serif;font-size:.72rem;letter-spacing:.13em;
text-transform:uppercase;color:var(--faint);border-top:1px solid var(--rule);padding-top:14px;margin:0 0 10px}
table{width:100%;border-collapse:collapse;font-size:.9rem}
td{padding:7px 0;border-bottom:1px solid var(--rule);vertical-align:top}
td.d{color:var(--faint);font-family:ui-monospace,monospace;font-size:.78rem;width:6.5rem;white-space:nowrap}
td.x{color:var(--soft);text-align:right;font-variant-numeric:tabular-nums;white-space:nowrap}
.warn{background:var(--card);border-left:3px solid var(--accent);padding:11px 14px;margin-bottom:22px;font-size:.9rem}
.empty{color:var(--faint);font-style:italic}
footer{margin-top:34px;padding-top:14px;border-top:1px solid var(--rule);color:var(--faint);
font-size:.75rem;font-family:ui-monospace,monospace}
{{end}}

{{define "page"}}<!doctype html><html lang="en"><head>{{template "head"}}</style></head><body><div class="wrap">
<h1>{{.Name}}</h1>
<p class="sub">last 30 days · goal {{.Goal}} · link expires {{.Expires}}</p>
{{if .Limits}}<div class="warn"><strong>Stated limitations:</strong> {{range $i,$l := .Limits}}{{if $i}}, {{end}}{{$l}}{{end}}</div>{{end}}
<div class="grid">
<div class="card"><div class="k">Weight</div><div class="v">{{if .Weight}}{{.WeightStr}} <small>kg</small><br><small>{{.WeightAgo}}</small>{{else}}<small class="empty">not recorded</small>{{end}}</div></div>
<div class="card"><div class="k">Sessions</div><div class="v">{{.Sessions}}</div></div>
<div class="card"><div class="k">Sets</div><div class="v">{{.Sets}}</div></div>
<div class="card"><div class="k">Cardio</div><div class="v">{{.Cardio}} <small>min</small></div></div>
<div class="card"><div class="k">Days logged</div><div class="v">{{.FoodDays}}</div></div>
<div class="card"><div class="k">Protein</div><div class="v">{{printf "%.0f" .Protein}} <small>g</small></div></div>
</div>
<h2>Recent</h2>
{{if .Recent}}<table>{{range .Recent}}<tr><td class="d">{{.Date}}</td><td>{{.What}}</td><td class="x">{{.Detail}}</td></tr>{{end}}</table>
{{else}}<p class="empty">Nothing logged in the last 30 days.</p>{{end}}
<footer>Sehaty · this link stops working an hour after it was made</footer>
</div></body></html>{{end}}

{{define "gone"}}
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Sehaty — link expired</title>
<style>
:root{color-scheme:light;--ink:#16211f;--soft:#41514c;--faint:#6d7e78;--paper:#f4f6f4;--rule:#d6dcd7;--accent:#1f6f5c}
@media (prefers-color-scheme:dark){
:root{color-scheme:dark;--ink:#dfe5e0;--soft:#a8b4ae;--faint:#7d8a85;--paper:#101614;--rule:#26312d;--accent:#4fae93}
}
body{margin:0;background:var(--paper);color:var(--ink);font:400 1rem/1.65 system-ui,-apple-system,"Segoe UI",Roboto,"Helvetica Neue",sans-serif;min-height:100vh;min-height:100svh;display:grid;place-items:center;padding:3rem 1.4rem;box-sizing:border-box}
main{max-width:33rem;width:100%}
.meta{display:flex;justify-content:space-between;font:500 .7rem/1 ui-monospace,"SF Mono",Menlo,Consolas,monospace;letter-spacing:.24em;text-transform:uppercase;color:var(--faint);border-bottom:1px solid var(--rule);padding-bottom:.8rem;margin-bottom:2.4rem}
.chart{display:block;width:100%;height:auto;margin-bottom:.4rem}
.cap{font:400 .72rem/1.5 ui-monospace,"SF Mono",Menlo,Consolas,monospace;letter-spacing:.05em;color:var(--faint);margin:0 0 2.2rem}
h1{font:500 clamp(1.5rem,3.8vw + .9rem,2.05rem)/1.25 "Iowan Old Style","Palatino Linotype",Palatino,Georgia,serif;letter-spacing:-.01em;margin:0 0 1.1rem;color:var(--ink)}
p{margin:0 0 1rem;color:var(--soft)}
.next{color:var(--ink);border-left:2px solid var(--accent);padding-left:1rem;margin-top:1.4rem}
footer{margin-top:2.6rem;border-top:1px solid var(--rule);padding-top:1rem;font-size:.8rem;line-height:1.6;color:var(--faint)}
.s-acc{stroke:var(--accent)}
.s-fnt{stroke:var(--faint)}
.f-pap{fill:var(--paper)}
.draw{stroke-dasharray:620;stroke-dashoffset:620;animation:draw 1.1s ease-out .2s forwards}
.late{opacity:0;animation:show .6s ease-out 1.3s forwards}
@keyframes draw{to{stroke-dashoffset:0}
}
@keyframes show{to{opacity:1}
}
@media (prefers-reduced-motion:reduce){
.draw{animation:none;stroke-dasharray:none;stroke-dashoffset:0}
.late{animation:none;opacity:1}
}
</style>
</head>
<body>
<main>
<header class="meta"><span>Sehaty</span><span>401</span></header>
<svg class="chart" viewBox="0 0 560 120" fill="none" aria-hidden="true">
<path class="s-acc draw" d="M20 72 C74 64 108 80 156 68 C204 56 236 50 282 57 C316 62 336 61 348 59" stroke-width="2" stroke-linecap="round"/>
<path class="s-fnt late" d="M366 58 C424 54 484 59 540 56" stroke-width="2.5" stroke-linecap="round" stroke-dasharray=".1 9"/>
<circle class="s-acc f-pap late" cx="348" cy="59" r="4.5" stroke-width="2"/>
</svg>
<p class="cap">fig. 401 — a key lasts one hour; the record goes on</p>
<h1>This link is past its hour.</h1>
<p>Links to a Sehaty dashboard work for one hour from the moment they are made. This is deliberate: a link is the only key to the record it opens, so it is not allowed to outlive the moment it was asked for.</p>
<p>If this one sat in a chat for a while before you tapped it, nothing is wrong and nothing has been lost. The dashboard is still there, exactly as it was.</p>
<p class="next">Ask your agent for a fresh link. It takes a moment, and opens the same dashboard.</p>
<footer>Sehaty is a private, self-hosted health record — encrypted at rest, reached only by signed links.</footer>
</main>
</body>
</html>
{{end}}

{{define "notfound"}}
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Sehaty — nothing at this address</title>
<style>
:root{color-scheme:light;--ink:#16211f;--soft:#41514c;--faint:#6d7e78;--paper:#f4f6f4;--rule:#d6dcd7;--accent:#1f6f5c}
@media (prefers-color-scheme:dark){
:root{color-scheme:dark;--ink:#dfe5e0;--soft:#a8b4ae;--faint:#7d8a85;--paper:#101614;--rule:#26312d;--accent:#4fae93}
}
body{margin:0;background:var(--paper);color:var(--ink);font:400 1rem/1.65 system-ui,-apple-system,"Segoe UI",Roboto,"Helvetica Neue",sans-serif;min-height:100vh;min-height:100svh;display:grid;place-items:center;padding:3rem 1.4rem;box-sizing:border-box}
main{max-width:33rem;width:100%}
.meta{display:flex;justify-content:space-between;font:500 .7rem/1 ui-monospace,"SF Mono",Menlo,Consolas,monospace;letter-spacing:.24em;text-transform:uppercase;color:var(--faint);border-bottom:1px solid var(--rule);padding-bottom:.8rem;margin-bottom:2.4rem}
.chart{display:block;width:100%;height:auto;margin-bottom:.4rem}
.cap{font:400 .72rem/1.5 ui-monospace,"SF Mono",Menlo,Consolas,monospace;letter-spacing:.05em;color:var(--faint);margin:0 0 2.2rem}
h1{font:500 clamp(1.5rem,3.8vw + .9rem,2.05rem)/1.25 "Iowan Old Style","Palatino Linotype",Palatino,Georgia,serif;letter-spacing:-.01em;margin:0 0 1.1rem;color:var(--ink)}
p{margin:0 0 1rem;color:var(--soft)}
.next{color:var(--ink);border-left:2px solid var(--accent);padding-left:1rem;margin-top:1.4rem}
footer{margin-top:2.6rem;border-top:1px solid var(--rule);padding-top:1rem;font-size:.8rem;line-height:1.6;color:var(--faint)}
.s-fnt{stroke:var(--faint)}
.s-rul{stroke:var(--rule)}
</style>
</head>
<body>
<main>
<header class="meta"><span>Sehaty</span><span>404</span></header>
<svg class="chart" viewBox="0 0 560 120" fill="none" aria-hidden="true">
<path class="s-fnt" d="M20 64 H540" stroke-width="2.5" stroke-linecap="round" stroke-dasharray=".1 9"/>
<path class="s-rul" d="M20 78 v10 M150 78 v10 M280 78 v10 M410 78 v10 M540 78 v10" stroke-width="2" stroke-linecap="round"/>
</svg>
<p class="cap">fig. 404 — no series recorded at this address</p>
<h1>There is nothing at this address.</h1>
<p>Not an error in the usual sense. Sehaty has no front page, no directory of people, and no pages to browse — every dashboard is reached by its own signed link, and by nothing else.</p>
<p>An address typed by hand, trimmed by a chat app, or guessed will land here, whatever it was meant to reach.</p>
<p class="next">If someone shared a dashboard with you, ask them for the link itself and open it exactly as it was sent.</p>
<footer>Sehaty is a private, self-hosted health record — encrypted at rest, reached only by signed links.</footer>
</main>
</body>
</html>
{{end}}

{{define "servererror"}}
<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Sehaty — something went wrong</title>
<style>
:root{color-scheme:light;--ink:#16211f;--soft:#41514c;--faint:#6d7e78;--paper:#f4f6f4;--rule:#d6dcd7;--accent:#1f6f5c}
@media (prefers-color-scheme:dark){
:root{color-scheme:dark;--ink:#dfe5e0;--soft:#a8b4ae;--faint:#7d8a85;--paper:#101614;--rule:#26312d;--accent:#4fae93}
}
body{margin:0;background:var(--paper);color:var(--ink);font:400 1rem/1.65 system-ui,-apple-system,"Segoe UI",Roboto,"Helvetica Neue",sans-serif;min-height:100vh;min-height:100svh;display:grid;place-items:center;padding:3rem 1.4rem;box-sizing:border-box}
main{max-width:33rem;width:100%}
.meta{display:flex;justify-content:space-between;font:500 .7rem/1 ui-monospace,"SF Mono",Menlo,Consolas,monospace;letter-spacing:.24em;text-transform:uppercase;color:var(--faint);border-bottom:1px solid var(--rule);padding-bottom:.8rem;margin-bottom:2.4rem}
.chart{display:block;width:100%;height:auto;margin-bottom:.4rem}
.cap{font:400 .72rem/1.5 ui-monospace,"SF Mono",Menlo,Consolas,monospace;letter-spacing:.05em;color:var(--faint);margin:0 0 2.2rem}
h1{font:500 clamp(1.5rem,3.8vw + .9rem,2.05rem)/1.25 "Iowan Old Style","Palatino Linotype",Palatino,Georgia,serif;letter-spacing:-.01em;margin:0 0 1.1rem;color:var(--ink)}
p{margin:0 0 1rem;color:var(--soft)}
.next{color:var(--ink);border-left:2px solid var(--accent);padding-left:1rem;margin-top:1.4rem}
footer{margin-top:2.6rem;border-top:1px solid var(--rule);padding-top:1rem;font-size:.8rem;line-height:1.6;color:var(--faint)}
.s-acc{stroke:var(--accent)}
.s-fnt{stroke:var(--faint)}
.draw{stroke-dasharray:620;stroke-dashoffset:620;animation:draw 1s ease-out .2s forwards}
.late{opacity:0;animation:show .6s ease-out 1.4s forwards}
@keyframes draw{to{stroke-dashoffset:0}
}
@keyframes show{to{opacity:1}
}
@media (prefers-reduced-motion:reduce){
.draw{animation:none;stroke-dasharray:none;stroke-dashoffset:0}
.late{animation:none;opacity:1}
}
</style>
</head>
<body>
<main>
<header class="meta"><span>Sehaty</span><span>500</span></header>
<svg class="chart" viewBox="0 0 560 120" fill="none" aria-hidden="true">
<path class="s-acc draw" d="M20 68 C66 60 112 76 158 64 C196 54 226 51 252 56" stroke-width="2" stroke-linecap="round"/>
<path class="s-fnt late" d="M268 55 L296 54" stroke-width="2.5" stroke-linecap="round" stroke-dasharray=".1 9"/>
<path class="s-acc draw" style="animation-delay:.7s" d="M312 53 C368 47 428 62 486 55 C508 52 526 55 540 54" stroke-width="2" stroke-linecap="round"/>
</svg>
<p class="cap">fig. 500 — a gap in the rendering, not in the record</p>
<h1>We could not build this page.</h1>
<p>Something failed on our side while the dashboard was being put together. The fault is ours, and there is nothing here for you to fix.</p>
<p>Your records are stored, encrypted, apart from the pages that display them. A page that fails to render leaves them untouched.</p>
<p class="next">Try your link again in a minute. If it keeps failing, the details are already in the server&rsquo;s log for whoever runs it.</p>
<footer>Sehaty is a private, self-hosted health record — encrypted at rest, reached only by signed links.</footer>
</main>
</body>
</html>
{{end}}
`))
