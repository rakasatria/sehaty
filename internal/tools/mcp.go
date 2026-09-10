package tools

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rakasatria/sehaty/internal/catalog"
	"github.com/rakasatria/sehaty/internal/storage"
)

// Input types are the tool schemas. The SDK derives JSON Schema from these structs, so
// the field tags and comments here are what an agent actually reads when deciding
// whether to call a tool — they are documentation, not decoration.

type ProfileArgs struct {
	Profile string `json:"profile" jsonschema:"the profile to act on"`
}

type PlanArgs struct {
	Profile string `json:"profile" jsonschema:"the profile to plan for"`
	Focus   string `json:"focus,omitempty" jsonschema:"full, upper or lower; defaults to full"`
	Minutes int    `json:"minutes,omitempty" jsonschema:"session length; defaults to the profile's setting"`
}

type LogSetArgs struct {
	Profile  string  `json:"profile"`
	Exercise string  `json:"exercise" jsonschema:"exact exercise name from find_exercises"`
	Sets     int     `json:"sets"`
	Reps     int     `json:"reps"`
	WeightKg float64 `json:"weight_kg,omitempty" jsonschema:"omit for body weight movements"`
}

type LogWeightArgs struct {
	Profile string  `json:"profile"`
	Kg      float64 `json:"kg"`
	Note    string  `json:"note,omitempty"`
}

type LogCardioArgs struct {
	Profile    string  `json:"profile"`
	Minutes    float64 `json:"minutes"`
	SpeedKmh   float64 `json:"speed_kmh,omitempty"`
	InclinePct float64 `json:"incline_pct,omitempty"`
}

type FindArgs struct {
	Profile string `json:"profile"`
	Muscle  string `json:"muscle,omitempty" jsonschema:"filter by muscle or body part"`
	Limit   int    `json:"limit,omitempty"`
}

type ProgressArgs struct {
	Profile string `json:"profile"`
	Days    int    `json:"days,omitempty" jsonschema:"lookback window; defaults to 14"`
}

type UpdateProfileArgs struct {
	Profile         string   `json:"profile" jsonschema:"the profile to update"`
	Equipment       []string `json:"equipment,omitempty" jsonschema:"what they can train with, e.g. body weight, dumbbell, cable, treadmill"`
	Goal            string   `json:"goal,omitempty" jsonschema:"fat_loss, strength, hypertrophy or general"`
	Experience      string   `json:"experience,omitempty" jsonschema:"beginner, intermediate or advanced; sets the difficulty ceiling"`
	SessionsPerWeek int      `json:"sessions_per_week,omitempty" jsonschema:"1-14"`
	SessionMinutes  int      `json:"session_minutes,omitempty" jsonschema:"10-180"`
	MaxDifficulty   int      `json:"max_difficulty,omitempty" jsonschema:"1-5; overrides the ceiling implied by experience"`
	Limitations     []string `json:"limitations,omitempty" jsonschema:"injuries or conditions in the person's own words, e.g. bad knee, rotator cuff injury. Replaces the whole list; send an empty array to clear it"`
}

type RegisterArgs struct {
	Channel    string `json:"channel" jsonschema:"telegram, access or mcp"`
	ExternalID string `json:"external_id" jsonschema:"the id on that channel"`
	Profile    string `json:"profile" jsonschema:"desired profile name, lowercase"`
	Passphrase string `json:"passphrase" jsonschema:"the registration passphrase"`
}

// Slice results are wrapped: an MCP output schema must be an object, and a bare slice
// renders as type ["null","array"], which clients reject.
type ProfilesOut struct {
	Profiles []storage.ProfileSummary `json:"profiles"`
	Count    int                      `json:"count"`
}

type ExercisesOut struct {
	Exercises []catalog.Exercise `json:"exercises"`
	Count     int                `json:"count"`
}

type CardioArgs struct {
	Protocol string `json:"protocol,omitempty" jsonschema:"zone2, incline, intervals or easy"`
}

// CARDIO protocols are programmed here rather than looked up: the exercise dataset holds
// exactly one treadmill entry, because treadmill work is a protocol question rather than
// an exercise-selection one.
var CARDIO = map[string]map[string]any{
	"zone2": {"minutes": 35, "speed_kmh": 5.5, "incline_pct": 6,
		"note": "Conversational pace. If you cannot speak a full sentence, slow down."},
	"incline": {"minutes": 30, "speed_kmh": 5.0, "incline_pct": 12,
		"note": "Steep and slow. Do not hold the rails — it removes most of the work."},
	"intervals": {"minutes": 22, "speed_kmh": 8.0, "incline_pct": 2,
		"note": "5 min easy, then 1 min hard / 2 min easy x5, then 2 min down."},
	"easy": {"minutes": 25, "speed_kmh": 4.5, "incline_pct": 3,
		"note": "Recovery. Deliberately easy."},
}

// ok wraps a value as a tool result. The SDK marshals the typed output; the empty
// CallToolResult lets it supply the default text representation.
func ok[T any](v T) (*mcp.CallToolResult, T, error) { return nil, v, nil }

// Register wires every deterministic tool onto the server.
//
// Descriptions matter more than they look: they are how an agent decides which tool to
// call, so each says what the tool does AND what it refuses to do.
func RegisterTools(s *mcp.Server, d Deps, passphrase string) {
	mcp.AddTool(s, &mcp.Tool{Name: "list_profiles",
		Description: "List every profile on this server. Returns names and goals only, never anyone's logs."},
		func(ctx context.Context, r *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ProfilesOut, error) {
			ps, err := d.DB.ListProfiles()
			if err != nil {
				return nil, ProfilesOut{}, err
			}
			out := make([]storage.ProfileSummary, 0, len(ps))
			for _, p := range ps {
				out = append(out, storage.ProfileSummary{ID: p.ID, Goal: p.Goal,
					Equipment: p.Equipment, SessionsPerWeek: p.SessionsPerWeek})
			}
			return ok(ProfilesOut{Profiles: out, Count: len(out)})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "get_profile",
		Description: "One profile's settings, plus how many exercises their equipment allows."},
		func(ctx context.Context, r *mcp.CallToolRequest, a ProfileArgs) (*mcp.CallToolResult, map[string]any, error) {
			p, err := d.DB.GetProfile(a.Profile)
			if err != nil {
				return nil, nil, err
			}
			return ok(map[string]any{"profile": p.ID, "goal": p.Goal,
				"equipment": p.Equipment, "experience": p.Experience,
				"max_difficulty": p.MaxDifficulty, "locale": p.Locale,
				"sessions_per_week":   p.SessionsPerWeek,
				"available_exercises": len(d.Cat.For(p.Equipment, p.MaxDifficulty)),
				"limitations":         p.Limitations,
				"equipment_options":   d.Cat.Equipment(),
				"prescription":        goals[p.Goal]})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "update_profile",
		Description: "Change a profile's equipment, goal, experience or session settings. Only the fields supplied are changed. Rejects unknown equipment rather than leaving someone with no exercises and no explanation."},
		func(ctx context.Context, r *mcp.CallToolRequest, a UpdateProfileArgs) (*mcp.CallToolResult, map[string]any, error) {
			p, err := UpdateProfile(d, a)
			if err != nil {
				return nil, nil, err
			}
			return ok(map[string]any{"profile": p.ID, "goal": p.Goal,
				"equipment": p.Equipment, "experience": p.Experience,
				"limitations":         p.Limitations,
				"max_difficulty":      p.MaxDifficulty,
				"sessions_per_week":   p.SessionsPerWeek,
				"session_minutes":     p.SessionMinutes,
				"available_exercises": len(d.Cat.For(p.Equipment, p.MaxDifficulty))})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "register",
		Description: "Bind a channel account to a profile, creating it if new. Requires the registration passphrase; without the correct passphrase nothing is created."},
		func(ctx context.Context, r *mcp.CallToolRequest, a RegisterArgs) (*mcp.CallToolResult, map[string]any, error) {
			id, err := Register(d, a.Channel, a.ExternalID, a.Profile, a.Passphrase, passphrase)
			if err != nil {
				return nil, nil, err
			}
			return ok(map[string]any{"profile": id, "channel": a.Channel})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "find_exercises",
		Description: "Search exercises this profile can actually perform. Never returns movements needing equipment they lack, or above their difficulty cap."},
		func(ctx context.Context, r *mcp.CallToolRequest, a FindArgs) (*mcp.CallToolResult, ExercisesOut, error) {
			if a.Limit == 0 {
				a.Limit = 10
			}
			ex, err := FindExercises(d, a.Profile, a.Muscle, a.Limit)
			if err != nil {
				return nil, ExercisesOut{}, err
			}
			return ok(ExercisesOut{Exercises: ex, Count: len(ex)})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "plan_session",
		Description: "Build today's resistance session. Rotates away from the last 10 days and is stable for the whole day."},
		func(ctx context.Context, r *mcp.CallToolRequest, a PlanArgs) (*mcp.CallToolResult, PlanOut, error) {
			if a.Focus == "" {
				a.Focus = "full"
			}
			p, err := PlanSession(d, a.Profile, a.Focus, a.Minutes)
			if err != nil {
				return nil, PlanOut{}, err
			}
			return ok(p)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "cardio_protocol",
		Description: "Treadmill protocol: zone2, incline, intervals or easy. zone2 is the default because it is the intensity people sustain."},
		func(ctx context.Context, r *mcp.CallToolRequest, a CardioArgs) (*mcp.CallToolResult, map[string]any, error) {
			if a.Protocol == "" {
				a.Protocol = "zone2"
			}
			c, found := CARDIO[a.Protocol]
			if !found {
				return nil, nil, fmt.Errorf("protocol must be zone2, incline, intervals or easy")
			}
			out := map[string]any{"protocol": a.Protocol}
			for k, v := range c {
				out[k] = v
			}
			return ok(out)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "log_set",
		Description: "Record a completed set. Refuses exercises outside the profile's equipment."},
		func(ctx context.Context, r *mcp.CallToolRequest, a LogSetArgs) (*mcp.CallToolResult, LogOut, error) {
			o, err := LogSet(d, a.Profile, a.Exercise, a.Sets, a.Reps, a.WeightKg)
			if err != nil {
				return nil, LogOut{}, err
			}
			return ok(o)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "log_cardio",
		Description: "Record a treadmill session."},
		func(ctx context.Context, r *mcp.CallToolRequest, a LogCardioArgs) (*mcp.CallToolResult, map[string]any, error) {
			err := d.DB.LogCardio(a.Profile, storage.CardioEntry{Date: today(),
				Minutes: a.Minutes, SpeedKmh: a.SpeedKmh, InclinePct: a.InclinePct})
			if err != nil {
				return nil, nil, err
			}
			return ok(map[string]any{"profile": a.Profile, "minutes": a.Minutes})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "log_weight",
		Description: "Record body weight and report the weekly trend. Day-to-day movement is water, not fat; only the weekly rate is signal."},
		func(ctx context.Context, r *mcp.CallToolRequest, a LogWeightArgs) (*mcp.CallToolResult, map[string]any, error) {
			if err := d.DB.LogWeight(a.Profile, storage.WeightEntry{Date: today(),
				WeightKg: a.Kg, Note: a.Note}); err != nil {
				return nil, nil, err
			}
			out := map[string]any{"profile": a.Profile, "logged_kg": a.Kg}
			if w, err := d.DB.Weights(a.Profile, 3650); err == nil && len(w) >= 2 {
				first, last := w[0], w[len(w)-1]
				t0, e0 := time.Parse("2006-01-02", first.Date)
				t1, e1 := time.Parse("2006-01-02", last.Date)
				if e0 == nil && e1 == nil {
					days := t1.Sub(t0).Hours() / 24
					if days < 1 {
						days = 1
					}
					out["change_kg"] = last.WeightKg - first.WeightKg
					out["kg_per_week"] = (last.WeightKg - first.WeightKg) / (days / 7)
					out["guide"] = "0.25-0.75 kg/week is sustainable; faster usually costs muscle"
				}
			}
			return ok(out)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "progress",
		Description: "Training, cardio, food and weight over N days. Reads the logs and invents nothing — absent data reports as absent."},
		func(ctx context.Context, r *mcp.CallToolRequest, a ProgressArgs) (*mcp.CallToolResult, ProgressOut, error) {
			if a.Days == 0 {
				a.Days = 14
			}
			p, err := Progress(d, a.Profile, a.Days)
			if err != nil {
				return nil, ProgressOut{}, err
			}
			return ok(p)
		})
}

// Serve runs the MCP server over Streamable HTTP.
//
// One server instance is shared across sessions: it holds no per-request state, and the
// database enforces isolation by profile_id rather than by connection.
func Serve(addr string, d Deps, passphrase string) error {
	srv := mcp.NewServer(&mcp.Implementation{Name: "sehaty", Version: "0.1.0"}, nil)
	RegisterTools(srv, d, passphrase)
	h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	mux := http.NewServeMux()
	mux.Handle("/mcp", h)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	return http.ListenAndServe(addr, mux)
}
