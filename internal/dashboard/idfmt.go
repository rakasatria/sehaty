package dashboard

import (
	"fmt"
	"strings"
	"time"
)

// Everything a number becomes on the way to the page happens HERE.
//
// The client formats nothing — not a decimal separator, not a rounding, not a
// date. That is not a style preference: the rules about what may be shown live in
// Go, next to the data, and a second formatter in JavaScript would be a second
// place for a value nobody measured to appear. So these functions are the only
// route from a float to something a person reads.

var idDays = [...]string{"Min", "Sen", "Sel", "Rab", "Kam", "Jum", "Sab"}

var idMonths = [...]string{"", "Jan", "Feb", "Mar", "Apr", "Mei", "Jun",
	"Jul", "Agu", "Sep", "Okt", "Nov", "Des"}

// idNum formats a number the Indonesian way: comma for the decimal, dot for
// thousands. 1782.5 becomes "1.782,5".
func idNum(v float64, decimals int) string {
	s := fmt.Sprintf("%.*f", decimals, v)
	intPart, frac := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, frac = s[:i], s[i+1:]
	}
	neg := strings.HasPrefix(intPart, "-")
	intPart = strings.TrimPrefix(intPart, "-")

	var b strings.Builder
	for i, r := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(r)
	}
	out := b.String()
	if frac != "" {
		out += "," + frac
	}
	if neg {
		// U+2212, the real minus sign. A hyphen next to a weight reads as a
		// dash or a range; the client's typography assumes this character.
		out = "−" + out
	}
	return out
}

// idSigned always carries its sign, because "1,8 kg" and "−1,8 kg" mean opposite
// things and a missing plus is read as neither.
func idSigned(v float64, decimals int) string {
	if v > 0 {
		return "+" + idNum(v, decimals)
	}
	return idNum(v, decimals)
}

// idShort renders 2026-09-10 as "10 Sep".
func idShort(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return fmt.Sprintf("%d %s", t.Day(), idMonths[int(t.Month())])
}

// idFull renders 2026-09-10 as "Kam 10 Sep".
//
// Stable within a day, which matters: the day view selects entries by matching
// this exact string, so two renderings of the same date must agree.
func idFull(date string) string {
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return date
	}
	return fmt.Sprintf("%s %d %s", idDays[int(t.Weekday())], t.Day(), idMonths[int(t.Month())])
}

// idGoal turns a stored goal into words. The client shows this verbatim.
var idGoal = map[string]string{
	"fat_loss":    "menurunkan lemak",
	"strength":    "menambah kekuatan",
	"hypertrophy": "menambah massa otot",
	"general":     "menjaga kondisi",
}

func goalText(goal string) string {
	if s, ok := idGoal[goal]; ok {
		return s
	}
	return strings.ReplaceAll(goal, "_", " ")
}
