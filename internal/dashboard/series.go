package dashboard

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

// Everything a chart needs is computed HERE, in Go, and handed to the template as finished
// coordinates and formatted strings.
//
// Go templates cannot do arithmetic. The alternative — letting a template approximate a
// scale — is how a health page ends up drawing a line that does not match the numbers
// beside it. Precomputing means the picture and the figures come from the same maths.

// Point is one plotted observation: the real values, and where it sits in the viewBox.
type Point struct {
	Date  string
	Label string // formatted value, e.g. "87.4"
	Value float64
	X, Y  float64
}

// Series is a plotted line ready to render.
type Series struct {
	Points  []Point
	Path    string // SVG polyline points attribute, "x,y x,y …"
	Area    string // same, closed to the baseline, for a fill
	Min     float64
	Max     float64
	MinLbl  string
	MaxLbl  string
	First   Point
	Last    Point
	Change  float64 // last minus first
	ChangeL string  // formatted with an explicit sign, e.g. "−1.8"
	Days    int     // span covered
	Empty   bool
}

// Bar is one column in a daily-activity strip.
type Bar struct {
	Date    string
	Label   string
	Value   float64
	X       float64
	Y       float64
	H       float64
	Weekend bool
	Zero    bool // nothing recorded that day — drawn as absence, never as a zero-height bar
}

const (
	chartW = 640.0
	chartH = 150.0
	padX   = 10.0
	padY   = 14.0
)

// buildSeries turns dated observations into a plotted line.
//
// The Y scale is padded away from the data's own min and max: a weight range of 1.2 kg
// drawn edge to edge makes normal fluctuation look like a collapse. Health data plotted
// without headroom is alarming by accident.
func buildSeries(dates []string, values []float64, decimals int) Series {
	s := Series{Empty: len(values) == 0}
	if s.Empty {
		return s
	}
	s.Min, s.Max = values[0], values[0]
	for _, v := range values {
		s.Min = math.Min(s.Min, v)
		s.Max = math.Max(s.Max, v)
	}
	span := s.Max - s.Min
	if span < 1e-9 {
		// A single value, or a flat line: invent a window around it rather than
		// dividing by zero and pinning everything to one edge.
		span = math.Max(math.Abs(s.Max)*0.04, 1)
		s.Min -= span / 2
		s.Max += span / 2
	} else {
		pad := span * 0.25
		s.Min -= pad
		s.Max += pad
	}
	f := "%." + fmt.Sprint(decimals) + "f"
	s.MinLbl = fmt.Sprintf(f, s.Min)
	s.MaxLbl = fmt.Sprintf(f, s.Max)

	n := len(values)
	var path, area strings.Builder
	for i, v := range values {
		x := padX
		if n > 1 {
			x = padX + (chartW-2*padX)*float64(i)/float64(n-1)
		} else {
			x = chartW / 2
		}
		y := padY + (chartH-2*padY)*(1-(v-s.Min)/(s.Max-s.Min))
		p := Point{Date: dates[i], Value: v, Label: fmt.Sprintf(f, v),
			X: round1(x), Y: round1(y)}
		s.Points = append(s.Points, p)
		fmt.Fprintf(&path, "%g,%g ", p.X, p.Y)
		fmt.Fprintf(&area, "%g,%g ", p.X, p.Y)
	}
	s.Path = strings.TrimSpace(path.String())
	s.Area = fmt.Sprintf("%g,%g %s %g,%g", s.Points[0].X, chartH-padY/2,
		strings.TrimSpace(area.String()), s.Points[n-1].X, chartH-padY/2)
	s.First, s.Last = s.Points[0], s.Points[n-1]
	s.Change = s.Last.Value - s.First.Value
	sign := "+"
	if s.Change < 0 {
		sign = "−" // a real minus sign, not a hyphen
	}
	s.ChangeL = sign + fmt.Sprintf(f, math.Abs(s.Change))
	if t0, err := time.Parse("2006-01-02", dates[0]); err == nil {
		if t1, err := time.Parse("2006-01-02", dates[n-1]); err == nil {
			s.Days = int(t1.Sub(t0).Hours()/24) + 1
		}
	}
	return s
}

// buildBars lays out one column per day across a fixed window, INCLUDING days with
// nothing recorded — a gap in training is information, and collapsing the axis to only
// the days that have data would hide exactly the pattern worth seeing.
func buildBars(byDate map[string]float64, days int, end time.Time, decimals int) []Bar {
	var max float64
	for _, v := range byDate {
		max = math.Max(max, v)
	}
	f := "%." + fmt.Sprint(decimals) + "f"
	out := make([]Bar, 0, days)
	slot := (chartW - 2*padX) / float64(days)
	for i := 0; i < days; i++ {
		d := end.AddDate(0, 0, -(days - 1 - i))
		key := d.Format("2006-01-02")
		v := byDate[key]
		b := Bar{Date: key, Value: v, Label: fmt.Sprintf(f, v),
			X:       round1(padX + slot*float64(i)),
			Weekend: d.Weekday() == time.Saturday || d.Weekday() == time.Sunday,
			Zero:    v == 0}
		if max > 0 && v > 0 {
			b.H = round1((chartH - 2*padY) * v / max)
		}
		b.Y = round1(chartH - padY - b.H)
		out = append(out, b)
	}
	return out
}

func round1(v float64) float64 { return math.Round(v*10) / 10 }

// daily collapses dated rows into one value per day.
func daily(dates []string, values []float64) map[string]float64 {
	m := map[string]float64{}
	for i := range dates {
		m[dates[i]] += values[i]
	}
	return m
}

// lastPerDay keeps the final observation of each day, then orders by date. Weight is
// measured, not accumulated: two weigh-ins in a day are two readings, not 175 kg.
func lastPerDay(dates []string, values []float64) ([]string, []float64) {
	m := map[string]float64{}
	for i := range dates {
		m[dates[i]] = values[i]
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]float64, 0, len(keys))
	for _, k := range keys {
		out = append(out, m[k])
	}
	return keys, out
}
