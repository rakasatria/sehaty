package agent

import (
	"embed"
	"regexp"
	"strings"
)

//go:embed soul
var soulFS embed.FS

// soulOrder is the order the files are composed in.
//
// Not alphabetical and not discovered: the sequence is load-bearing. What Sehaty IS comes
// before what it will not do, which comes before how it sounds, and the behaviour skills
// come last because each one assumes all three.
var soulOrder = []string{
	"soul/intent.md",
	"soul/limits.md",
	"soul/skills/taking-a-food-entry.md",
	"soul/skills/asking-a-question.md",
	"soul/skills/declining-a-target.md",
	"soul/skills/after-a-gap.md",
	"soul/skills/when-they-push-back.md",
	"soul/manner.md",
	"soul/skills/noticing-progress.md",
}

// soulText is the soul with its provenance stripped: what the model is given, and
// nothing else.
//
// Every claim in those files carries its source in an HTML comment, visible to anyone
// reading the file and invisible here. Stripping saves tokens and prevents something
// worse: a model that can see its citations starts performing them, and "research shows"
// in a health record is a sentence this project must never produce. The papers decide
// what Sehaty does and stay out of what it says.
func soulText() string {
	var b strings.Builder
	for _, name := range soulOrder {
		raw, err := soulFS.ReadFile(name)
		if err != nil {
			// A missing file is a build-time mistake, not a runtime condition: the
			// files are embedded, so if one is absent the binary was built wrong.
			panic("soul: " + name + ": " + err.Error())
		}
		b.WriteString(strip(string(raw)))
	}
	return b.String()
}

// comment matches an HTML comment, including one spanning several lines.
var comment = regexp.MustCompile(`(?s)[ \t]*<!--.*?-->`)

// strip removes provenance comments and the whitespace that preceded them, leaving the
// claim and its newline exactly as they were.
func strip(md string) string {
	return comment.ReplaceAllString(md, "")
}

// Compose builds the system prompt: the soul, then the section generated from the
// capability registry.
//
// Two sources, and the boundary between them is the point. What Sehaty is and how it
// behaves is written by a person and reviewed by a person. What it can and cannot do,
// and why each question is asked, is generated from the tools — so it cannot drift from
// them, and nobody has to remember to update a paragraph when a tool gains an input.
func Compose() string {
	return soulText()
}

// SystemPrompt is what the model is given. Computed once at init from the embedded soul.
var SystemPrompt = Compose()
