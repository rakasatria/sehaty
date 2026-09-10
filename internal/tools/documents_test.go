package tools

import (
	"strings"
	"testing"
)

func TestCanonicalKeyCollapsesSpellingVariants(t *testing.T) {
	for _, in := range []string{"Diet_Plan", "diet plan", "  Diet   Plan  ", "DIET-PLAN", "diet--plan"} {
		got, err := canonicalKey(in)
		if err != nil {
			t.Fatalf("canonicalKey(%q): %v", in, err)
		}
		if got != "diet-plan" {
			t.Errorf("canonicalKey(%q) = %q, want diet-plan", in, got)
		}
	}
	if _, err := canonicalKey("   "); err == nil {
		t.Error("accepted an empty key")
	}
	if _, err := canonicalKey(strings.Repeat("a", 80)); err == nil {
		t.Error("accepted an 80-character key")
	}
}

func TestPutDocumentRequiresCreateNewForAFreshKey(t *testing.T) {
	d := foodDeps(t)
	_, err := PutDocument(d, PutDocumentArgs{Profile: "raka", Key: "diet", Body: "hello"})
	if err == nil {
		t.Fatal("created a document without create_new")
	}
	if !strings.Contains(err.Error(), "create_new") {
		t.Errorf("error does not tell the caller how to proceed: %v", err)
	}
}

func TestPutDocumentCreatesThenRequiresExpectedVersion(t *testing.T) {
	d := foodDeps(t)
	out, err := PutDocument(d, PutDocumentArgs{Profile: "raka", Key: "diet",
		Title: "Nutritionist plan", Kind: "prescription", Body: "v1", CreateNew: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.Version != 1 {
		t.Fatalf("version = %d, want 1", out.Version)
	}
	// A blind second write must be refused.
	if _, err := PutDocument(d, PutDocumentArgs{Profile: "raka", Key: "diet",
		Body: "v2"}); err == nil {
		t.Fatal("overwrote an existing document without expected_version")
	}
	got, err := PutDocument(d, PutDocumentArgs{Profile: "raka", Key: "diet",
		Body: "v2", ExpectedVersion: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != 2 {
		t.Fatalf("version = %d, want 2", got.Version)
	}
}

// The failure this whole mechanism exists for.
func TestPutDocumentRejectsAStaleWrite(t *testing.T) {
	d := foodDeps(t)
	must := func(a PutDocumentArgs) {
		if _, err := PutDocument(d, a); err != nil {
			t.Fatal(err)
		}
	}
	must(PutDocumentArgs{Profile: "raka", Key: "diet", Body: "v1", CreateNew: true})
	must(PutDocumentArgs{Profile: "raka", Key: "diet", Body: "v2", ExpectedVersion: 1})

	_, err := PutDocument(d, PutDocumentArgs{Profile: "raka", Key: "diet",
		Body: "written from a stale read", ExpectedVersion: 1})
	if err == nil {
		t.Fatal("a stale write succeeded")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "re-read") {
		t.Errorf("conflict does not tell the caller to re-read: %v", err)
	}
	cur, err := GetDocument(d, GetDocumentArgs{Profile: "raka", Key: "diet"})
	if err != nil {
		t.Fatal(err)
	}
	if cur.Body != "v2" {
		t.Errorf("head body = %q; the stale write leaked through", cur.Body)
	}
}

// diet-plan and dietplan are the same document wearing different clothes.
func TestPutDocumentRefusesANearDuplicateKey(t *testing.T) {
	d := foodDeps(t)
	if _, err := PutDocument(d, PutDocumentArgs{Profile: "raka", Key: "diet-plan",
		Body: "v1", CreateNew: true}); err != nil {
		t.Fatal(err)
	}
	_, err := PutDocument(d, PutDocumentArgs{Profile: "raka", Key: "dietplan",
		Body: "a second one", CreateNew: true})
	if err == nil {
		t.Fatal("created dietplan alongside diet-plan")
	}
	if !strings.Contains(err.Error(), "diet-plan") {
		t.Errorf("error does not name the existing document: %v", err)
	}
}

func TestGetDocumentReturnsTheVersionToWriteBackWith(t *testing.T) {
	d := foodDeps(t)
	if _, err := PutDocument(d, PutDocumentArgs{Profile: "raka", Key: "diet",
		Body: "the plan", CreateNew: true}); err != nil {
		t.Fatal(err)
	}
	got, err := GetDocument(d, GetDocumentArgs{Profile: "raka", Key: "Diet"}) // note casing
	if err != nil {
		t.Fatal(err)
	}
	if got.Body != "the plan" {
		t.Errorf("body = %q", got.Body)
	}
	if !strings.Contains(got.Note, "expected_version=1") {
		t.Errorf("note does not hand back the version to write with: %q", got.Note)
	}
}

func TestOldVersionsStayReadable(t *testing.T) {
	d := foodDeps(t)
	must := func(a PutDocumentArgs) {
		if _, err := PutDocument(d, a); err != nil {
			t.Fatal(err)
		}
	}
	must(PutDocumentArgs{Profile: "raka", Key: "diet", Body: "September plan", CreateNew: true})
	must(PutDocumentArgs{Profile: "raka", Key: "diet", Body: "October revision", ExpectedVersion: 1})

	old, err := GetDocument(d, GetDocumentArgs{Profile: "raka", Key: "diet", Version: 1})
	if err != nil {
		t.Fatal(err)
	}
	if old.Body != "September plan" {
		t.Errorf("version 1 = %q; what I was told in September must stay answerable", old.Body)
	}
}

func TestListDocumentsHasNoBodiesAndSuggestsKeys(t *testing.T) {
	d := foodDeps(t)
	out, err := ListDocuments(d, "raka")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Suggested) == 0 {
		t.Error("no suggested keys for an empty profile — the namespace is invisible")
	}
	if _, err := PutDocument(d, PutDocumentArgs{Profile: "raka", Key: "diet",
		Title: "Plan", Body: "secret body text", CreateNew: true}); err != nil {
		t.Fatal(err)
	}
	out, err = ListDocuments(d, "raka")
	if err != nil {
		t.Fatal(err)
	}
	if out.Count != 1 {
		t.Fatalf("count = %d", out.Count)
	}
	for _, s := range out.Documents {
		if strings.Contains(s.Title, "secret body") {
			t.Error("a body leaked into the listing")
		}
	}
}

func TestDocumentHistoryListsNewestFirst(t *testing.T) {
	d := foodDeps(t)
	must := func(a PutDocumentArgs) {
		if _, err := PutDocument(d, a); err != nil {
			t.Fatal(err)
		}
	}
	must(PutDocumentArgs{Profile: "raka", Key: "diet", Body: "v1", CreateNew: true})
	must(PutDocumentArgs{Profile: "raka", Key: "diet", Body: "v2", ExpectedVersion: 1})
	must(PutDocumentArgs{Profile: "raka", Key: "diet", Body: "v3", ExpectedVersion: 2})

	h, err := DocumentHistory(d, "raka", "diet")
	if err != nil {
		t.Fatal(err)
	}
	if h.Count != 3 {
		t.Fatalf("count = %d, want 3", h.Count)
	}
	if h.Versions[0].Version != 3 {
		t.Errorf("history starts at %d, want 3 (newest first)", h.Versions[0].Version)
	}
}

func TestPutDocumentRejectsAnEmptyBody(t *testing.T) {
	d := foodDeps(t)
	if _, err := PutDocument(d, PutDocumentArgs{Profile: "raka", Key: "diet",
		Body: "   ", CreateNew: true}); err == nil {
		t.Fatal("stored an empty document")
	}
}
