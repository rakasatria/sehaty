package food

import "testing"

// The bug: Indonesian qualifies food, and the table does not.
//
// "Nasi putih" is what everyone says. The table calls entry AP001 "Nasi" and contains no
// entry with "putih" in it, so an all-terms-must-match query returned nothing — and the
// only conclusion available from nothing is that rice is absent from the Indonesian food
// table. It is not; it is the first entry in the cereals group.
func TestAQualifiedNameStillFindsThePlainEntry(t *testing.T) {
	tbl := load(t)

	strict, exact := tbl.Search("nasi", 5)
	if len(strict) == 0 || !exact {
		t.Fatalf("the plain name stopped working: %d hits, exact=%v", len(strict), exact)
	}

	loose, exact := tbl.Search("nasi putih", 5)
	if len(loose) == 0 {
		t.Fatal("a qualified name still finds nothing at all")
	}
	if exact {
		t.Error("a partial match is being reported as exact")
	}
	if loose[0].NameID != strict[0].NameID {
		t.Errorf("the loosened search ranked %q first, not %q",
			loose[0].NameID, strict[0].NameID)
	}
}

// Loosening must not leak into single-word queries: "tempe" finding nothing means
// nothing, and quietly widening it would turn an honest miss into a wrong answer.
func TestASingleWordMissStaysAMiss(t *testing.T) {
	found, exact := load(t).Search("zzzqqq", 5)
	if len(found) != 0 || exact {
		t.Errorf("a nonsense single-word query returned %d hits (exact=%v)", len(found), exact)
	}
}

// An exact match must never be downgraded just because the query had two words.
func TestAMultiWordExactMatchIsStillExact(t *testing.T) {
	found, exact := load(t).Search("nasi tim", 5)
	if len(found) == 0 {
		t.Fatal("a real two-word entry cannot be found")
	}
	if !exact {
		t.Error("a genuine all-terms match was reported as partial")
	}
}
