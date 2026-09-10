package agent

import (
	"os"
	"strings"
	"testing"
)

// The split moves text; it does not rewrite it.
//
// It cannot be a byte comparison: four sections are discontiguous in the original — THE
// RULE and WHAT YOU WILL NOT SUGGEST both belong in limits.md but sat second and eighth —
// so composing whole files necessarily reorders them. What must NOT change is the text
// itself, so this compares the multiset of lines: nothing reworded, nothing dropped,
// nothing invented. Blank lines are ignored, because blank-line structure legitimately
// shifts at a file boundary.
func TestTheSplitChangedNoText(t *testing.T) {
	want, err := os.ReadFile("testdata/prompt.golden")
	if err != nil {
		t.Fatal(err)
	}
	before, after := contentLines(string(want)), contentLines(soulText())

	if lost := absent(before, after); len(lost) > 0 {
		t.Errorf("%d line(s) LOST in the split. First few:", len(lost))
		for i, l := range lost {
			if i >= 5 {
				break
			}
			t.Errorf("  lost: %q", l)
		}
	}
	if added := absent(after, before); len(added) > 0 {
		t.Errorf("%d line(s) ADDED in the split. First few:", len(added))
		for i, l := range added {
			if i >= 5 {
				break
			}
			t.Errorf("  added: %q", l)
		}
	}
}

// contentLines is every non-blank line, trailing whitespace removed.
func contentLines(s string) []string {
	var out []string
	for _, l := range strings.Split(s, "\n") {
		if t := strings.TrimRight(l, " \t"); strings.TrimSpace(t) != "" {
			out = append(out, t)
		}
	}
	return out
}

// absent returns the lines of a that b does not have, counting duplicates.
func absent(a, b []string) []string {
	pool := map[string]int{}
	for _, l := range b {
		pool[l]++
	}
	var out []string
	for _, l := range a {
		if pool[l] > 0 {
			pool[l]--
			continue
		}
		out = append(out, l)
	}
	return out
}

// The reordering the split forces is deliberate, so it is asserted rather than tolerated.
// If a future edit shuffles these, that is a decision someone should have to make on
// purpose.
func TestTheComposedOrderIsTheIntendedOne(t *testing.T) {
	got := soulText()
	order := []string{
		"WHAT YOU ARE",
		"LANGUAGE",
		"WHAT YOU ARE FOR",
		"THE RULE THAT MATTERS MOST",
		"WHAT YOU WILL NOT SUGGEST",
		"FOOD",
		"THE FIRST CONSULTATION",
		"ESTIMATING ENERGY",
		"WHAT NOT TO DO",
		"MANNER",
		"STYLE",
		"ENCOURAGEMENT",
	}
	at := -1
	for _, section := range order {
		i := strings.Index(got, "\n"+section)
		if i < 0 {
			t.Fatalf("the composed soul has no %q section", section)
		}
		if i < at {
			t.Errorf("%q appears out of the intended order", section)
		}
		at = i
	}
}

// Every claim carries its source, and the model never sees one. A model that can read
// its own citations starts performing them — "research shows" in a health record is
// exactly the sentence this project must never produce.
func TestProvenanceIsStrippedBeforeTheModelSeesIt(t *testing.T) {
	if strings.Contains(SystemPrompt, "<!--") {
		t.Error("an HTML comment reached the prompt")
	}
	for _, tag := range []string{"PAPER:", "RULE:", "DATA:", "JUDGEMENT:"} {
		if strings.Contains(SystemPrompt, tag) {
			t.Errorf("the provenance tag %q reached the prompt", tag)
		}
	}
	if strings.Contains(SystemPrompt, "research shows") {
		t.Error("the prompt invites Sehaty to cite research")
	}
}

// The tags have to exist to be stripped. A soul with no provenance is a soul nobody can
// audit, and JUDGEMENT is the tag that earns its keep: it marks a rule as somebody's
// confident guess, and during this design five such guesses were checked and all five
// were wrong.
func TestTheSourceFilesActuallyCarryProvenance(t *testing.T) {
	files, err := soulFS.ReadDir("soul/skills")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 6 {
		t.Fatalf("expected six behaviour skills, found %d", len(files))
	}

	tagged, judgement := 0, 0
	for _, name := range []string{"soul/intent.md", "soul/limits.md", "soul/manner.md"} {
		b, err := soulFS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		tagged += strings.Count(string(b), "<!--")
		judgement += strings.Count(string(b), "JUDGEMENT:")
	}
	for _, f := range files {
		b, err := soulFS.ReadFile("soul/skills/" + f.Name())
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "<!--") {
			t.Errorf("%s carries no provenance at all", f.Name())
		}
		tagged += strings.Count(string(b), "<!--")
		judgement += strings.Count(string(b), "JUDGEMENT:")
	}
	if tagged < 20 {
		t.Errorf("only %d claims carry a source; the soul is mostly unattributed", tagged)
	}
	if judgement == 0 {
		t.Error("nothing is marked JUDGEMENT, which means either perfect evidence or a dishonest tagger")
	}
}

// Stripping must remove the comment and leave the line, including its trailing content,
// without eating the newline that separates two rules.
func TestStripRemovesOnlyTheComment(t *testing.T) {
	in := "Praise the person.   <!-- PAPER: +0.33 -->\nNever a token.\n"
	want := "Praise the person.\nNever a token.\n"
	if got := strip(in); got != want {
		t.Errorf("strip(%q) = %q, want %q", in, got, want)
	}

	multi := "A rule.\n<!--\nPAPER: something\nover two lines\n-->\nAnother rule.\n"
	if got := strip(multi); strings.Contains(got, "PAPER") {
		t.Errorf("a multi-line comment survived: %q", got)
	}
}
