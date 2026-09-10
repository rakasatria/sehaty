package capability

// Have is the set of fields whose value is actually on file.
//
// Have and Asked are DIFFERENT QUESTIONS and conflating them was the trap this pair
// exists to avoid. "Do I have an age?" decides whether estimate_energy can run. "Has the
// age question been put?" decides whether to ask again. Someone who declined to give
// their age has answered the question and still has no age, and only one of those two
// facts should stop the tool.
type Have map[Field]bool

// Asked is the set of questions already put, declines included.
type Asked map[Field]bool

// Blocked reports the unmet HARD requirements of a tool.
//
// A tool absent from the registry needs nothing and is never blocked. That is the safe
// default in both directions: a typo cannot gate a working tool, and a tool that needs
// something must say so here to get it enforced.
func Blocked(tool string, have Have) ([]Requirement, bool) {
	var unmet []Requirement
	for _, c := range known {
		if c.Tool != tool {
			continue
		}
		for _, n := range c.Needs {
			if n.Hard && !have[n.Field] {
				unmet = append(unmet, n)
			}
		}
	}
	return unmet, len(unmet) > 0
}

// SoftGaps reports the unmet SOFT requirements of a tool: what it is proceeding without,
// so it can say so rather than assume in silence.
func SoftGaps(tool string, have Have) []Requirement {
	var gaps []Requirement
	for _, c := range known {
		if c.Tool != tool {
			continue
		}
		for _, n := range c.Needs {
			if !n.Hard && !have[n.Field] {
				gaps = append(gaps, n)
			}
		}
	}
	return gaps
}

// ToAsk is what is still worth putting to this person: unmet, and not yet put.
//
// In ask order, not in unlock order. Unlock order says what would be most useful to know
// next; ask order says what a person will not mind being asked next, and the second one
// is what decides whether they answer at all. The brief leads with the energy inputs
// separately when the intake is unfinished, which is where unlock order gets its say.
//
// The Because carried here is the QUESTION's reason — what the field is for in general —
// rather than any one tool's, because the same field usually serves several.
func ToAsk(have Have, asked Asked) []Requirement {
	var out []Requirement
	for _, q := range askOrder {
		if have[q.Field] || asked[q.Field] {
			continue
		}
		out = append(out, Requirement{Field: q.Field, Because: q.Because})
	}
	return out
}

// Locked is what this person cannot do yet, and the unmet hard requirements that are the
// reason. Soft gaps are excluded: those do not lock anything.
func Locked(have Have) []Capability {
	var out []Capability
	for _, c := range known {
		unmet, blocked := Blocked(c.Tool, have)
		if !blocked {
			continue
		}
		out = append(out, Capability{Tool: c.Tool, Needs: unmet, Unlocks: c.Unlocks})
	}
	return out
}

// aliases map the finer-grained names a UI may use onto the question they answer.
//
// "Seminggu berapa kali, dan berapa lama?" is one question to a person and two sets of
// buttons on a phone.
var aliases = map[string]Field{
	"sessions per week": FieldSchedule,
	"session minutes":   FieldSchedule,
}

// Canonical resolves a question name to the field it answers.
func Canonical(q string) (Field, bool) {
	if f, ok := aliases[q]; ok {
		return f, true
	}
	for _, k := range askOrder {
		if string(k.Field) == q {
			return k.Field, true
		}
	}
	if q == string(FieldWeightLog) {
		return FieldWeightLog, true
	}
	return "", false
}

// Names lists the fields of a requirement or question slice, for error messages.
func Names(in any) []string {
	switch v := in.(type) {
	case []Requirement:
		out := make([]string, 0, len(v))
		for _, r := range v {
			out = append(out, string(r.Field))
		}
		return out
	case []Question:
		out := make([]string, 0, len(v))
		for _, q := range v {
			out = append(out, string(q.Field))
		}
		return out
	}
	return nil
}
