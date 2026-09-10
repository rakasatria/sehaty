package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/rakasatria/sehaty/internal/food"
	"github.com/rakasatria/sehaty/internal/media"
	"github.com/rakasatria/sehaty/internal/storage"
)

type FindFoodsArgs struct {
	Query string `json:"query" jsonschema:"food name in Bahasa Indonesia (English works for some foods only), e.g. beras giling, tempe, ikan bakar"`
	Limit int    `json:"limit,omitempty" jsonschema:"defaults to 10"`
}

type FoodHit struct {
	Code           string      `json:"code"`
	Name           string      `json:"name"`
	NameEN         string      `json:"name_en,omitempty"`
	Group          string      `json:"group,omitempty"`
	Per100g        food.Macros `json:"per_100g"`
	SourceCitation string      `json:"source_citation"`
	Flags          []string    `json:"verification_flags,omitempty"`
}

type FoodsOut struct {
	Count int       `json:"count"`
	Foods []FoodHit `json:"foods"`
	Note  string    `json:"note,omitempty"`
}

type LogFoodArgs struct {
	Profile        string  `json:"profile"`
	Food           string  `json:"food" jsonschema:"a TKPI code such as AR001, or an exact food name from find_foods"`
	Grams          float64 `json:"grams" jsonschema:"portion in grams of edible food"`
	Meal           string  `json:"meal,omitempty" jsonschema:"sarapan, makan siang, makan malam, snack"`
	Date           string  `json:"date,omitempty" jsonschema:"YYYY-MM-DD; defaults to today"`
	Photo          string  `json:"photo,omitempty" jsonschema:"content hash from attach_media, to attach a meal photo to this entry"`
	AllowDuplicate bool    `json:"allow_duplicate,omitempty" jsonschema:"set true only when the same food really was eaten twice; otherwise an identical repeat is treated as a retry"`
}

type LogFoodOut struct {
	Status         string   `json:"status"` // logged | duplicate
	Profile        string   `json:"profile"`
	Date           string   `json:"date"`
	Meal           string   `json:"meal,omitempty"`
	Item           string   `json:"item"`
	Grams          float64  `json:"grams"`
	Kcal           float64  `json:"kcal"`
	ProteinG       float64  `json:"protein_g"`
	CarbsG         float64  `json:"carbs_g"`
	FatG           float64  `json:"fat_g"`
	Source         string   `json:"source"`
	Photo          string   `json:"photo,omitempty"`
	SourceCitation string   `json:"source_citation"`
	Flags          []string `json:"verification_flags,omitempty"`
	Note           string   `json:"note,omitempty"`
}

func hit(f food.Food) FoodHit {
	name := f.NameFull
	if name == "" {
		name = f.NameID
	}
	return FoodHit{Code: f.Code, Name: name, NameEN: f.NameEN, Group: f.Group, Per100g: f.Macros(100),
		SourceCitation: f.SourceCitation, Flags: f.Flags()}
}

func FindFoods(d Deps, a FindFoodsArgs) (FoodsOut, error) {
	if d.Food == nil {
		return FoodsOut{}, fmt.Errorf("no food table loaded")
	}
	limit := a.Limit
	if limit <= 0 {
		limit = 10
	}
	found, exact := d.Food.Search(a.Query, limit)
	// An empty result must serialise as [] rather than null: a client should not have
	// to distinguish "no matches" from "field missing".
	out := FoodsOut{Count: len(found), Foods: []FoodHit{}}
	for _, f := range found {
		out.Foods = append(out.Foods, hit(f))
	}
	switch {
	case len(found) == 0:
		out.Note = fmt.Sprintf("Nothing in the Indonesian food table (TKPI 2020) matches %q. "+
			"It lists ingredients, not composite dishes — nasi goreng and gado-gado are not "+
			"in it and must be logged as their parts.", a.Query)
	case !exact:
		out.Note = fmt.Sprintf("No entry matches all of %q, so these match part of it — "+
			"the table qualifies food differently from the way people speak. Ask which one "+
			"they mean before logging; do not choose.", a.Query)
	}
	return out, nil
}

// resolve turns what the caller typed into exactly one food, or refuses.
//
// It never picks among several candidates. Choosing for the caller is how the wrong number
// ends up in someone's health record looking entirely deliberate.
func resolve(t *food.Table, q string) (food.Food, error) {
	if f, ok := t.ByCode(q); ok {
		return f, nil
	}
	found, exact := t.Search(q, 6)
	switch {
	// A loosened match is never resolved automatically, not even when it is the only one.
	// "Nasi putih" narrowing to "Nasi" is almost certainly right, and "almost certainly"
	// is not the standard for writing a number into someone's health record.
	case len(found) == 1 && exact:
		return found[0], nil
	case len(found) == 0:
		return food.Food{}, fmt.Errorf("no food in TKPI 2020 matches %q; use find_foods to "+
			"search, and note the table lists ingredients rather than composite dishes", q)
	default:
		var b strings.Builder
		if exact {
			fmt.Fprintf(&b, "%q matches %d foods — pass one of these codes instead: ", q, len(found))
		} else {
			fmt.Fprintf(&b, "nothing matches all of %q; these match part of it, so ask which "+
				"is meant and pass its code: ", q)
		}
		for i, f := range found {
			if i > 0 {
				b.WriteString("; ")
			}
			name := f.NameID
			if len(name) > 44 {
				name = name[:44]
			}
			fmt.Fprintf(&b, "%s (%s)", f.Code, name)
		}
		return food.Food{}, fmt.Errorf("%s", b.String())
	}
}

// dedupeWindow finds an identical entry already logged on the same day.
//
// Sehaty stores food at DAY granularity, so this cannot be a strict clock window. Same day,
// same meal, same item, same portion is treated as a retry — eating precisely the same
// weight of the same food twice in one meal slot is far rarer than a client re-issuing a
// request whose response stream broke, which the current MCP spec requires it to do.
func dedupeWindow(d Deps, profileID, date, meal, item string, grams float64) (*storage.FoodEntry, error) {
	recent, err := d.DB.Foods(profileID, 2)
	if err != nil {
		return nil, err
	}
	for i := range recent {
		e := recent[i]
		if e.Date == date && strings.EqualFold(e.Meal, meal) &&
			strings.EqualFold(e.Item, item) && sameGrams(e.Grams, grams) {
			return &e, nil
		}
	}
	return nil, nil
}

func sameGrams(a, b float64) bool { return a-b < 0.01 && b-a < 0.01 }

func LogFood(d Deps, a LogFoodArgs) (LogFoodOut, error) {
	if d.Food == nil {
		return LogFoodOut{}, fmt.Errorf("no food table loaded")
	}
	if _, err := requireProfile(d, a.Profile); err != nil {
		return LogFoodOut{}, err
	}
	if a.Grams <= 0 {
		return LogFoodOut{}, fmt.Errorf("grams must be greater than zero, got %v", a.Grams)
	}
	f, err := resolve(d.Food, a.Food)
	if err != nil {
		return LogFoodOut{}, err
	}

	date := a.Date
	if date == "" {
		date = today()
	}
	m := f.Macros(a.Grams)
	item := f.NameID

	if !a.AllowDuplicate {
		if dup, err := dedupeWindow(d, a.Profile, date, a.Meal, item, a.Grams); err != nil {
			return LogFoodOut{}, err
		} else if dup != nil {
			// Echo the photo too: a caller must get the same response SHAPE whether the
			// entry was just created or already existed, or it cannot tell from the
			// reply whether its photo made it onto the meal.
			return LogFoodOut{Status: "duplicate", Profile: a.Profile, Date: dup.Date,
				Meal: dup.Meal, Item: dup.Item, Grams: dup.Grams, Kcal: dup.Kcal,
				ProteinG: dup.ProteinG, CarbsG: dup.CarbsG, FatG: dup.FatG,
				Source: dup.Source, Photo: dup.PhotoHash,
				SourceCitation: f.SourceCitation, Flags: f.Flags(),
				Note: "Already logged today — treated as a repeat of the same entry, not " +
					"logged again. Pass allow_duplicate if it really was eaten twice."}, nil
		}
	}

	// A photo must actually exist before it is recorded against a meal — a dangling hash
	// would look like evidence and resolve to nothing.
	if a.Photo != "" && d.Media != nil {
		ok, err := d.Media.Exists(context.Background(), a.Profile, media.KindPhoto, a.Photo)
		if err != nil {
			return LogFoodOut{}, err
		}
		if !ok {
			return LogFoodOut{}, fmt.Errorf("no photo %q stored for this profile; "+
				"call attach_media first and use the hash it returns", a.Photo)
		}
	}

	source := "tkpi:" + f.Code
	err = d.DB.LogFood(a.Profile, storage.FoodEntry{
		Date: date, Meal: a.Meal, Item: item, Grams: a.Grams,
		Kcal: m.Kcal, ProteinG: m.ProteinG, CarbsG: m.CarbsG, FatG: m.FatG,
		Source: source, PhotoHash: a.Photo,
	})
	if err != nil {
		return LogFoodOut{}, err
	}

	out := LogFoodOut{Status: "logged", Profile: a.Profile, Date: date, Meal: a.Meal,
		Item: item, Grams: a.Grams, Kcal: m.Kcal, ProteinG: m.ProteinG, CarbsG: m.CarbsG,
		FatG: m.FatG, Source: source, Photo: a.Photo,
		SourceCitation: f.SourceCitation, Flags: f.Flags()}
	if len(out.Flags) > 0 {
		out.Note = "This food's values could not be confirmed across sources — treat the " +
			"numbers as uncertain rather than measured."
	}
	return out, nil
}
