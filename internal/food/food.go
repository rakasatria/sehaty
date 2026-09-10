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
	"github.com/blevesearch/bleve/v2"
	"os"
	"strings"
	"sync"
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

	// NameEN is an English name supplied later by the assistant, not by TKPI. Empty
	// means either "not yet named" or "has no English name" — Named() tells them apart.
	NameEN     string `json:"name_en,omitempty"`
	NameENFrom string `json:"name_en_source,omitempty"`
	named      bool
}

// Named reports whether this food has been considered for an English name. A food can be
// named and still have an empty NameEN — that records "no common English name exists",
// which is a real answer and stops it being re-asked forever.
func (f Food) Named() bool { return f.named }

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
	mu      sync.RWMutex
	all     []Food
	byCode  map[string]Food
	flagged int
	index   bleve.Index
}

// SetAlias attaches an English name to a food and rebuilds its search text.
//
// nameEN may be empty: that records "this food has no common English name". Safe for
// concurrent use, because MCP requests arrive in parallel while searches are running.
func (t *Table) SetAlias(code, nameEN, source string) bool {
	code = strings.ToUpper(strings.TrimSpace(code))
	t.mu.Lock()
	defer t.mu.Unlock()
	for i := range t.all {
		if strings.ToUpper(t.all[i].Code) != code {
			continue
		}
		t.all[i].NameEN = strings.TrimSpace(nameEN)
		t.all[i].NameENFrom = source
		t.all[i].named = true
		t.byCode[code] = t.all[i]
		// Reindex so the new English name is searchable immediately, not after a restart.
		if t.index != nil {
			_ = t.index.Index(code, docFor(t.all[i]))
		}
		return true
	}
	return false
}

// Unnamed returns foods that have not yet been considered for an English name, so the
// assistant can work through the table in batches rather than all 1,142 at once.
func (t *Table) Unnamed(limit int) []Food {
	t.mu.RLock()
	defer t.mu.RUnlock()
	var out []Food
	for _, f := range t.all {
		if f.named {
			continue
		}
		out = append(out, f)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// NamedCount reports how many foods have been considered.
func (t *Table) NamedCount() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	n := 0
	for _, f := range t.all {
		if f.named {
			n++
		}
	}
	return n
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
		t.all = append(t.all, f)
		t.byCode[strings.ToUpper(f.Code)] = f
		if !f.Verified() {
			t.flagged++
		}
	}
	if len(t.all) == 0 {
		return nil, fmt.Errorf("food table %s contains no foods", path)
	}
	idx, err := newIndex()
	if err != nil {
		return nil, err
	}
	batch := idx.NewBatch()
	for _, f := range t.all {
		if err := batch.Index(strings.ToUpper(f.Code), docFor(f)); err != nil {
			return nil, fmt.Errorf("index %s: %w", f.Code, err)
		}
	}
	if err := idx.Batch(batch); err != nil {
		return nil, fmt.Errorf("build index: %w", err)
	}
	t.index = idx
	return t, nil
}

func (t *Table) Count() int   { return len(t.all) }
func (t *Table) Flagged() int { return t.flagged }

func (t *Table) All() []Food {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]Food, len(t.all))
	copy(out, t.all)
	return out
}

func (t *Table) ByCode(code string) (Food, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	f, ok := t.byCode[strings.ToUpper(strings.TrimSpace(code))]
	return f, ok
}
