// Command gendifficulty rates every exercise in the dataset from 1 (anyone) to 5 (elite),
// writing internal/catalog/difficulty.json.
//
// WHY THIS EXISTS. The upstream dataset has 1,324 exercises and no difficulty field. The
// catalog scores anything unrated as 4, deliberately, so that an unknown movement is
// excluded from a beginner's plan rather than prescribed to them. That fail-safe is right,
// but with only a handful rated it excluded almost everything: a real profile could see ONE
// exercise out of 1,324.
//
// WHAT THIS IS NOT. These are HEURISTIC ratings derived from equipment and name, not expert
// assessment. They are reproducible and reviewable — run this again and you get the same
// file — and they are wrong in individual cases. overrides.json exists for exactly that:
// hand-corrected values win over anything computed here.
//
// The bias is deliberately toward CAUTION. A movement that looks technical scores high; the
// cost of over-rating is a missing exercise, the cost of under-rating is someone attempting
// a pistol squat in week one.
//
//	go run ./cmd/gendifficulty
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

const (
	datasetPath   = "third_party/exercises-dataset/data/exercises.json"
	overridesPath = "internal/catalog/overrides.json"
	outputPath    = "internal/catalog/difficulty.json"
)

type exercise struct {
	Name      string `json:"name"`
	Equipment string `json:"equipment"`
	BodyPart  string `json:"body_part"`
}

// equipmentBase is the starting score before the name is considered.
//
// The ordering logic: machines and cables score low because the path is guided and the
// failure mode is dropping the handle. Free weights score higher because the lifter owns
// the path. Anything implying added load or an odd object scores higher still.
var equipmentBase = map[string]int{
	"assisted":             1, // the whole point of assistance is to make it accessible
	"stationary bike":      1,
	"elliptical machine":   1,
	"stepmill machine":     1,
	"upper body ergometer": 1,
	"skierg machine":       2,
	"band":                 2,
	"resistance band":      2,
	"cable":                2,
	"leverage machine":     2,
	"smith machine":        2,
	"sled machine":         2,
	"medicine ball":        2,
	"body weight":          2,
	"roller":               3,
	"rope":                 3,
	"dumbbell":             3,
	"kettlebell":           3,
	"barbell":              3,
	"ez barbell":           3,
	"stability ball":       3, // an unstable surface is a skill, not just a load
	"bosu ball":            3,
	"wheel roller":         4, // the ab wheel is genuinely hard and hurts backs
	"olympic barbell":      4,
	"trap bar":             4,
	"weighted":             4,
	"hammer":               4,
	"tire":                 4,
}

const defaultBase = 3 // unknown equipment: assume free-weight difficulty, not machine

// elite movements are capped to 5 outright. These are skills, not loads — no amount of
// being strong makes a muscle-up appropriate for someone's first month. Phrases are
// deliberately specific: "one arm" alone would wrongly promote a one-arm dumbbell curl.
var elite = mustCompile(
	`snatch`, `clean and jerk`, `\bjerk\b`, `muscle[- ]up`, `planche`,
	`front lever`, `back lever`, `human flag`, `iron cross`, `handstand`,
	`pistol`, `single leg squat`, `l[- ]sit`, `dragon flag`,
	`turkish get[- ]?up`, `overhead squat`, `windmill`,
	`one arm push[- ]?up`, `one arm pull[- ]?up`, `one arm chin[- ]?up`,
)

// advanced adds two: real skill or real spinal load, but learnable inside a year.
var advanced = mustCompile(
	`pull[- ]?up`, `chin[- ]?up`, `\bdips?\b`, `deadlift`, `overhead press`,
	`push press`, `thruster`, `burpee`, `box jump`, `jump squat`, `plyo`,
	`\bclean\b`, `front squat`, `good morning`, `\bhang\b`, `bulgarian`, `sprawl`,
)

// isolation subtracts one: single joint, seated or supported, low consequence if it fails.
var isolation = mustCompile(
	`curl`, `extension`, `raise`, `\bfl(y|ye)s?\b`, `crunch`, `sit[- ]?up`,
	`plank`, `bridge`, `shrug`, `calf`, `kickback`, `pullover`, `twist`,
	`rotation`, `march`, `\bhold\b`, `scapular`, `wrist`, `\bneck\b`,
	`adduction`, `abduction`, `leg lift`,
)

// stretches are 1 for everyone. Mobility work is where a beginner should start, and the
// isolation rule alone would not push them low enough.
var stretch = mustCompile(`stretch`, `mobility`)

func mustCompile(parts ...string) *regexp.Regexp {
	return regexp.MustCompile(`(?i)(` + strings.Join(parts, `|`) + `)`)
}

// rate scores one exercise. Exported logic lives here so the test can assert on it.
func rate(name, equipment string) int {
	n := strings.ToLower(name)

	if stretch.MatchString(n) {
		return 1
	}
	if elite.MatchString(n) {
		return 5
	}

	base, ok := equipmentBase[strings.ToLower(equipment)]
	if !ok {
		base = defaultBase
	}
	switch {
	case advanced.MatchString(n):
		base += 2
	case isolation.MatchString(n):
		base--
	}

	// Assistance CAPS difficulty rather than being outweighed by the movement it assists.
	// Without this an "assisted chest dip" scored 3, because the dip rule added two on top
	// of the assisted base — defeating the entire purpose of the assistance.
	if strings.Contains(n, "assisted") || strings.EqualFold(equipment, "assisted") {
		return clamp(base, 1, 2)
	}
	return clamp(base, 1, 5)
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func main() {
	raw, err := os.ReadFile(datasetPath)
	if err != nil {
		fatal("read dataset: %v\n  (run: git submodule update --init --depth 1)", err)
	}
	var list []exercise
	if err := json.Unmarshal(raw, &list); err != nil {
		fatal("parse dataset: %v", err)
	}

	out := make(map[string]int, len(list))
	for _, e := range list {
		out[strings.ToLower(e.Name)] = rate(e.Name, e.Equipment)
	}

	// Hand-corrected ratings win. This is the escape hatch for everything the heuristic
	// gets wrong, and the file the next person should edit rather than tuning regexes.
	applied := 0
	if ov, err := os.ReadFile(overridesPath); err == nil {
		var overrides map[string]int
		if err := json.Unmarshal(ov, &overrides); err != nil {
			fatal("parse overrides: %v", err)
		}
		for name, d := range overrides {
			out[strings.ToLower(name)] = clamp(d, 1, 5)
			applied++
		}
	}

	// Sorted keys so the committed file has a stable diff between runs.
	keys := make([]string, 0, len(out))
	for k := range out {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("{\n")
	for i, k := range keys {
		kj, _ := json.Marshal(k)
		comma := ","
		if i == len(keys)-1 {
			comma = ""
		}
		fmt.Fprintf(&b, "  %s: %d%s\n", kj, out[k], comma)
	}
	b.WriteString("}\n")
	if err := os.WriteFile(outputPath, []byte(b.String()), 0o644); err != nil {
		fatal("write %s: %v", outputPath, err)
	}

	dist := map[int]int{}
	for _, d := range out {
		dist[d]++
	}
	fmt.Printf("  rated %d exercises (%d hand-overridden) -> %s\n", len(out), applied, outputPath)
	for d := 1; d <= 5; d++ {
		fmt.Printf("    difficulty %d: %4d  %s\n", d, dist[d], strings.Repeat("#", dist[d]/12))
	}
}

func fatal(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "gendifficulty: "+format+"\n", a...)
	os.Exit(1)
}
