package dashboard

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rakasatria/sehaty/internal/dashlink"
)

// The API is the only thing that hands out a record, so it is the only thing
// that has to be right about who is asking.
func TestTheApiRefusesEveryRequestWithoutAValidCredential(t *testing.T) {
	d, id := setup(t)
	d.WebApp = func(init string, _ time.Time) (string, error) {
		if init == "good" {
			return id, nil
		}
		return "", errRefused
	}
	h := Handler(d)

	for _, tc := range []struct {
		name   string
		header string
		value  string
	}{
		{"no credential at all", "", ""},
		{"an empty launch string", "X-Telegram-Init-Data", ""},
		{"a forged launch string", "X-Telegram-Init-Data", "forged"},
		{"an oversized launch string", "X-Telegram-Init-Data", strings.Repeat("x", 9000)},
		{"a forged dashboard token", "X-Dashboard-Token", "not.a.real.token"},
		{"an empty dashboard token", "X-Dashboard-Token", ""},
	} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/summary?days=30", nil)
		if tc.header != "" {
			req.Header.Set(tc.header, tc.value)
		}
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("%s: answered %d, want 401", tc.name, rr.Code)
		}
		if strings.Contains(rr.Body.String(), "Raka") {
			t.Errorf("%s: a refusal leaked the record", tc.name)
		}
	}
}

// And accepts the two that are real.
func TestBothDoorsOpenWithTheRightCredential(t *testing.T) {
	d, id := setup(t)
	d.WebApp = func(init string, _ time.Time) (string, error) {
		if init == "good" {
			return id, nil
		}
		return "", errRefused
	}
	h := Handler(d)

	token, _ := dashlink.Mint(d.Key, id, time.Now())

	for name, set := range map[string]func(*http.Request){
		"a Telegram launch": func(r *http.Request) { r.Header.Set("X-Telegram-Init-Data", "good") },
		"a signed link":     func(r *http.Request) { r.Header.Set("X-Dashboard-Token", token) },
	} {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/api/summary?days=30", nil)
		set(req)
		h.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("%s: answered %d, want 200 — %s", name, rr.Code, rr.Body.String())
		}
		var got summary
		if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
			t.Fatalf("%s: unreadable body: %v", name, err)
		}
		if got.Name != "Raka" {
			t.Errorf("%s: returned %q", name, got.Name)
		}
		if rr.Header().Get("Cache-Control") != "no-store, private" {
			t.Errorf("%s: a health record was served cacheable", name)
		}
	}
}

// Nothing unrecorded may arrive as a zero. The client draws "belum dicatat" from
// a null and a real figure from a number; collapsing them would turn "we have no
// idea" into "you did nothing", which is a different and crueller claim.
func TestUnrecordedArrivesAsNullNotZero(t *testing.T) {
	d, id := setup(t)
	d.WebApp = func(string, time.Time) (string, error) { return id, nil }

	out, err := buildSummary(d, id, 30)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(out)
	var generic map[string]any
	_ = json.Unmarshal(raw, &generic)

	training := generic["training"].(map[string]any)
	for _, k := range []string{"days", "sets", "cardioMinutes"} {
		if v, present := training[k]; present && v != nil {
			if n, isNum := v.(float64); isNum && n == 0 {
				t.Errorf("training.%s serialised as 0 instead of null", k)
			}
		}
	}
	food := generic["food"].(map[string]any)
	for _, k := range []string{"daysLogged", "kcal", "protein"} {
		if v, present := food[k]; present && v != nil {
			if n, isNum := v.(float64); isNum && n == 0 {
				t.Errorf("food.%s serialised as 0 instead of null", k)
			}
		}
	}
}

// The client formats nothing, so these are the only place a number becomes
// readable — and Indonesian punctuates the opposite way round from English.
func TestNumbersAreIndonesianAndTheMinusIsReal(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"1782.5|1", "1.782,5"},
		// 77.35 is not exactly representable in float64 — it is stored as
		// 77.34999…, so 77,3 is the honest rendering of the value actually held.
		// Rounding it up would mean displaying a weight nobody recorded, which is
		// the precise failure this whole application is built to avoid.
		{"77.35|1", "77,3"},
		{"1930|0", "1.930"},
		{"999|0", "999"},
		{"-1.8|1", "−1,8"},
	} {
		parts := strings.Split(tc.in, "|")
		v, _ := strconv.ParseFloat(parts[0], 64)
		dec, _ := strconv.Atoi(parts[1])
		if got := idNum(v, dec); got != tc.want {
			t.Errorf("idNum(%v, %d) = %q, want %q", v, dec, got, tc.want)
		}
	}
	// U+2212, not a hyphen. The client's typography assumes it, and a hyphen
	// beside a weight reads as a range.
	if !strings.Contains(idNum(-1.8, 1), "−") {
		t.Error("a negative used a hyphen instead of a real minus sign")
	}
	if got := idSigned(1.8, 1); got != "+1,8" {
		t.Errorf("a gain came back as %q — an unsigned number reads as neither", got)
	}
}

func TestDatesAreHumanisedAndStable(t *testing.T) {
	if got := idFull("2026-09-10"); got != "Kam 10 Sep" {
		t.Errorf("idFull = %q, want %q", got, "Kam 10 Sep")
	}
	if got := idShort("2026-08-12"); got != "12 Agu" {
		t.Errorf("idShort = %q, want %q", got, "12 Agu")
	}
	// The day view matches entries by this exact string, so it must not drift.
	if idFull("2026-09-10") != idFull("2026-09-10") {
		t.Error("the same date rendered two ways")
	}
	// A date it cannot parse comes back untouched rather than as a wrong date.
	if got := idFull("not a date"); got != "not a date" {
		t.Errorf("an unparseable date became %q", got)
	}
}

// errRefused stands in for whatever the real verifier returns; the API must not
// care which failure it was.
var errRefused = errors.New("refused")
