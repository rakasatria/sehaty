package agent

import (
	"fmt"
	"strings"

	"github.com/rakasatria/sehaty/internal/storage"
	"github.com/rakasatria/sehaty/internal/tools"
)

// brief tells the model who it is talking to and what it still does not know.
//
// Rebuilt from the database on every message, so an answer given a minute ago is already
// reflected here and never asked for twice.
func Brief(d tools.Deps, p storage.Profile) string {
	var s strings.Builder
	s.WriteString("WHO YOU ARE TALKING TO\n")
	fmt.Fprintf(&s, "Name: %s\n", p.DisplayName)
	if p.Age > 0 {
		fmt.Fprintf(&s, "Age: %d\n", p.Age)
	}
	if p.HeightCm > 0 {
		fmt.Fprintf(&s, "Height: %d cm\n", p.HeightCm)
	}
	if p.Sex != "" {
		fmt.Fprintf(&s, "Sex: %s\n", p.Sex)
	}
	fmt.Fprintf(&s, "Goal: %s\n", strings.ReplaceAll(p.Goal, "_", " "))
	fmt.Fprintf(&s, "Experience: %s\n", p.Experience)
	fmt.Fprintf(&s, "Trains: %d times a week, %d minutes\n", p.SessionsPerWeek, p.SessionMinutes)
	if len(p.Equipment) > 0 {
		fmt.Fprintf(&s, "Equipment: %s\n", strings.Join(p.Equipment, ", "))
	}
	if len(p.Limitations) > 0 {
		fmt.Fprintf(&s, "Injuries or conditions: %s\n", strings.Join(p.Limitations, ", "))
	}
	if len(p.Allergies) > 0 {
		fmt.Fprintf(&s, "Food allergies: %s\n", strings.Join(p.Allergies, ", "))
	}
	if len(p.Dislikes) > 0 {
		fmt.Fprintf(&s, "Will not eat: %s\n", strings.Join(p.Dislikes, ", "))
	}
	if p.DietNotes != "" {
		fmt.Fprintf(&s, "How they eat: %s\n", p.DietNotes)
	}

	if d.DB != nil {
		if pr, err := tools.Progress(d, p.ID, 30); err == nil {
			s.WriteString("\nLAST 30 DAYS\n")
			if pr.LatestWeightKg != nil {
				fmt.Fprintf(&s, "Latest weight: %.1f kg\n", *pr.LatestWeightKg)
			} else {
				s.WriteString("Latest weight: never recorded\n")
			}
			fmt.Fprintf(&s, "Training: %d sessions, %d sets, %.0f minutes of cardio\n",
				pr.LiftingSessions, pr.SetsLogged, pr.CardioMinutes)
			fmt.Fprintf(&s, "Food logged on %d days\n", pr.FoodDaysLogged)
		}
	}

	missing := p.Missing()
	if len(missing) == 0 {
		s.WriteString("\nYou know everything you need about them. Do not ask profile " +
			"questions; just help.\n")
		return s.String()
	}

	s.WriteString("\nSTILL UNKNOWN, in the order to ask:\n")
	for i, f := range missing {
		if moment, gated := storage.Gated(f); gated {
			fmt.Fprintf(&s, "  %d. %s — %s\n", i+1, f, moment)
			continue
		}
		fmt.Fprintf(&s, "  %d. %s\n", i+1, f)
	}
	s.WriteString("Take the first one this moment suits, ask it at the end of a reply " +
		"that already did something useful, and ask nothing else. If this message gave " +
		"you nothing useful to do, ask nothing at all.\n")
	return s.String()
}
