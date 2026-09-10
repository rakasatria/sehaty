package tools

import (
	"strings"
	"testing"
)

func TestNameFoodsMakesEnglishSearchWork(t *testing.T) {
	d := foodDeps(t)
	// "curd" is used deliberately: no Indonesian name in the table is within one edit of
	// it, so fuzzy matching cannot reach it. ("tempeh" would be a bad probe — bleve
	// matches it to "tempe" one edit away, so English partly works there by accident.)
	const probe = "curd"
	if got := d.Food.Search(probe, 5); len(got) != 0 {
		t.Fatalf("%q already matched %d foods before naming", probe, len(got))
	}
	out, err := NameFoods(d, NameFoodsArgs{Entries: []FoodNameEntry{
		{Code: "CP061", NameEN: "bean curd (tofu), raw"}}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Saved != 1 {
		t.Fatalf("saved %d", out.Saved)
	}
	got := d.Food.Search(probe, 5)
	if len(got) == 0 {
		t.Fatalf("%q still finds nothing after naming CP061", probe)
	}
	if got[0].Code != "CP061" {
		t.Errorf("top hit is %s, want CP061", got[0].Code)
	}
}

// An empty name is a real answer: this food has no English equivalent.
func TestNameFoodsRecordsAbsenceOfAnEnglishName(t *testing.T) {
	d := foodDeps(t)
	before := d.Food.NamedCount()
	out, err := NameFoods(d, NameFoodsArgs{Entries: []FoodNameEntry{{Code: "CP071", NameEN: ""}}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Saved != 1 {
		t.Fatalf("an explicit 'no English name' was not saved")
	}
	if d.Food.NamedCount() != before+1 {
		t.Error("food not counted as considered, so it will be asked about forever")
	}
	for _, f := range out.Next {
		if f.Code == "CP071" {
			t.Error("CP071 came back in the next batch despite being answered")
		}
	}
}

func TestNameFoodsReportsUnknownCodes(t *testing.T) {
	d := foodDeps(t)
	out, err := NameFoods(d, NameFoodsArgs{Entries: []FoodNameEntry{
		{Code: "ZZ999", NameEN: "nonsense"}, {Code: "CP077", NameEN: "tempeh"}}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Saved != 1 {
		t.Errorf("saved %d, want 1 (the valid one)", out.Saved)
	}
	if len(out.UnknownCodes) != 1 || out.UnknownCodes[0] != "ZZ999" {
		t.Errorf("unknown codes = %v, want [ZZ999]", out.UnknownCodes)
	}
}

func TestNameFoodsHandsBackWorkWhenCalledEmpty(t *testing.T) {
	d := foodDeps(t)
	out, err := NameFoods(d, NameFoodsArgs{Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Next) != 5 {
		t.Fatalf("returned %d foods to name, want 5", len(out.Next))
	}
	if out.Remaining < 1000 {
		t.Errorf("remaining = %d; the whole table starts unnamed", out.Remaining)
	}
}

// Names must survive a restart, or every session re-does the work.
func TestNameFoodsPersistsAcrossReload(t *testing.T) {
	d := foodDeps(t)
	if _, err := NameFoods(d, NameFoodsArgs{Entries: []FoodNameEntry{
		{Code: "CP077", NameEN: "soybean curd cake"}}, Source: "user"}); err != nil {
		t.Fatal(err)
	}
	// A fresh table, as a restart would produce.
	fresh := foodDeps(t)
	fresh.DB = d.DB
	n, err := LoadFoodAliases(fresh)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("reapplied %d aliases, want 1", n)
	}
	got := fresh.Food.Search("curd", 3)
	if len(got) == 0 || got[0].Code != "CP077" {
		t.Error("alias did not survive the reload")
	}
	f, _ := fresh.Food.ByCode("CP077")
	if f.NameENFrom != "user" {
		t.Errorf("provenance lost on reload: %q", f.NameENFrom)
	}
	if !strings.Contains(strings.ToLower(f.NameEN), "curd") {
		t.Errorf("name lost on reload: %q", f.NameEN)
	}
}
