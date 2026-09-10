package dashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/rakasatria/sehaty/internal/capability"
	"github.com/rakasatria/sehaty/internal/storage"
)

// summary is the wire shape the Mini App consumes.
//
// Every string here is DISPLAY-READY and every nullable field is a genuine
// "not recorded" rather than a zero — the same distinction the tools make, and
// the reason a pointer appears where a plain int would be simpler. Chart
// coordinates are normalised 0..1 with y measured downward, because the client
// places points and must never derive one.
type summary struct {
	Name      string             `json:"name"`
	Goal      string             `json:"goal"`
	Equipment []string           `json:"equipment"`
	Window    windowInfo         `json:"window"`
	Weight    weightBlock        `json:"weight"`
	Training  training           `json:"training"`
	Food      food               `json:"food"`
	Recent    []entry            `json:"recent"`
	Limits    []string           `json:"limitations"`
	Locked    []lockedCapability `json:"locked"`
}

type windowInfo struct {
	Days  int    `json:"days"`
	Label string `json:"label"`
}

type weightBlock struct {
	Latest *latest `json:"latest"`
	Series []point `json:"series"`
	Change *string `json:"change"`
}

type latest struct {
	Value string `json:"value"`
	Unit  string `json:"unit"`
	Date  string `json:"date"`
}

type point struct {
	Date  string  `json:"date"`
	Label string  `json:"label"`
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
}

type training struct {
	Days          *int `json:"days"`
	Sets          *int `json:"sets"`
	CardioMinutes *int `json:"cardioMinutes"`
}

type food struct {
	DaysLogged *int    `json:"daysLogged"`
	Kcal       *string `json:"kcal"`
	Protein    *string `json:"protein"`
}

type entry struct {
	Date   string `json:"date"`
	What   string `json:"what"`
	Detail string `json:"detail"`
}

// lockedCapability is something this record cannot do yet, and why.
//
// Shown rather than discovered: finding out by asking and being refused reads as the
// software being broken, while a named gap with a named reason reads as a record that
// knows what it is short of.
type lockedCapability struct {
	Unlocks string       `json:"unlocks"`
	Needs   []lockedNeed `json:"needs"`
}

type lockedNeed struct {
	Field   string `json:"field"`
	Because string `json:"because"`
}

// nz returns a pointer only when something was actually recorded. A zero count
// and an unrecorded count are different facts and the page renders them
// differently, so they must not arrive as the same JSON.
func nz(n int) *int {
	if n == 0 {
		return nil
	}
	v := n
	return &v
}

// apiSummary serves the Mini App's data.
//
// Authentication is the same question the pages ask, in the same order: a
// Telegram launch string, or the signed token from a dashboard link. One
// refusal for every failure — distinguishing "expired" from "forged" would
// confirm to a forger that the id inside was real.
func (d Deps) apiSummary(w http.ResponseWriter, r *http.Request) {
	profileID, ok := d.authenticate(r)
	if !ok {
		w.Header().Set("Cache-Control", "no-store, private")
		http.Error(w, `{"error":"not authorised"}`, http.StatusUnauthorized)
		return
	}

	days := 30
	if q := r.URL.Query().Get("days"); q != "" {
		if n, err := strconv.Atoi(q); err == nil {
			switch n {
			case 7, 30, 90:
				days = n
			}
		}
	}

	out, err := buildSummary(d, profileID, days)
	if err != nil {
		w.Header().Set("Cache-Control", "no-store, private")
		http.Error(w, `{"error":"unavailable"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store, private")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_ = json.NewEncoder(w).Encode(out)
}

func buildSummary(d Deps, profileID string, days int) (summary, error) {
	p, err := d.DB.GetProfile(profileID)
	if err != nil {
		return summary{}, err
	}
	name := p.DisplayName
	if name == "" {
		name = p.ID
	}
	out := summary{
		Name:      name,
		Goal:      goalText(p.Goal),
		Equipment: nonNil(p.Equipment),
		Window:    windowInfo{Days: days, Label: fmt.Sprintf("%d hari terakhir", days)},
		Recent:    []entry{},
		Limits:    nonNil(p.Limitations),
		Locked:    []lockedCapability{},
		Weight:    weightBlock{Series: []point{}},
	}

	if ws, err := d.DB.Weights(profileID, days); err == nil && len(ws) > 0 {
		last := ws[len(ws)-1]
		out.Weight.Latest = &latest{
			Value: idNum(last.WeightKg, 1), Unit: "kg", Date: idShort(last.Date),
		}

		dates := make([]string, 0, len(ws))
		vals := make([]float64, 0, len(ws))
		for _, entry := range ws {
			dates = append(dates, entry.Date)
			vals = append(vals, entry.WeightKg)
		}
		// lastPerDay, not daily: weight is MEASURED, so two weigh-ins on one
		// day are two readings of the same thing and the later one wins.
		dd, vv := lastPerDay(dates, vals)
		s := buildSeries(dd, vv, 1)
		for _, pt := range s.Points {
			out.Weight.Series = append(out.Weight.Series, point{
				Date:  idShort(pt.Date),
				Label: idNum(pt.Value, 1) + " kg",
				// buildSeries works in a fixed viewBox; the client wants 0..1
				// with y already measured downward, which it is.
				X: clamp01(pt.X / chartW),
				Y: clamp01(pt.Y / chartH),
			})
		}
		if len(out.Weight.Series) == 1 {
			out.Weight.Series[0].X = 0.5
		}
		if len(s.Points) > 1 {
			ch := idSigned(s.Change, 1)
			out.Weight.Change = &ch
		}
	}

	sets, _ := d.DB.Sets(profileID, days)
	trainingDays := map[string]bool{}
	byDate := map[string][]string{}
	for _, s := range sets {
		trainingDays[s.Date] = true
		byDate[s.Date] = append(byDate[s.Date], s.Exercise)
		out.Recent = append(out.Recent, entry{
			Date: idFull(s.Date), What: s.Exercise,
			Detail: fmt.Sprintf("%d × %d", s.Sets, s.Reps),
		})
	}
	out.Training.Days = nz(len(trainingDays))
	out.Training.Sets = nz(len(sets))

	cardio, _ := d.DB.Cardio(profileID, days)
	mins := 0
	for _, c := range cardio {
		mins += int(c.Minutes)
		out.Recent = append(out.Recent, entry{
			Date: idFull(c.Date), What: "Kardio",
			Detail: fmt.Sprintf("%d menit", int(c.Minutes)),
		})
	}
	out.Training.CardioMinutes = nz(mins)

	foods, _ := d.DB.Foods(profileID, days)
	foodDays := map[string]bool{}
	var kcal, protein float64
	for _, f := range foods {
		foodDays[f.Date] = true
		kcal += f.Kcal
		protein += f.ProteinG
		out.Recent = append(out.Recent, entry{
			Date: idFull(f.Date), What: f.Item,
			Detail: fmt.Sprintf("%s g · %s kkal", idNum(f.Grams, 0), idNum(f.Kcal, 0)),
		})
	}
	out.Food.DaysLogged = nz(len(foodDays))
	if len(foodDays) > 0 {
		// Whole-window totals, and labelled as such by the client. Rounded here
		// because floating-point accumulation produces 1273.4999999999998, and a
		// health record must not display a number nobody logged.
		k := idNum(kcal, 0) + " kkal"
		pr := idNum(protein, 0) + " g"
		out.Food.Kcal = &k
		out.Food.Protein = &pr
	}

	// Newest first, and bounded: a dashboard is a summary, not an export.
	sort.SliceStable(out.Recent, func(i, j int) bool { return i > j })
	if len(out.Recent) > 40 {
		out.Recent = out.Recent[:40]
	}
	// The same registry the tools refuse from and the brief asks from. Three readers,
	// one declaration — a lock the person is shown cannot disagree with a refusal they
	// would get, because both are the same fact.
	weighed := false
	if ws, err := d.DB.Weights(profileID, 3650); err == nil && len(ws) > 0 {
		weighed = true
	}
	for _, c := range capability.Locked(p.Have(weighed)) {
		lc := lockedCapability{Unlocks: c.Unlocks}
		for _, n := range c.Needs {
			lc.Needs = append(lc.Needs, lockedNeed{
				Field: string(n.Field), Because: n.Because})
		}
		out.Locked = append(out.Locked, lc)
	}
	return out, nil
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// nonNil keeps an absent list serialising as [] rather than null: the client
// should not have to tell "no equipment" from "field missing".
func nonNil(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

var _ = storage.Profile{}
var _ = time.Now
