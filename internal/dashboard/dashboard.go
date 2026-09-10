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

{{define "state"}}
.state{max-width:31rem;margin:13vh auto 0;text-align:center}
.mark{width:76px;height:76px;color:var(--rule);margin-bottom:24px}
.state h1{font-size:1.55rem;margin:0 0 12px;letter-spacing:-.01em}
.state p{color:var(--soft);margin:0 0 22px;font-size:.97rem}
.hint{display:inline-block;border:1px solid var(--rule);border-radius:4px;padding:11px 16px;
background:var(--card);color:var(--faint);font-size:.83rem;
font-family:ui-monospace,SFMono-Regular,monospace;text-align:left;line-height:1.7}
.hint b{color:var(--soft);font-weight:600;font-family:ui-sans-serif,system-ui,sans-serif;
font-size:.7rem;letter-spacing:.12em;text-transform:uppercase}
{{end}}

{{define "gone"}}<!doctype html><html lang="en"><head>{{template "head"}}{{template "state"}}
</style></head><body><div class="wrap"><div class="state">
<svg class="mark" viewBox="0 0 100 100" aria-hidden="true" fill="none" stroke="currentColor"
 stroke-width="1.5" stroke-linecap="round"><circle cx="50" cy="50" r="33"/><path d="M50 29v22l14 9"/></svg>
<h1>This link has expired</h1>
<p>Dashboard links last one&nbsp;hour. That is deliberate — the link is the only key, so it
should not outlive the moment you asked for&nbsp;it.</p>
<div class="hint"><b>To get back in</b><br>Ask your agent for a new dashboard link.</div>
</div></div></body></html>{{end}}

{{define "servererror"}}<!doctype html><html lang="en"><head>{{template "head"}}{{template "state"}}
</style></head><body><div class="wrap"><div class="state">
<svg class="mark" viewBox="0 0 100 100" aria-hidden="true" fill="none" stroke="currentColor"
 stroke-width="1.5" stroke-linecap="round"><circle cx="50" cy="50" r="33"/><path d="M50 34v20"/><path d="M50 64v1"/></svg>
<h1>Something broke on our side</h1>
<p>This is our fault, not yours, and your records are untouched — nothing was lost.</p>
<div class="hint"><b>Nothing for you to fix</b><br>Try the same link again shortly.</div>
</div></div></body></html>{{end}}

{{define "notfound"}}<!doctype html><html lang="en"><head>{{template "head"}}{{template "state"}}
</style></head><body><div class="wrap"><div class="state">
<svg class="mark" viewBox="0 0 100 100" aria-hidden="true" fill="none" stroke="currentColor"
 stroke-width="1.5" stroke-linecap="round"><circle cx="50" cy="50" r="33"/><path d="M35 50h30"/></svg>
<h1>Nothing here</h1>
<p>Sehaty has no front page and no directory of people. Every dashboard is reached by its
own link and nothing&nbsp;else.</p>
<div class="hint"><b>Looking for your dashboard?</b><br>Ask your agent for a link.</div>
</div></div></body></html>{{end}}
`))
