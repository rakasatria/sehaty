package tools

import (
	"fmt"
	"strings"
)

type FoodNameEntry struct {
	Code   string `json:"code" jsonschema:"the TKPI code, e.g. CP077"`
	NameEN string `json:"name_en" jsonschema:"the common English name. Send an EMPTY string when the food has no English equivalent (oncom, gembus) — that is a real answer and stops it being asked again"`
}

type NameFoodsArgs struct {
	Entries []FoodNameEntry `json:"entries,omitempty" jsonschema:"English names to record. Omit to just fetch the next batch needing names"`
	Limit   int             `json:"limit,omitempty" jsonschema:"how many unnamed foods to return, default 25"`
	Source  string          `json:"source,omitempty" jsonschema:"agent (a translation) or user (a human correction); defaults to agent"`
}

type UnnamedFood struct {
	Code  string `json:"code"`
	Name  string `json:"name_id"`
	Group string `json:"group,omitempty"`
}

type NameFoodsOut struct {
	Saved        int           `json:"saved"`
	UnknownCodes []string      `json:"unknown_codes,omitempty"`
	NamedTotal   int           `json:"named_total"`
	Remaining    int           `json:"remaining"`
	Next         []UnnamedFood `json:"next"`
	Note         string        `json:"note,omitempty"`
}

// NameFoods records English names for TKPI foods and hands back the next batch to name.
//
// TKPI carries an English gloss for only a handful of its 1,142 foods, so searching in
// English mostly fails. Rather than run a separate translation pipeline, the assistant
// already reading these names supplies them, and Sehaty remembers.
//
// This is SEARCH METADATA. It never touches a nutrition value — a wrong name costs a
// ranking, not a wrong number in someone's log. That is why a model is trusted with it
// and is not trusted to estimate macros.
func NameFoods(d Deps, a NameFoodsArgs) (NameFoodsOut, error) {
	if d.Food == nil {
		return NameFoodsOut{}, fmt.Errorf("no food table loaded")
	}
	source := strings.ToLower(strings.TrimSpace(a.Source))
	if source != "user" {
		source = "agent"
	}

	out := NameFoodsOut{}
	for _, e := range a.Entries {
		code := strings.ToUpper(strings.TrimSpace(e.Code))
		if code == "" {
			continue
		}
		if _, ok := d.Food.ByCode(code); !ok {
			out.UnknownCodes = append(out.UnknownCodes, code)
			continue
		}
		if err := d.DB.PutFoodAlias(code, e.NameEN, source); err != nil {
			return out, err
		}
		d.Food.SetAlias(code, e.NameEN, source)
		out.Saved++
	}

	limit := a.Limit
	if limit <= 0 {
		limit = 25
	}
	next := d.Food.Unnamed(limit)
	out.Next = make([]UnnamedFood, 0, len(next))
	for _, f := range next {
		out.Next = append(out.Next, UnnamedFood{Code: f.Code, Name: f.NameID, Group: f.Group})
	}
	out.NamedTotal = d.Food.NamedCount()
	out.Remaining = d.Food.Count() - out.NamedTotal
	if out.Remaining == 0 {
		out.Note = "Every food has been considered. Nothing left to name."
	}
	return out, nil
}

// LoadFoodAliases applies stored English names to the in-memory table at startup.
func LoadFoodAliases(d Deps) (int, error) {
	if d.Food == nil {
		return 0, nil
	}
	aliases, err := d.DB.FoodAliases()
	if err != nil {
		return 0, err
	}
	n := 0
	for code, a := range aliases {
		if d.Food.SetAlias(code, a.NameEN, a.Source) {
			n++
		}
	}
	return n, nil
}
