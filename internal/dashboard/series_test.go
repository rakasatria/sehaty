package dashboard

import (
	"math"
	"strings"
	"testing"
	"time"
)

// A weight range of a kilo drawn edge to edge makes normal fluctuation look like a
// collapse. Health data plotted without headroom is alarming by accident.
func TestSeriesPadsTheScaleAwayFromTheData(t *testing.T) {
	dates := []string{"2026-09-01", "2026-09-05", "2026-09-10"}
	vals := []float64{87.4, 86.9, 86.2}
	s := buildSeries(dates, vals, 1)
	if s.Min >= 86.2 || s.Max <= 87.4 {
		t.Fatalf("scale %v..%v does not contain the data with headroom", s.Min, s.Max)
	}
	// No point may sit on the very edge of the box.
	for _, p := range s.Points {
		if p.Y <= 0 || p.Y >= chartH {
			t.Errorf("point %s at y=%v is outside the chart", p.Date, p.Y)
		}
	}
}

// A flat line must not divide by zero and pin everything to one edge.
func TestFlatSeriesStillPlots(t *testing.T) {
	s := buildSeries([]string{"2026-09-01", "2026-09-02"}, []float64{80, 80}, 1)
	if s.Empty {
		t.Fatal("a flat series reported empty")
	}
	for _, p := range s.Points {
		if math.IsNaN(p.Y) || math.IsInf(p.Y, 0) {
			t.Fatalf("flat series produced y=%v", p.Y)
		}
	}
}

func TestSingleObservationPlotsWithoutBlowingUp(t *testing.T) {
	s := buildSeries([]string{"2026-09-10"}, []float64{87.4}, 1)
	if len(s.Points) != 1 || math.IsNaN(s.Points[0].X) {
		t.Fatalf("single point: %+v", s.Points)
	}
	if s.ChangeL != "+0.0" {
		t.Errorf("change over one point = %q, want +0.0", s.ChangeL)
	}
}

func TestEmptySeriesIsMarkedNotFaked(t *testing.T) {
	s := buildSeries(nil, nil, 1)
	if !s.Empty || s.Path != "" || len(s.Points) != 0 {
		t.Fatalf("empty series produced a path: %+v", s)
	}
}

// Direction matters more than magnitude on a weight chart, and a hyphen is not a minus.
func TestChangeCarriesAnExplicitSign(t *testing.T) {
	down := buildSeries([]string{"2026-09-01", "2026-09-10"}, []float64{88.0, 86.2}, 1)
	if !strings.HasPrefix(down.ChangeL, "−") {
		t.Errorf("downward change = %q, want a leading minus", down.ChangeL)
	}
	up := buildSeries([]string{"2026-09-01", "2026-09-10"}, []float64{86.2, 88.0}, 1)
	if !strings.HasPrefix(up.ChangeL, "+") {
		t.Errorf("upward change = %q, want a leading plus", up.ChangeL)
	}
}

// A gap in training is information. Collapsing the axis to only the days that have data
// would hide exactly the pattern worth seeing.
func TestBarsIncludeDaysWithNothingRecorded(t *testing.T) {
	end := time.Date(2026, 9, 10, 0, 0, 0, 0, time.UTC)
	bars := buildBars(map[string]float64{"2026-09-10": 3, "2026-09-08": 5}, 7, end, 0)
	if len(bars) != 7 {
		t.Fatalf("%d bars for a 7-day window", len(bars))
	}
	var zeros int
	for _, b := range bars {
		if b.Zero {
			zeros++
			if b.H != 0 {
				t.Errorf("%s has nothing recorded but a height of %v", b.Date, b.H)
			}
		}
	}
	if zeros != 5 {
		t.Errorf("%d empty days marked, want 5", zeros)
	}
	if bars[len(bars)-1].Date != "2026-09-10" {
		t.Errorf("last bar is %s, want the end of the window", bars[len(bars)-1].Date)
	}
}

// Weight is measured, not accumulated: two weigh-ins in a day are two readings, not 175 kg.
func TestWeightTakesTheLastReadingPerDayNotTheSum(t *testing.T) {
	dates, vals := lastPerDay(
		[]string{"2026-09-09", "2026-09-10", "2026-09-10"},
		[]float64{88.0, 87.0, 86.5})
	if len(vals) != 2 {
		t.Fatalf("got %d days", len(vals))
	}
	if vals[1] != 86.5 {
		t.Errorf("kept %v for the doubled day, want the last reading 86.5", vals[1])
	}
	if dates[0] > dates[1] {
		t.Error("days are not in order")
	}
}

// Food IS accumulated: three meals in a day is one day's intake.
func TestFoodSumsWithinADay(t *testing.T) {
	m := daily([]string{"2026-09-10", "2026-09-10"}, []float64{500, 700})
	if m["2026-09-10"] != 1200 {
		t.Errorf("daily total = %v, want 1200", m["2026-09-10"])
	}
}
