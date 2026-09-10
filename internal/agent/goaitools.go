package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/zendev-sh/goai"
)

// profileKey carries the profile id through the tool loop.
//
// THE PROFILE IS NEVER AN ARGUMENT. Not in any schema the model sees, and therefore not
// something it can name, guess or hallucinate. It rides in the context instead, put there
// by the caller from a verified channel identity and read back by each tool at the moment
// it runs. A model that invents "profile": "someone-else" is writing a field that does not
// exist in the tool it is calling.
type profileKey struct{}

func withProfile(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, profileKey{}, id)
}

func profileFrom(ctx context.Context) string {
	id, _ := ctx.Value(profileKey{}).(string)
	return id
}

// Tools exposes the registry as goai tools.
//
// The conversion is almost nothing, which is the point of choosing this SDK: goai.Tool
// carries its schema as raw JSON and its handler as func(ctx, json.RawMessage) (string,
// error). Sehaty's schemas are already JSON and Invoke already has that shape, so the
// hand-written tool descriptions cross over untouched rather than being re-derived
// through a reflection layer that could quietly drop one.
func (r *Registry) Tools() ([]goai.Tool, error) {
	out := make([]goai.Tool, 0, len(r.schemas))
	for _, s := range r.schemas {
		raw, err := json.Marshal(s.Function.Parameters)
		if err != nil {
			return nil, fmt.Errorf("tool %s: %w", s.Function.Name, err)
		}
		name := s.Function.Name
		out = append(out, goai.Tool{
			Name:        name,
			Description: s.Function.Description,
			InputSchema: raw,
			// Never returns an error, deliberately. A refusal is information the model
			// should act on — "that food is ambiguous, here are the codes" — not a
			// failure it should apologise for. Invoke turns refusals and panics alike
			// into an ordinary result, so a tool can never abort the run.
			Execute: func(ctx context.Context, in json.RawMessage) (string, error) {
				id := profileFrom(ctx)
				if id == "" {
					// Belt and braces. Reaching here means a caller invoked the agent
					// without binding an identity, and doing anything at all in that
					// state would be doing it to nobody in particular.
					return jsonErr("no profile is bound to this conversation"), nil
				}
				return r.Invoke(ctx, id, name, string(in)), nil
			},
		})
	}
	return out, nil
}
