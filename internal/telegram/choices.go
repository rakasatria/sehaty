package telegram

import (
	"fmt"

	"github.com/rakasatria/sehaty/internal/tools"
)

// choice is one button: what it says, and what it writes.
//
// apply is a closure over a value the model never sees. A button press therefore cannot
// carry a payload — Telegram hands back an index into this table and nothing else — so
// the worst a forged callback can do is pick a different one of these.
type choice struct {
	Label string
	apply func(*tools.UpdateProfileArgs)
}

// choiceSet is the buttons for one question, laid out in rows.
type choiceSet struct {
	// Prompt is a fallback only, for a channel that cannot draw buttons at all.
	Prompt  string
	PerRow  int
	Choices []choice
}

func set(field string, opts ...[2]string) []choice {
	out := make([]choice, 0, len(opts))
	for _, o := range opts {
		label, value := o[0], o[1]
		out = append(out, choice{Label: label, apply: func(a *tools.UpdateProfileArgs) {
			switch field {
			case "goal":
				a.Goal = value
			case "experience":
				a.Experience = value
			case "sex":
				a.Sex = value
			case "diet":
				a.DietPreference = value
			}
		}})
	}
	return out
}

// choices is the whole catalogue, keyed by the question names the model may offer.
var choices = map[string]choiceSet{
	"goal": {PerRow: 2, Choices: set("goal",
		[2]string{"Turun lemak", "fat_loss"},
		[2]string{"Nambah kuat", "strength"},
		[2]string{"Nambah otot", "hypertrophy"},
		[2]string{"Jaga kondisi", "general"},
	)},

	"experience": {PerRow: 3, Choices: set("experience",
		[2]string{"Baru mulai", "beginner"},
		[2]string{"Udah lumayan", "intermediate"},
		[2]string{"Udah lama", "advanced"},
	)},

	"sex": {PerRow: 3, Choices: set("sex",
		[2]string{"Cowok", "male"},
		[2]string{"Cewek", "female"},
		[2]string{"Lainnya", "other"},
	)},

	"diet preference": {PerRow: 3, Choices: set("diet",
		[2]string{"Makan semua", "non_vegetarian"},
		[2]string{"Vegetarian", "vegetarian"},
		[2]string{"Vegan", "vegan"},
	)},

	"sessions per week": {PerRow: 5, Choices: weekly()},
	"session minutes":   {PerRow: 4, Choices: minutes()},
	"equipment":         {PerRow: 2, Choices: equipmentSets()},

	// Both of these are usually "none", and typing "tidak ada" to say so is friction on
	// the one answer most people give. Anything else stays free text, because an injury
	// in somebody's own words is worth more than a category.
	"injuries or conditions": {PerRow: 1, Choices: []choice{
		{Label: "Nggak ada", apply: func(a *tools.UpdateProfileArgs) {
			a.Limitations = []string{}
		}},
	}},
	"food allergies": {PerRow: 1, Choices: []choice{
		{Label: "Nggak ada alergi", apply: func(a *tools.UpdateProfileArgs) {
			a.Allergies = []string{}
		}},
	}},
}

func weekly() []choice {
	out := make([]choice, 0, 5)
	for _, n := range []int{2, 3, 4, 5, 6} {
		n := n
		out = append(out, choice{
			Label: fmt.Sprintf("%dx/minggu", n),
			apply: func(a *tools.UpdateProfileArgs) { a.SessionsPerWeek = n },
		})
	}
	return out
}

func minutes() []choice {
	out := make([]choice, 0, 4)
	for _, n := range []int{30, 45, 60, 90} {
		n := n
		out = append(out, choice{
			Label: fmt.Sprintf("%d menit", n),
			apply: func(a *tools.UpdateProfileArgs) { a.SessionMinutes = n },
		})
	}
	return out
}

// equipmentSets are presets, not a multi-select.
//
// A tick-list of twenty-eight catalogue entries is a worse experience than typing, and
// almost everyone falls into one of these. Anything unusual is still describable in words.
func equipmentSets() []choice {
	presets := []struct {
		label string
		kit   []string
	}{
		{"Bodyweight aja", []string{"body weight"}},
		{"Dumbbell di rumah", []string{"body weight", "dumbbell"}},
		{"Dumbbell + treadmill", []string{"body weight", "dumbbell", "treadmill"}},
		{"Gym lengkap", []string{"body weight", "dumbbell", "barbell", "cable",
			"leverage machine", "smith machine", "kettlebell"}},
	}
	out := make([]choice, 0, len(presets))
	for _, p := range presets {
		kit := p.kit
		out = append(out, choice{Label: p.label, apply: func(a *tools.UpdateProfileArgs) {
			a.Equipment = kit
		}})
	}
	return out
}
