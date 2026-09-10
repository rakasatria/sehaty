// Package guardrails decides what Sehaty must not do.
//
// Two mechanisms, deliberately different in severity:
//
//	Screen    some situations are not "train around it" situations. Sehaty declines to
//	          plan at all and says who to ask instead.
//	Excluded  ordinary injuries NARROW the plan. A bad knee should cost you squats, not
//	          your whole training week.
//
// The distinction matters. A tool that refuses everyone with a sore shoulder is a tool
// people stop telling the truth to, and a person who hides an injury gets a worse plan than
// one who declares it.
//
// THIS IS NOT MEDICAL SCREENING. It is a keyword filter over what someone typed. It will
// miss phrasings it does not know, and it errs toward caution when it does match. It exists
// so the obvious cases are handled, not so anyone can claim the output is clinically safe.
package guardrails

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/rakasatria/sehaty/internal/catalog"
)

// highRisk are situations where a general fitness plan is the wrong answer regardless of
// how it is constructed.
var highRisk = []string{
	"chest pain", "chest tightness", "heart condition", "heart attack", "cardiac",
	"angina", "arrhythmia", "pregnant", "pregnancy", "postpartum",
	"eating disorder", "anorexia", "bulimia",
	"fainting", "faint", "syncope", "dizzy spells", "blackout",
	"severe injury", "recent surgery", "had surgery", "blood clot", "stroke",
	"seizure", "chemotherapy", "uncontrolled",
}

// negators cancel a term that follows them closely. The implementation this borrows from
// lacked this and would refuse to help someone who wrote "no chest pain" — declining to
// assist a person who has explicitly said they are fine is a failure, not a safe default.
var negators = []string{"no ", "not ", "never ", "without ", "denies ", "negative for ", "free of ", "history of no "}

// negationWindow is how far before a term a negator still applies. Long enough for
// "no history of eating disorder", short enough that a negation in an unrelated clause
// does not silently disarm a real warning.
const negationWindow = 24

type Refusal struct {
	Term    string // the phrase that triggered it, so the person can see why
	Message string
}

// Screen reports whether planning should be refused outright.
func Screen(limitations []string) (Refusal, bool) {
	text := strings.ToLower(strings.Join(limitations, "; "))
	for _, term := range highRisk {
		idx := strings.Index(text, term)
		if idx < 0 || negated(text, idx) {
			continue
		}
		return Refusal{
			Term: term,
			Message: fmt.Sprintf(
				"Sehaty will not build a plan around %q. That needs a qualified health "+
					"professional who can examine you, not an app. Ask your doctor or "+
					"physiotherapist first — once they have cleared you, come back and "+
					"say what they advised.", term),
		}, true
	}
	return Refusal{}, false
}

// negated reports whether the term at idx is preceded by a negator.
func negated(text string, idx int) bool {
	start := idx - negationWindow
	if start < 0 {
		start = 0
	}
	before := text[start:idx]
	for _, n := range negators {
		if strings.Contains(before, n) {
			return true
		}
	}
	return false
}

// rule links stated limitations to the movements they rule out.
//
// Matching is on the exercise NAME first because that is where the movement pattern lives;
// body part and target are broader nets for cases the name does not reveal.
type rule struct {
	when    []string
	name    *regexp.Regexp
	body    []string
	target  []string
	because string
}

var rules = []rule{
	{
		when:    []string{"knee", "acl", "mcl", "meniscus", "patella"},
		name:    regexp.MustCompile(`(?i)(squat|lunge|jump|step[- ]?up|leg press|leg extension|pistol|burpee|sprint|sissy)`),
		target:  []string{"quads"},
		because: "loads the knee",
	},
	{
		// Deliberately NOT excluding all of "back": a lat pulldown is usually fine and
		// removing every back exercise would leave nothing to train.
		when:    []string{"lower back", "low back", "lumbar", "herniated", "disc", "sciatica", "spine"},
		name:    regexp.MustCompile(`(?i)(deadlift|good morning|bent[- ]?over|back extension|hyperextension|sit[- ]?up|clean|snatch|jerk|romanian)`),
		because: "loads the lumbar spine",
	},
	{
		when:    []string{"shoulder", "rotator cuff", "labrum", "ac joint", "impingement"},
		name:    regexp.MustCompile(`(?i)(overhead|shoulder press|military|snatch|jerk|upright row|lateral raise|\bdips?\b|behind the neck|handstand)`),
		body:    []string{"shoulders"},
		because: "loads the shoulder",
	},
	{
		when:    []string{"wrist", "carpal"},
		name:    regexp.MustCompile(`(?i)(push[- ]?up|plank|handstand|front rack|clean|burpee|wrist)`),
		because: "loads the wrist in extension",
	},
	{
		when:    []string{"elbow", "tennis elbow", "golfer's elbow", "tendonitis"},
		name:    regexp.MustCompile(`(?i)(curl|extension|\bdips?\b|chin[- ]?up|pull[- ]?up|push[- ]?down|skull)`),
		because: "loads the elbow",
	},
	{
		when:    []string{"ankle", "achilles", "plantar", "shin"},
		name:    regexp.MustCompile(`(?i)(jump|calf|sprint|\brun\b|lunge|box|skip|hop)`),
		body:    []string{"lower legs"},
		because: "loads the ankle",
	},
	{
		when:    []string{"hip", "groin", "labral", "piriformis"},
		name:    regexp.MustCompile(`(?i)(squat|lunge|deadlift|adduction|abduction|leg raise|split)`),
		because: "loads the hip",
	},
	{
		when:    []string{"hernia"},
		name:    regexp.MustCompile(`(?i)(deadlift|squat|sit[- ]?up|crunch|leg raise|clean|snatch|press|plank)`),
		because: "raises intra-abdominal pressure",
	},
	{
		when:    []string{"neck", "cervical", "whiplash"},
		name:    regexp.MustCompile(`(?i)(shrug|overhead|bridge|headstand|handstand|behind the neck|neck)`),
		body:    []string{"neck"},
		because: "loads the neck",
	},
}

// Excluded reports whether an exercise is contraindicated, and why.
//
// The reason is returned rather than swallowed so a person can see WHY a movement vanished
// from their plan. Silent removal looks like the app being broken, and invites them to go
// looking for the exercise on their own.
func Excluded(limitations []string, e catalog.Exercise) (bool, string) {
	stated := strings.ToLower(strings.Join(limitations, "; "))
	if strings.TrimSpace(stated) == "" {
		return false, ""
	}
	name := strings.ToLower(e.Name)
	body := strings.ToLower(e.BodyPart)
	target := strings.ToLower(e.Target)

	// Mobility work is usually what an injured joint NEEDS, not what it must avoid. A
	// quad stretch targets "quads" and so tripped the knee rule, which excluded exactly
	// the movements a physio would prescribe. Stretches remain subject to the name rules
	// below — a "jump stretch" is still a jump — but not to the broad body/target nets.
	isStretch := strings.Contains(name, "stretch") || strings.Contains(name, "yoga") ||
		strings.Contains(name, "mobility")

	for _, r := range rules {
		hit := ""
		for _, w := range r.when {
			if i := strings.Index(stated, w); i >= 0 && !negated(stated, i) {
				hit = w
				break
			}
		}
		if hit == "" {
			continue
		}
		if r.name != nil && r.name.MatchString(name) {
			return true, fmt.Sprintf("%s (%s)", r.because, hit)
		}
		if isStretch {
			continue
		}
		for _, b := range r.body {
			if body == b {
				return true, fmt.Sprintf("%s (%s)", r.because, hit)
			}
		}
		for _, tg := range r.target {
			if target == tg {
				return true, fmt.Sprintf("%s (%s)", r.because, hit)
			}
		}
	}
	return false, ""
}

// Filter removes contraindicated exercises, returning what remains and what was dropped.
func Filter(limitations []string, in []catalog.Exercise) (kept []catalog.Exercise, dropped map[string]string) {
	if len(limitations) == 0 {
		return in, nil
	}
	dropped = map[string]string{}
	for _, e := range in {
		if excluded, why := Excluded(limitations, e); excluded {
			dropped[e.Name] = why
			continue
		}
		kept = append(kept, e)
	}
	if len(dropped) == 0 {
		dropped = nil
	}
	return kept, dropped
}

// Summary is what a caller is told about exclusions.
//
// The full map is hundreds of entries for a plan of seven exercises — useless to a person
// and expensive to send to a model. What matters is that removals happened, why, and a few
// examples to make it concrete.
type Summary struct {
	Count    int      `json:"count"`
	Reasons  []string `json:"reasons"`
	Examples []string `json:"examples,omitempty"`
}

// Summarise condenses the dropped map. Examples are sorted so the output is stable.
func Summarise(dropped map[string]string) *Summary {
	if len(dropped) == 0 {
		return nil
	}
	seen := map[string]bool{}
	s := &Summary{Count: len(dropped)}
	names := make([]string, 0, len(dropped))
	for name, why := range dropped {
		names = append(names, name)
		if !seen[why] {
			seen[why] = true
			s.Reasons = append(s.Reasons, why)
		}
	}
	sort.Strings(names)
	sort.Strings(s.Reasons)
	if len(names) > 5 {
		names = names[:5]
	}
	s.Examples = names
	return s
}
