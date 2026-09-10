package food

import (
	"math"
	"strings"
	"testing"
)

const dataset = "../../data/tkpi/tkpi-2020.json"

func load(t *testing.T) *Table {
	t.Helper()
	tab, err := Load(dataset)
	if err != nil {
		t.Fatalf("load %s: %v", dataset, err)
	}
	return tab
}

func TestLoadsTheWholeTable(t *testing.T) {
	tab := load(t)
	if tab.Count() < 1100 {
		t.Fatalf("loaded %d foods, expected the full TKPI table", tab.Count())
	}
	// The flagged rows are the ones the cross-source check could not confirm. If this
	// ever reads 100%, the verification data has been lost somewhere in the pipeline.
	if tab.Flagged() == 0 {
		t.Fatal("no flagged rows — verification metadata did not survive loading")
	}
}

func TestByCodeReturnsBookValues(t *testing.T) {
	tab := load(t)
	f, ok := tab.ByCode("AR001")
	if !ok {
		t.Fatal("AR001 (beras giling) missing")
	}
	if got := f.Nutrients["energy_kcal"].Value; math.Abs(got-357) > 0.01 {
		t.Errorf("AR001 energy = %v, want 357 (the book value)", got)
	}
	if f.SourceCitation == "" {
		t.Error("no source citation — a nutrition number without provenance is unusable")
	}
}

// CP082 is the row the spreadsheet got wrong: it recorded calcium (517 mg) as
// carbohydrate. If this regresses, a bad transcription has been loaded instead of the book.
func TestTempePasarHasBookValuesNotSpreadsheetValues(t *testing.T) {
	tab := load(t)
	f, ok := tab.ByCode("CP082")
	if !ok {
		t.Fatal("CP082 missing")
	}
	if got := f.Nutrients["carbohydrate_g"].Value; math.Abs(got-9.1) > 0.01 {
		t.Errorf("CP082 carbohydrate = %v, want 9.1; 517 would mean the spreadsheet leaked in", got)
	}
	if got := f.Nutrients["calcium_mg"].Value; math.Abs(got-517) > 0.01 {
		t.Errorf("CP082 calcium = %v, want 517", got)
	}
}

func TestSearchFindsIndonesianNames(t *testing.T) {
	tab := load(t)
	for _, q := range []string{"beras giling", "tempe", "tahu", "telur", "ikan"} {
		if got, _ := tab.Search(q, 5); len(got) == 0 {
			t.Errorf("Search(%q) found nothing", q)
		}
	}
}

// English search works only where TKPI supplied a gloss, which is a MINORITY of rows —
// there is no "tofu" in the table at all, only "tahu". This test records the limitation
// so nobody later assumes the table is bilingual and builds an English-first UI on it.
func TestEnglishSearchIsPartialByDesign(t *testing.T) {
	tab := load(t)
	if got, _ := tab.Search("rice", 5); len(got) == 0 {
		t.Error(`Search("rice") found nothing; some rows do carry an English gloss`)
	}
	// Note: fuzziness means some English terms DO match by accident — "tempeh" reaches
	// "tempe" one edit away. That is harmless and useful. "tofu" is two edits from "tahu",
	// so it stays out of reach until an alias is set.
	if got, _ := tab.Search("tofu", 5); len(got) != 0 {
		t.Errorf(`Search("tofu") returned %d results — the table has no English for tahu, `+
			`so a hit here means matching has become too loose`, len(got))
	}
}

func TestSearchIsRankedNotArbitrary(t *testing.T) {
	tab := load(t)
	got, _ := tab.Search("tempe", 5)
	if len(got) == 0 {
		t.Fatal("no results for tempe")
	}
	if !strings.Contains(strings.ToLower(got[0].NameID), "tempe") {
		t.Errorf("top hit for 'tempe' is %q — ranking is not working", got[0].NameID)
	}
}

// Refusing to guess is the whole design. A nonsense query must return nothing rather
// than the least-bad fuzzy match.
func TestSearchReturnsNothingForNonsense(t *testing.T) {
	tab := load(t)
	if got, _ := tab.Search("zzzqqxnotafood", 5); len(got) != 0 {
		t.Errorf("Search(nonsense) returned %d results: %v", len(got), got[0].NameID)
	}
}

// Values are per 100 g edible portion; a logged portion must scale from that.
func TestMacrosScaleToPortion(t *testing.T) {
	tab := load(t)
	f, _ := tab.ByCode("AR001")

	m := f.Macros(200)
	if math.Abs(m.Kcal-714) > 0.5 {
		t.Errorf("200 g of AR001 = %v kcal, want 714 (2 × 357)", m.Kcal)
	}
	if math.Abs(m.ProteinG-16.8) > 0.05 {
		t.Errorf("200 g protein = %v, want 16.8", m.ProteinG)
	}
	half := f.Macros(50)
	if math.Abs(half.Kcal-178.5) > 0.5 {
		t.Errorf("50 g = %v kcal, want 178.5", half.Kcal)
	}
}

func TestMacrosRejectNonPositivePortions(t *testing.T) {
	tab := load(t)
	f, _ := tab.ByCode("AR001")
	for _, g := range []float64{0, -10} {
		if m := f.Macros(g); m.Kcal != 0 {
			t.Errorf("Macros(%v) returned %v kcal; a non-portion must yield nothing", g, m.Kcal)
		}
	}
}

// A flagged row must announce itself. Presenting an unverified number as measured fact
// is exactly what the verification pass exists to prevent.
func TestFlaggedFoodsCarryTheirFlags(t *testing.T) {
	tab := load(t)
	var flagged *Food
	for i := range tab.All() {
		if !tab.All()[i].Verified() {
			flagged = &tab.All()[i]
			break
		}
	}
	if flagged == nil {
		t.Fatal("no flagged food found")
	}
	if len(flagged.Flags()) == 0 {
		t.Errorf("%s is unverified but reports no flags", flagged.Code)
	}
}

// "tempe" must not surface fried tempe chips first. The chips are 581 kcal against 150-201
// for tempe itself, so an agent taking the top hit would log nearly four times the energy.
func TestSearchPrefersNamesThatBeginWithTheQuery(t *testing.T) {
	tab := load(t)
	got, _ := tab.Search("tempe", 5)
	if len(got) == 0 {
		t.Fatal("no results for tempe")
	}
	first := strings.ToLower(got[0].NameID)
	if !strings.HasPrefix(first, "tempe") {
		t.Errorf("top hit for 'tempe' is %q; a name starting with the query should win",
			got[0].NameID)
	}
	if strings.Contains(first, "keripik") || strings.Contains(first, "kerupik") {
		t.Errorf("top hit for 'tempe' is a crisp/chip variant (%q)", got[0].NameID)
	}
}

// The reason for taking on bleve: a slip should still find the food. The hand-rolled
// matcher this replaced required exact substrings, so one wrong letter found nothing.
func TestSearchToleratesASingleTypo(t *testing.T) {
	tab := load(t)
	for _, tc := range []struct{ typo, want string }{
		{"tempo", "tempe"},  // e -> o
		{"berass", "beras"}, // doubled letter
		{"tahi", "tahu"},    // u -> i
	} {
		got, _ := tab.Search(tc.typo, 5)
		if len(got) == 0 {
			t.Errorf("Search(%q) found nothing; a single typo should still reach %q",
				tc.typo, tc.want)
			continue
		}
		var hit bool
		for _, f := range got {
			if strings.Contains(strings.ToLower(f.NameID), tc.want) {
				hit = true
				break
			}
		}
		if !hit {
			t.Errorf("Search(%q) returned %q first; expected something containing %q",
				tc.typo, got[0].NameID, tc.want)
		}
	}
}

// Fuzziness must not become a licence to guess. Two edits away is a different word.
func TestFuzzinessDoesNotReachUnrelatedFoods(t *testing.T) {
	tab := load(t)
	if got, _ := tab.Search("zzzqqxnotafood", 5); len(got) != 0 {
		t.Errorf("nonsense matched %d foods: %q", len(got), got[0].NameID)
	}
	if got, _ := tab.Search("automobile", 5); len(got) != 0 {
		t.Errorf("an unrelated English word matched %d foods: %q", len(got), got[0].NameID)
	}
}
