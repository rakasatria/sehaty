package agent

import (
	"fmt"
	"strings"

	"github.com/rakasatria/sehaty/internal/capability"
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

	have := tools.Have(d, p)
	unmet, blocked := capability.Blocked("estimate_energy", have)
	toAsk := capability.ToAsk(have, p.Asked())

	if !blocked && len(toAsk) == 0 {
		s.WriteString("\nCONSULTATION COMPLETE. You know everything you need. Do not ask " +
			"profile questions any more; just help, and log what they tell you.\n")
		return s.String()
	}

	// FIRST CONSULTATION. Different rules apply here from the ongoing relationship,
	// and the brief says which are in force so the prompt does not have to guess.
	s.WriteString("\nYOU ARE IN THE FIRST CONSULTATION — the intake is not finished.\n")

	if !blocked {
		s.WriteString("\nYou now have age, height, sex and a weight. Before asking anything " +
			"else, call estimate_energy and give them the result — that is what they have " +
			"been answering questions FOR, and it should arrive as soon as it can be earned " +
			"rather than at the end. Then carry on with what is still missing.\n")
	} else {
		need := []string{}
		for _, r := range unmet {
			if r.Field == capability.FieldWeightLog {
				need = append(need, "a current weight")
				continue
			}
			need = append(need, string(r.Field))
		}
		fmt.Fprintf(&s, "\nStill needed before you can estimate their energy: %s. "+
			"These come first — the estimate is the point of the intake.\n",
			strings.Join(need, ", "))
	}

	if len(toAsk) > 0 {
		s.WriteString("\nSTILL UNKNOWN, in the order to ask. The reason after each one is " +
			"the true reason — give it if they ask why, and never invent a different one:\n")
		for i, r := range toAsk {
			q, _ := capability.Ask(r.Field)
			if q.Gate != "" && blocked {
				fmt.Fprintf(&s, "  %d. %s — %s — %s\n", i+1, r.Field, q.Gate, r.Because)
				continue
			}
			fmt.Fprintf(&s, "  %d. %s — %s\n", i+1, r.Field, r.Because)
		}
		s.WriteString("During the first consultation the moment-gating above is relaxed: " +
			"they came to be assessed, so height and the rest may be asked directly.\n")
	}

	if blocked {
		for _, r := range unmet {
			if r.Field == capability.FieldWeightLog {
				s.WriteString("\nThey have never been weighed. Ask for a current weight and " +
					"log it with log_weight.\n")
				break
			}
		}
	}

	s.WriteString("\nAsk the NEXT question as soon as they answer the last one — do not wait " +
		"for something useful to do first, and do not pad between questions. One question " +
		"per message still, buttons where offer_choices has them, and skip is always a " +
		"complete answer. When the intake is done, offer to build their plan.\n")

	return s.String()
}
