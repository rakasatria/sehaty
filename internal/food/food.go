// Package food is the Indonesian food composition table.
//
// Values come from TKPI 2020 (Kementerian Kesehatan RI), parsed from the book and
// cross-checked against two other sources — see data/tkpi/README.md. Every food carries
// its original SOURCE CITATION and any VERIFICATION FLAG raised during that check.
//
// The flags are the point. A nutrition number whose provenance is unknown, or which two
// sources disagree about, must announce itself rather than be presented as measured fact.
// 83 of 1,142 rows are flagged; the rest agree across sources.
//
// WHAT THIS TABLE IS NOT. TKPI is a composition table of INGREDIENTS. It has tempe, tahu,
// beras and ikan in depth, exactly one beverage, and no composite dishes — nasi goreng and
// gado-gado are not in here and must be composed from their ingredients.
package food

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"unicode"
)

type Nutrient struct {
	Value float64 `json:"value"`
	Unit  string  `json:"unit"`
}

type verification struct {
	Flags []string `json:"flags"`
	Clean bool     `json:"clean"`
}

type Food struct {
	Code           string              `json:"code"`
	NameID         string              `json:"name_id"`
	NameFull       string              `json:"name_full"`
	Group          string              `json:"group"`
	Type           string              `json:"type"`
	SourceCitation string              `json:"source_citation"`
	Per            string              `json:"per"`
	Nutrients      map[string]Nutrient `json:"nutrients"`
	Verification   verification        `json:"verification"`

	haystack string // lowercased searchable text, built at load
}

// Verified reports whether every cross-source check agreed on this row.
func (f Food) Verified() bool { return f.Verification.Clean }

// Flags lists what disagreed. Callers must surface these rather than swallow them.
func (f Food) Flags() []string { return f.Verification.Flags }

// Macros is a portion's worth of energy and macronutrients.
type Macros struct {
	Grams    float64 `json:"grams"`
	Kcal     float64 `json:"kcal"`
	ProteinG float64 `json:"protein_g"`
	CarbsG   float64 `json:"carbs_g"`
	FatG     float64 `json:"fat_g"`
	FibreG   float64 `json:"fibre_g"`
}

// Macros scales the per-100g values to an actual portion.
//
// A zero or negative portion yields zero rather than an error: it is not a nutrition
// question, and inventing a number for it would be the one thing this package must not do.
func (f Food) Macros(grams float64) Macros {
	if grams <= 0 {
		return Macros{}
	}
	k := grams / 100
	get := func(name string) float64 { return round2(f.Nutrients[name].Value * k) }
	return Macros{
		Grams:    grams,
		Kcal:     get("energy_kcal"),
		ProteinG: get("protein_g"),
		CarbsG:   get("carbohydrate_g"),
		FatG:     get("fat_g"),
		FibreG:   get("fibre_g"),
	}
}

func round2(v float64) float64 { return float64(int(v*100+0.5)) / 100 }

type Table struct {
	all     []Food
	byCode  map[string]Food
	flagged int
}

type document struct {
	Dataset   string `json:"dataset"`
	Publisher string `json:"publisher"`
	Count     int    `json:"count"`
	Foods     []Food `json:"foods"`
}

func Load(path string) (*Table, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read food table %s: %w", path, err)
	}
	var doc document
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse food table: %w", err)
	}
	t := &Table{byCode: make(map[string]Food, len(doc.Foods))}
	for _, f := range doc.Foods {
		// Both names are searched: TKPI carries the Indonesian name and an English gloss,
		// and a person logging in Bahasa or in English should find the same row.
		f.haystack = strings.ToLower(f.NameID + " " + f.NameFull + " " + f.Group)
		t.all = append(t.all, f)
		t.byCode[strings.ToUpper(f.Code)] = f
		if !f.Verified() {
			t.flagged++
		}
	}
	if len(t.all) == 0 {
		return nil, fmt.Errorf("food table %s contains no foods", path)
	}
	return t, nil
}

func (t *Table) Count() int   { return len(t.all) }
func (t *Table) Flagged() int { return t.flagged }
func (t *Table) All() []Food  { return t.all }

func (t *Table) ByCode(code string) (Food, bool) {
	f, ok := t.byCode[strings.ToUpper(strings.TrimSpace(code))]
	return f, ok
}

// tokenize splits a query into lowercase word tokens, dropping punctuation. TKPI names are
// comma-heavy ("Beras giling, mentah (Rice, raw)"), so matching on raw substrings misses
// obvious hits.
func tokenize(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	out := fields[:0]
	for _, f := range fields {
		if len(f) > 1 {
			out = append(out, f)
		}
	}
	return out
}

// Search returns foods matching every token in the query, best first.
//
// EVERY token must match. A query is an attempt to name one food, so returning rows that
// match only part of it hands the caller a plausible wrong answer — which for nutrition
// data is worse than returning nothing.
func (t *Table) Search(query string, limit int) []Food {
	tokens := tokenize(query)
	if len(tokens) == 0 {
		return nil
	}
	type hit struct {
		f     Food
		score int
	}
	var hits []hit
	for _, f := range t.all {
		score, ok := 0, true
		for _, tok := range tokens {
			if !strings.Contains(f.haystack, tok) {
				ok = false
				break
			}
			// A token starting a word beats one buried mid-word.
			if strings.HasPrefix(f.haystack, tok) || strings.Contains(f.haystack, " "+tok) {
				score += 3
			} else {
				score++
			}
		}
		if !ok {
			continue
		}
		// A food whose name BEGINS with the query is far more likely to be what was
		// meant than one that merely mentions it. Without this, "tempe" ranked
		// "Keripik tempe" — fried chips at 581 kcal — above tempe itself at 150-201.
		if strings.HasPrefix(strings.ToLower(f.NameID), tokens[0]) {
			score += 10
		}
		// Shorter names are more likely to be the plain form of the food rather than a
		// heavily qualified variant.
		score = score*100 - len(f.NameID)
		if f.Verified() {
			score += 20 // prefer a row every source agreed on
		}
		hits = append(hits, hit{f, score})
	}
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].score != hits[j].score {
			return hits[i].score > hits[j].score
		}
		return hits[i].f.Code < hits[j].f.Code
	})
	if limit <= 0 || limit > len(hits) {
		limit = len(hits)
	}
	out := make([]Food, 0, limit)
	for _, h := range hits[:limit] {
		out = append(out, h.f)
	}
	return out
}
