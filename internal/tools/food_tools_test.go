package tools

import (
	"math"
	"strings"
	"testing"

	"github.com/rakasatria/sehaty/internal/storage"
)

func foodDeps(t *testing.T) Deps {
	t.Helper()
	d := testDeps(t)
	if err := d.DB.SaveProfile(storage.Profile{ID: "raka", Equipment: []string{"body weight"},
		Goal: "fat_loss", SessionsPerWeek: 3, MaxDifficulty: 5}); err != nil {
		t.Fatal(err)
	}
	return d
}

func TestLogFoodByCodeScalesFromPer100g(t *testing.T) {
	d := foodDeps(t)
	out, err := LogFood(d, LogFoodArgs{Profile: "raka", Food: "AR001", Grams: 200, Meal: "sarapan"})
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(out.Kcal-714) > 0.5 {
		t.Errorf("200 g of AR001 logged as %v kcal, want 714", out.Kcal)
	}
	if out.Status != "logged" {
		t.Errorf("status = %q, want logged", out.Status)
	}
	// Provenance must ride along: a stored number with no source cannot be audited later.
	if !strings.Contains(out.Source, "AR001") {
		t.Errorf("source = %q, does not identify the food row", out.Source)
	}
	if out.SourceCitation == "" {
		t.Error("no source citation recorded")
	}

	got, err := d.DB.Foods("raka", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("stored %d food entries, want 1", len(got))
	}
	if math.Abs(got[0].Kcal-714) > 0.5 {
		t.Errorf("persisted kcal = %v, want 714", got[0].Kcal)
	}
}

// Guessing is the failure mode this whole design exists to prevent.
func TestLogFoodRefusesAnUnknownFood(t *testing.T) {
	d := foodDeps(t)
	if _, err := LogFood(d, LogFoodArgs{Profile: "raka", Food: "zzzqqnotafood", Grams: 100}); err == nil {
		t.Fatal("logged a food that is not in the table")
	}
}

// An ambiguous name must come back with the candidates, not with a silent pick.
func TestLogFoodRefusesAmbiguityAndNamesTheCandidates(t *testing.T) {
	d := foodDeps(t)
	_, err := LogFood(d, LogFoodArgs{Profile: "raka", Food: "tempe", Grams: 100})
	if err == nil {
		t.Fatal("picked one of many 'tempe' rows without asking")
	}
	if !strings.Contains(err.Error(), "CP") {
		t.Errorf("error does not list candidate codes, so the caller cannot resolve it: %v", err)
	}
}

// The 2026-07-28 spec makes a client re-issue a request whose stream broke, so a committed
// meal WILL be sent twice. Without this the food log silently doubles.
func TestLogFoodDedupesAnIdenticalRetry(t *testing.T) {
	d := foodDeps(t)
	a := LogFoodArgs{Profile: "raka", Food: "AR001", Grams: 150, Meal: "makan siang"}
	first, err := LogFood(d, a)
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != "logged" {
		t.Fatalf("first call status = %q", first.Status)
	}
	second, err := LogFood(d, a)
	if err != nil {
		t.Fatal(err)
	}
	if second.Status != "duplicate" {
		t.Errorf("retry status = %q, want duplicate", second.Status)
	}
	got, _ := d.DB.Foods("raka", 2)
	if len(got) != 1 {
		t.Fatalf("stored %d entries after a retry, want 1", len(got))
	}
}

// ...but someone really can eat the same thing twice.
func TestLogFoodAllowsAnIntentionalDuplicate(t *testing.T) {
	d := foodDeps(t)
	a := LogFoodArgs{Profile: "raka", Food: "AR001", Grams: 150, Meal: "makan siang"}
	if _, err := LogFood(d, a); err != nil {
		t.Fatal(err)
	}
	a.AllowDuplicate = true
	out, err := LogFood(d, a)
	if err != nil {
		t.Fatal(err)
	}
	if out.Status != "logged" {
		t.Errorf("status = %q, want logged when the caller insists", out.Status)
	}
	got, _ := d.DB.Foods("raka", 2)
	if len(got) != 2 {
		t.Fatalf("stored %d entries, want 2", len(got))
	}
}

// A row two sources disagreed about must say so at the point of use.
func TestLogFoodSurfacesVerificationFlags(t *testing.T) {
	d := foodDeps(t)
	var flaggedCode string
	for _, f := range d.Food.All() {
		if !f.Verified() {
			flaggedCode = f.Code
			break
		}
	}
	if flaggedCode == "" {
		t.Skip("no flagged foods in the table")
	}
	out, err := LogFood(d, LogFoodArgs{Profile: "raka", Food: flaggedCode, Grams: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Flags) == 0 {
		t.Errorf("%s is unverified but logging it reported no flags", flaggedCode)
	}
}

func TestLogFoodRejectsNonPositiveGrams(t *testing.T) {
	d := foodDeps(t)
	for _, g := range []float64{0, -5} {
		if _, err := LogFood(d, LogFoodArgs{Profile: "raka", Food: "AR001", Grams: g}); err == nil {
			t.Errorf("accepted a portion of %v g", g)
		}
	}
}

func TestFindFoodsReturnsProvenanceAndPer100g(t *testing.T) {
	d := foodDeps(t)
	out, err := FindFoods(d, FindFoodsArgs{Query: "beras giling", Limit: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Foods) == 0 {
		t.Fatal("no results for 'beras giling'")
	}
	f := out.Foods[0]
	if f.Code == "" || f.Name == "" {
		t.Error("result missing code or name")
	}
	if f.Per100g.Kcal <= 0 {
		t.Error("result carries no energy value")
	}
	if f.SourceCitation == "" {
		t.Error("result carries no source citation")
	}
}
