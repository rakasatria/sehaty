// Package catalog loads the exercise dataset and answers "what can this person actually
// do?".
//
// The upstream dataset has 1,324 exercises and NO difficulty field, so ratings live here
// in difficulty.json. Without them the planner cheerfully prescribes pistol squats and
// korean dips to beginners — which the Python prototype did, on its first run.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

//go:embed difficulty.json
var difficultyJSON []byte

// unratedDifficulty is what an exercise scores when difficulty.json has no entry for it.
// Deliberately high: an unknown movement should be EXCLUDED from a beginner's plan, not
// included by default. With 1,324 exercises and ratings arriving incrementally, failing
// safe matters more than coverage.
const unratedDifficulty = 4

type Exercise struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	BodyPart   string `json:"body_part"`
	Equipment  string `json:"equipment"`
	Target     string `json:"target"`
	Difficulty int    `json:"-"`
}

type Catalog struct {
	all   []Exercise
	byKey map[string]Exercise
	rated int
}

func Load(path string) (*Catalog, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read exercises %s: %w "+
			"(run: git submodule update --init --depth 1)", path, err)
	}
	var list []Exercise
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("parse exercises: %w", err)
	}
	ratings := map[string]int{}
	if err := json.Unmarshal(difficultyJSON, &ratings); err != nil {
		return nil, fmt.Errorf("parse difficulty.json: %w", err)
	}
	c := &Catalog{byKey: make(map[string]Exercise, len(list))}
	for _, e := range list {
		if d, ok := ratings[strings.ToLower(e.Name)]; ok {
			e.Difficulty = d
			c.rated++
		} else {
			e.Difficulty = unratedDifficulty
		}
		c.all = append(c.all, e)
		c.byKey[strings.ToLower(e.Name)] = e
	}
	return c, nil
}

func (c *Catalog) Count() int { return len(c.all) }

// Rated reports how many exercises have a real difficulty rating rather than the
// fail-safe default. Surfaced so the gap is visible instead of silently assumed away.
func (c *Catalog) Rated() int { return c.rated }

func (c *Catalog) ByName(name string) (Exercise, bool) {
	e, ok := c.byKey[strings.ToLower(name)]
	return e, ok
}

// For returns only what this person can perform. Equipment is filtered before anything
// else and there is no override at prescribe time — a plan someone cannot do is worse
// than no plan.
func (c *Catalog) For(equipment []string, maxDifficulty int) []Exercise {
	have := make(map[string]bool, len(equipment))
	for _, eq := range equipment {
		have[strings.ToLower(strings.TrimSpace(eq))] = true
	}
	var out []Exercise
	for _, e := range c.all {
		if !have[strings.ToLower(e.Equipment)] {
			continue
		}
		if maxDifficulty > 0 && e.Difficulty > maxDifficulty {
			continue
		}
		out = append(out, e)
	}
	return out
}
