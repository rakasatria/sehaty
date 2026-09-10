package telegram

import (
	"testing"

	"github.com/rakasatria/sehaty/internal/catalog"
	"github.com/rakasatria/sehaty/internal/crypto"
	"github.com/rakasatria/sehaty/internal/food"
	"github.com/rakasatria/sehaty/internal/storage"
	"github.com/rakasatria/sehaty/internal/tools"
)

// profileFixture is a minimal profile for copy tests that do not touch a database.
func profileFixture() storage.Profile {
	return storage.Profile{ID: "abc", DisplayName: "Raka", Goal: "fat_loss"}
}

// testDeps gives the button tests a real database and a real exercise catalogue.
//
// Real, not mocked, because what they are checking is precisely that the values in
// choices.go survive the same validation a typed answer would face — equipment names are
// rejected against the catalogue, and a preset naming a piece of kit the dataset does not
// have would fail only at the moment somebody taps it.
func testDeps(t *testing.T) tools.Deps {
	t.Helper()
	db, err := storage.Open(t.TempDir()+"/t.db",
		"00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	// The REAL dataset, not the three-entry fixture. These tests exist to prove the
	// equipment presets in choices.go are things the catalogue actually knows about, and
	// a cut-down catalogue would either pass them vacuously or fail them wrongly.
	cat, err := catalog.Load("../../third_party/exercises-dataset/data/exercises.json")
	if err != nil {
		t.Skipf("the exercise dataset submodule is not checked out: %v", err)
	}
	c, err := crypto.New("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	if err != nil {
		t.Fatal(err)
	}
	tbl, err := food.Load("../../data/tkpi/tkpi-2020.json")
	if err != nil {
		t.Fatal(err)
	}
	return tools.Deps{DB: db, Cat: cat, Cipher: c, Food: tbl}
}
