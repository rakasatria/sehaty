package storage

import "testing"

func TestFoodAliasRoundTrip(t *testing.T) {
	db, err := Open(t.TempDir()+"/t.db", testKey)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := db.PutFoodAlias("CP077", "soybean tempeh, raw", "agent"); err != nil {
		t.Fatal(err)
	}
	got, err := db.FoodAliases()
	if err != nil {
		t.Fatal(err)
	}
	if got["CP077"].NameEN != "soybean tempeh, raw" {
		t.Fatalf("stored alias = %+v", got["CP077"])
	}
	if got["CP077"].Source != "agent" {
		t.Errorf("source = %q; provenance must survive so a machine guess is distinguishable "+
			"from a human correction", got["CP077"].Source)
	}
}

// A later correction must replace the earlier value, not accumulate duplicates.
func TestFoodAliasIsUpserted(t *testing.T) {
	db, _ := Open(t.TempDir()+"/t.db", testKey)
	defer db.Close()

	must := func(e error) {
		if e != nil {
			t.Fatal(e)
		}
	}
	must(db.PutFoodAlias("CP077", "tempeh", "agent"))
	must(db.PutFoodAlias("CP077", "soybean tempeh", "user"))

	got, err := db.FoodAliases()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("%d aliases stored for one code", len(got))
	}
	if got["CP077"].NameEN != "soybean tempeh" || got["CP077"].Source != "user" {
		t.Errorf("correction did not replace the guess: %+v", got["CP077"])
	}
}

// "This food has no English name" is a real answer and must be storable. Otherwise every
// run re-asks about oncom and gembus forever.
func TestFoodAliasCanRecordNoEnglishName(t *testing.T) {
	db, _ := Open(t.TempDir()+"/t.db", testKey)
	defer db.Close()

	if err := db.PutFoodAlias("CP071", "", "agent"); err != nil {
		t.Fatal(err)
	}
	got, err := db.FoodAliases()
	if err != nil {
		t.Fatal(err)
	}
	a, ok := got["CP071"]
	if !ok {
		t.Fatal("an explicit 'no English name' was not recorded at all")
	}
	if a.NameEN != "" {
		t.Errorf("NameEN = %q, want empty", a.NameEN)
	}
}

func TestFoodAliasRejectsAnEmptyCode(t *testing.T) {
	db, _ := Open(t.TempDir()+"/t.db", testKey)
	defer db.Close()
	if err := db.PutFoodAlias("", "something", "agent"); err == nil {
		t.Fatal("accepted an alias with no food code")
	}
}
