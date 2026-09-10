package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
)

// ChoiceQuestions are the assessment questions with a fixed set of answers.
//
// Age and height are absent deliberately: a number typed once is faster than scrolling a
// list of eighty, and a button grid pretending otherwise is decoration.
var ChoiceQuestions = []string{
	"goal",
	"experience",
	"sessions per week",
	"session minutes",
	"equipment",
	"injuries or conditions",
	"diet preference",
	"food allergies",
	"sex",
}

// Offer is how the model asks for buttons without being able to say what they do.
//
// The split matters. The model decides WHEN a question is worth offering as taps rather
// than typing — it is the only party that knows what it just said. What each button
// contains, what value it writes, and what happens on a tap are Go's, built from a fixed
// catalogue. So the worst a confused model can do is offer the wrong question's buttons,
// never invent an option or smuggle a value into one.
type Offer struct {
	mu       sync.Mutex
	question string
}

func (o *Offer) set(q string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.question = q
}

// Question reports what the model asked to be offered, if anything.
func (o *Offer) Question() string {
	if o == nil {
		return ""
	}
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.question
}

type offerKey struct{}

// WithOffer attaches a sink for the offer_choices tool to write into.
func WithOffer(ctx context.Context, o *Offer) context.Context {
	return context.WithValue(ctx, offerKey{}, o)
}

func offerFrom(ctx context.Context) *Offer {
	o, _ := ctx.Value(offerKey{}).(*Offer)
	return o
}

// registerOffer adds the offer_choices tool.
//
// Kept out of NewRegistry's list because it is the one tool that touches no health data:
// it changes how the next message is drawn, nothing else.
func (r *Registry) registerOffer() {
	r.add("offer_choices",
		"Show the answers to a profile question as buttons instead of asking them to type. "+
			"Call this in the SAME reply where you ask the question, then keep the question "+
			"itself to one short line and do NOT list the options in your text — they will "+
			"appear underneath. Only for: "+strings.Join(ChoiceQuestions, ", ")+". "+
			"Never for age or height; typing a number is faster than a grid.",
		obj(map[string]any{
			"question": str("which question to show buttons for"),
		}, "question"),
		func(ctx context.Context, _ string, a json.RawMessage) (any, error) {
			var in struct {
				Question string `json:"question"`
			}
			_ = json.Unmarshal(a, &in)
			q := strings.ToLower(strings.TrimSpace(in.Question))
			for _, known := range ChoiceQuestions {
				if q == known {
					// The sink is per-request. Its absence means the caller is not a
					// channel that can draw buttons — a terminal, say — which is not an
					// error, just a place where the question gets typed instead.
					if o := offerFrom(ctx); o != nil {
						o.set(q)
						return map[string]any{"showing": q,
							"note": "Buttons for " + q + " will appear under your reply. " +
								"Ask the question in one short line and do not list the options."}, nil
					}
					return map[string]any{"showing": "nothing",
						"note": "This conversation cannot show buttons. Ask normally and " +
							"list the options in your text."}, nil
				}
			}
			return nil, fmt.Errorf("%q has no fixed set of answers; ask for it in words. "+
				"Buttons exist only for: %s", in.Question, strings.Join(ChoiceQuestions, ", "))
		})
}
