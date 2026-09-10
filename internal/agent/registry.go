package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"

	"time"

	"github.com/rakasatria/sehaty/internal/storage"
	"github.com/rakasatria/sehaty/internal/tools"
)

type ToolSchema struct {
	Type     string `json:"type"`
	Function struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	} `json:"function"`
}

// Registry is the set of Sehaty tools a conversation may reach.
//
// Deliberately a SUBSET. register is absent because identity is already established;
// document and media tools are absent because free text is a poor way to decide what to
// overwrite; name_foods is absent because it edits shared reference data. What remains is
// reading a record and adding to it.
type Registry struct {
	deps    tools.Deps
	schemas []ToolSchema
	invoke  map[string]handler
}

func obj(props map[string]any, required ...string) map[string]any {
	if required == nil {
		required = []string{}
	}
	return map[string]any{"type": "object", "properties": props, "required": required}
}

func str(desc string) map[string]any  { return map[string]any{"type": "string", "description": desc} }
func num(desc string) map[string]any  { return map[string]any{"type": "number", "description": desc} }
func inte(desc string) map[string]any { return map[string]any{"type": "integer", "description": desc} }

func NewRegistry(d tools.Deps) *Registry {
	r := &Registry{deps: d, invoke: map[string]handler{}}

	// Note what is NOT in any schema below: `profile`. It is injected from the verified
	// channel identity, so the model has no argument through which to name anyone else.
	r.add("find_foods",
		"Search the Indonesian food table (TKPI 2020). Use this BEFORE logging any food, to get its exact code. Composite dishes like nasi goreng are not in the table — only ingredients.",
		obj(map[string]any{
			"query": str("food name in Bahasa or English, e.g. 'beras giling', 'tempe'"),
			"limit": inte("how many matches, default 5"),
		}, "query"),
		func(_ context.Context, p string, a json.RawMessage) (any, error) {
			var in tools.FindFoodsArgs
			_ = json.Unmarshal(a, &in)
			if in.Limit == 0 {
				in.Limit = 5
			}
			return tools.FindFoods(r.deps, in)
		})

	r.add("log_food",
		"Record something eaten. `food` must be a TKPI code from find_foods. `grams` is required and must be a real weight — never guess it; ask the person instead.",
		obj(map[string]any{
			"food":  str("TKPI code, e.g. AR001"),
			"grams": num("portion in grams"),
			"meal":  str("sarapan, makan siang, makan malam or snack"),
		}, "food", "grams"),
		func(_ context.Context, p string, a json.RawMessage) (any, error) {
			var in tools.LogFoodArgs
			_ = json.Unmarshal(a, &in)
			in.Profile = p
			return tools.LogFood(r.deps, in)
		})

	r.add("log_weight", "Record body weight in kilograms.",
		obj(map[string]any{"kg": num("weight in kg"), "note": str("optional")}, "kg"),
		func(_ context.Context, p string, a json.RawMessage) (any, error) {
			var in struct {
				Kg   float64 `json:"kg"`
				Note string  `json:"note"`
			}
			_ = json.Unmarshal(a, &in)
			if in.Kg <= 20 || in.Kg > 400 {
				return nil, fmt.Errorf("%.1f kg is outside any plausible body weight", in.Kg)
			}
			err := r.deps.DB.LogWeight(p, storage.WeightEntry{
				Date: today(), WeightKg: in.Kg, Note: in.Note})
			if err != nil {
				return nil, err
			}
			return map[string]any{"logged_kg": in.Kg, "date": today()}, nil
		})

	r.add("log_set",
		"Record a completed strength set. `exercise` must be an exact name from find_exercises.",
		obj(map[string]any{
			"exercise":  str("exact exercise name"),
			"sets":      inte("number of sets"),
			"reps":      inte("reps per set"),
			"weight_kg": num("load in kg; omit for body weight"),
		}, "exercise", "sets", "reps"),
		func(_ context.Context, p string, a json.RawMessage) (any, error) {
			var in struct {
				Exercise string  `json:"exercise"`
				Sets     int     `json:"sets"`
				Reps     int     `json:"reps"`
				WeightKg float64 `json:"weight_kg"`
			}
			_ = json.Unmarshal(a, &in)
			return tools.LogSet(r.deps, p, in.Exercise, in.Sets, in.Reps, in.WeightKg)
		})

	r.add("log_cardio", "Record a cardio session.",
		obj(map[string]any{
			"minutes":     num("duration in minutes"),
			"speed_kmh":   num("optional"),
			"incline_pct": num("optional"),
		}, "minutes"),
		func(_ context.Context, p string, a json.RawMessage) (any, error) {
			var in struct {
				Minutes    float64 `json:"minutes"`
				SpeedKmh   float64 `json:"speed_kmh"`
				InclinePct float64 `json:"incline_pct"`
			}
			_ = json.Unmarshal(a, &in)
			if in.Minutes <= 0 {
				return nil, fmt.Errorf("minutes must be greater than zero")
			}
			err := r.deps.DB.LogCardio(p, storage.CardioEntry{
				Date: today(), Minutes: in.Minutes,
				SpeedKmh: in.SpeedKmh, InclinePct: in.InclinePct})
			if err != nil {
				return nil, err
			}
			return map[string]any{"logged_minutes": in.Minutes, "date": today()}, nil
		})

	r.add("progress", "Training, cardio, food and weight over the last N days. Use this to answer 'how am I doing'.",
		obj(map[string]any{"days": inte("lookback window, default 14")}),
		func(_ context.Context, p string, a json.RawMessage) (any, error) {
			var in struct {
				Days int `json:"days"`
			}
			_ = json.Unmarshal(a, &in)
			if in.Days == 0 {
				in.Days = 14
			}
			return tools.Progress(r.deps, p, in.Days)
		})

	r.add("find_exercises", "Search exercises this person can actually perform, given their equipment and any stated injuries.",
		obj(map[string]any{
			"muscle": str("muscle or body part, e.g. chest"),
			"limit":  inte("default 8"),
		}),
		func(_ context.Context, p string, a json.RawMessage) (any, error) {
			var in struct {
				Muscle string `json:"muscle"`
				Limit  int    `json:"limit"`
			}
			_ = json.Unmarshal(a, &in)
			if in.Limit == 0 {
				in.Limit = 8
			}
			out, err := tools.FindExercises(r.deps, p, in.Muscle, in.Limit)
			return out, err
		})

	r.add("plan_session", "Build today's training session. Sets, reps and rest come from the software, not from you.",
		obj(map[string]any{
			"focus":   str("full, upper or lower; default full"),
			"minutes": inte("session length; defaults to their setting"),
		}),
		func(_ context.Context, p string, a json.RawMessage) (any, error) {
			var in struct {
				Focus   string `json:"focus"`
				Minutes int    `json:"minutes"`
			}
			_ = json.Unmarshal(a, &in)
			if in.Focus == "" {
				in.Focus = "full"
			}
			return tools.PlanSession(r.deps, p, in.Focus, in.Minutes)
		})

	r.add("get_profile", "This person's settings: goal, equipment, experience, stated limitations.",
		obj(map[string]any{}),
		func(_ context.Context, p string, a json.RawMessage) (any, error) {
			return r.deps.DB.GetProfile(p)
		})

	r.add("update_profile", "Save something you have learned about them. Only pass what is new or changing — never re-send a field just to repeat its current value.",
		obj(map[string]any{
			"equipment":         arr("what they can train with"),
			"goal":              str("fat_loss, strength, hypertrophy or general"),
			"experience":        str("beginner, intermediate or advanced"),
			"limitations":       arr("injuries or conditions, in their own words"),
			"age":               map[string]any{"type": "integer", "description": "years, 13 to 100"},
			"height_cm":         map[string]any{"type": "integer", "description": "height in centimetres"},
			"sex":               str("male, female or other"),
			"allergies":         arr("food allergies. Replaces the whole list, so include ones already known"),
			"dislikes":          arr("foods they will not eat. Replaces the whole list"),
			"diet_notes":        str("anything else about how they eat — puasa, an instruction from their dietitian"),
			"diet_preference":   str("vegetarian, non_vegetarian or vegan"),
			"sessions_per_week": map[string]any{"type": "integer", "description": "how many times a week they train, 1-14"},
			"session_minutes":   map[string]any{"type": "integer", "description": "how long one session runs, in minutes"},
			"declined":          arr("questions they were asked and chose not to answer, exactly as named in STILL UNKNOWN, so they are not asked again"),
		}),
		func(_ context.Context, p string, a json.RawMessage) (any, error) {
			var in tools.UpdateProfileArgs
			_ = json.Unmarshal(a, &in)
			in.Profile = p
			return tools.UpdateProfile(r.deps, in)
		})

	r.add("estimate_energy",
		"Estimate how much energy they use in a day — from their age, height, sex, "+
			"latest weight and training frequency. Returns a RANGE with its uncertainty, "+
			"and says what it had to assume. You may NOT do this arithmetic yourself; "+
			"call this. It refuses when an input is missing, and the right response to "+
			"that is to ask for the missing thing, never to assume it.",
		obj(map[string]any{}),
		func(_ context.Context, p string, _ json.RawMessage) (any, error) {
			return tools.EstimateEnergy(r.deps, p)
		})

	r.add("reset_assessment",
		"Start the questions over: clears equipment, goal, experience, injuries, age, "+
			"height, sex, allergies, dislikes and diet notes. Does NOT delete anything "+
			"they logged — training, food, weight and cardio all survive. Ask them to "+
			"confirm before you call it.",
		obj(map[string]any{}),
		func(_ context.Context, p string, a json.RawMessage) (any, error) {
			return tools.ResetAssessment(r.deps, p)
		})

	r.add("dashboard_link", "A link to their dashboard, valid for one hour.",
		obj(map[string]any{}),
		func(_ context.Context, p string, a json.RawMessage) (any, error) {
			return tools.DashboardLink(r.deps, tools.DashboardLinkArgs{Profile: p})
		})

	r.registerOffer()

	sort.Slice(r.schemas, func(i, j int) bool {
		return r.schemas[i].Function.Name < r.schemas[j].Function.Name
	})
	return r
}

// handler runs one tool. The context carries the things a tool may need but must never
// be told by the model: who this is, and whether the channel can draw buttons.
type handler func(ctx context.Context, profileID string, args json.RawMessage) (any, error)

func (r *Registry) add(name, desc string, params map[string]any, fn handler) {
	var s ToolSchema
	s.Type = "function"
	s.Function.Name = name
	s.Function.Description = desc
	s.Function.Parameters = params
	r.schemas = append(r.schemas, s)
	r.invoke[name] = fn
}

func (r *Registry) Schemas() []ToolSchema { return r.schemas }

func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.schemas))
	for _, s := range r.schemas {
		out = append(out, s.Function.Name)
	}
	return out
}

// Invoke runs a tool and returns JSON for the model.
//
// A refusal is returned as a RESULT, not an error: "that food is ambiguous, here are the
// codes" is exactly what the model needs to ask a better question. Turning it into a
// failure would make the model apologise instead of resolving it.
func (r *Registry) Invoke(ctx context.Context, profileID, name, rawArgs string) (result string) {
	// A model calls these with whatever it likes. One tool panicking on odd arguments
	// must not take down the conversation — or, in the Telegram case, the whole bot.
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("agent: tool %q panicked: %v", name, rec)
			result = jsonErr("that tool failed unexpectedly")
		}
	}()

	fn, ok := r.invoke[name]
	if !ok {
		return jsonErr(fmt.Sprintf("no such tool %q", name))
	}
	args := json.RawMessage(rawArgs)
	if len(args) == 0 {
		args = json.RawMessage(`{}`)
	}
	out, err := fn(ctx, profileID, args)
	if err != nil {
		return jsonErr(err.Error())
	}
	b, err := json.Marshal(out)
	if err != nil {
		return jsonErr("result could not be encoded")
	}
	return string(b)
}

func jsonErr(msg string) string {
	b, _ := json.Marshal(map[string]string{"refused": msg})
	return string(b)
}

func today() string { return time.Now().Format("2006-01-02") }

// arr is a string-list parameter. Enough of these accumulated to be worth a helper.
func arr(desc string) map[string]any {
	return map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": desc,
	}
}
