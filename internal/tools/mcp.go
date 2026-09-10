package tools

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/rakasatria/sehaty/internal/catalog"
	"github.com/rakasatria/sehaty/internal/media"
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
	Channel     string `json:"channel" jsonschema:"telegram, access or mcp"`
	ExternalID  string `json:"external_id" jsonschema:"the id on that channel"`
	DisplayName string `json:"display_name" jsonschema:"what this person is called, e.g. Raka. The profile id is generated, not chosen"`
	Passphrase  string `json:"passphrase" jsonschema:"the registration passphrase"`
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
		Annotations: annRead(),
		Description: "List every profile on this server. Returns names and goals only, never anyone's logs."},
		func(ctx context.Context, r *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ProfilesOut, error) {
			ps, err := d.DB.ListProfiles()
			if err != nil {
				return nil, ProfilesOut{}, err
			}
			out := make([]storage.ProfileSummary, 0, len(ps))
			for _, p := range ps {
				// The display name is the point of having one: an opaque id alone
				// would force every reply to address someone as "n1cmykaqax3g…".
				out = append(out, storage.ProfileSummary{ID: p.ID, DisplayName: p.DisplayName,
					Goal: p.Goal, Equipment: p.Equipment, SessionsPerWeek: p.SessionsPerWeek})
			}
			return ok(ProfilesOut{Profiles: out, Count: len(out)})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "get_profile",
		Annotations: annRead(),
		Description: "One profile's settings, plus how many exercises their equipment allows."},
		func(ctx context.Context, r *mcp.CallToolRequest, a ProfileArgs) (*mcp.CallToolResult, map[string]any, error) {
			p, err := requireProfile(d, a.Profile)
			if err != nil {
				return nil, nil, err
			}
			return ok(map[string]any{"profile": p.ID, "goal": p.Goal,
				"equipment": p.Equipment, "experience": p.Experience,
				"max_difficulty": p.MaxDifficulty, "locale": p.Locale,
				"display_name":        p.DisplayName,
				"sessions_per_week":   p.SessionsPerWeek,
				"available_exercises": Available(d, p),
				"limitations":         p.Limitations,
				"equipment_options":   d.Cat.Equipment(),
				"prescription":        goals[p.Goal]})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "update_profile",
		Annotations: annOverwrite(),
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
				"available_exercises": Available(d, p)})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "register",
		Annotations: annAdd(),
		Description: "Register a person and get back their generated profile id. Call this once per person per channel, passing the passphrase they were given and the name they should be called. Do NOT choose the profile id — it is generated and unguessable, because the server has no authentication and a guessable id would be the only thing protecting one person's health data from another. Returns the profile id to use in every other tool; calling it again for an account that is already registered returns the same profile rather than creating a second."},
		func(ctx context.Context, r *mcp.CallToolRequest, a RegisterArgs) (*mcp.CallToolResult, map[string]any, error) {
			p, err := Register(d, a.Channel, a.ExternalID, a.DisplayName, a.Passphrase, passphrase)
			if err != nil {
				return nil, nil, err
			}
			return ok(map[string]any{"profile": p.ID, "display_name": p.DisplayName,
				"channel": a.Channel,
				"note": "Use this profile id in every other tool. It is generated and " +
					"unguessable — record it; there is no way to look it up by name."})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "find_exercises",
		Annotations: annRead(),
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
		Annotations: annRead(),
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
		Annotations: annRead(),
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
		Annotations: annAdd(),
		Description: "Record a completed set. Refuses exercises outside the profile's equipment."},
		func(ctx context.Context, r *mcp.CallToolRequest, a LogSetArgs) (*mcp.CallToolResult, LogOut, error) {
			o, err := LogSet(d, a.Profile, a.Exercise, a.Sets, a.Reps, a.WeightKg)
			if err != nil {
				return nil, LogOut{}, err
			}
			return ok(o)
		})

	mcp.AddTool(s, &mcp.Tool{Name: "log_cardio",
		Annotations: annAdd(),
		Description: "Record a treadmill session."},
		func(ctx context.Context, r *mcp.CallToolRequest, a LogCardioArgs) (*mcp.CallToolResult, map[string]any, error) {
			err := d.DB.LogCardio(a.Profile, storage.CardioEntry{Date: today(),
				Minutes: a.Minutes, SpeedKmh: a.SpeedKmh, InclinePct: a.InclinePct})
			if err != nil {
				return nil, nil, err
			}
			return ok(map[string]any{"profile": a.Profile, "minutes": a.Minutes})
		})

	mcp.AddTool(s, &mcp.Tool{Name: "attach_media",
		Annotations: annAdd(),
		Description: "Store a meal photo or a voice note and get back its content hash. Call this when someone sends a picture of what they ate, or a spoken note, before logging the meal. Does NOT read or interpret the file — nothing transcribes voice yet, and macros still come from the food table, never from a photo. Returns the hash to pass to log_food as `photo`; the same file sent twice stores once."},
		func(ctx context.Context, r *mcp.CallToolRequest, a AttachMediaArgs) (*mcp.CallToolResult, AttachMediaOut, error) {
			out, err := AttachMedia(d, a)
			if err != nil {
				return nil, AttachMediaOut{}, err
			}
			return nil, out, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_media",
		Annotations: annRead(),
		Description: "List the photos or voice notes stored for one profile, newest first. Call this to find a hash when someone refers to a picture they sent earlier. Does NOT return the files themselves, only their hashes and sizes. Returns nothing for another person's profile — media is scoped per profile and a hash alone will not open it."},
		func(ctx context.Context, r *mcp.CallToolRequest, a ListMediaArgs) (*mcp.CallToolResult, ListMediaOut, error) {
			out, err := ListMedia(d, a)
			if err != nil {
				return nil, ListMediaOut{}, err
			}
			return nil, out, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "get_media",
		Annotations: annRead(),
		Description: "Fetch a stored photo or voice note by its hash and return the file itself, so you can look at the meal. Call this when you need to see a picture the person sent. Do NOT use what you see to invent nutrition numbers — identify the dish and confirm the portion, then get the macros from find_foods. Returns the image or audio inline; a hash belonging to another profile will not open, because each file is cryptographically bound to its owner."},
		func(ctx context.Context, r *mcp.CallToolRequest, a GetMediaArgs) (*mcp.CallToolResult, struct{}, error) {
			raw, kind, err := GetMediaBytes(d, a)
			if err != nil {
				return nil, struct{}{}, err
			}
			mime := sniffMIME(raw, kind)
			var content mcp.Content
			if kind == media.KindVoice {
				content = &mcp.AudioContent{Data: raw, MIMEType: mime}
			} else {
				content = &mcp.ImageContent{Data: raw, MIMEType: mime}
			}
			return &mcp.CallToolResult{Content: []mcp.Content{content}}, struct{}{}, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "list_documents",
		Annotations: annRead(),
		Description: "List a profile's stored documents — the nutritionist's prescription, clinical notes, injury history — with their current version numbers. Call this BEFORE writing any document, so you reuse an existing key instead of inventing a second name for the same thing. Does NOT return document bodies; use get_document for that. Returns each document's key, title, current version and last-updated time, plus suggested keys for documents this profile does not have yet."},
		func(ctx context.Context, r *mcp.CallToolRequest, a ProfileArgs) (*mcp.CallToolResult, ListDocumentsOut, error) {
			out, err := ListDocuments(d, a.Profile)
			if err != nil {
				return nil, ListDocumentsOut{}, err
			}
			return nil, out, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "get_document",
		Annotations: annRead(),
		Description: "Read one document, current version by default or a specific earlier version. Call this whenever you need what a document actually says, and always immediately before writing changes back to it. Do NOT rely on a copy you read earlier in the conversation — it may have moved. Returns the markdown body, the version number, and the expected_version value to pass when writing your changes."},
		func(ctx context.Context, r *mcp.CallToolRequest, a GetDocumentArgs) (*mcp.CallToolResult, GetDocumentOut, error) {
			out, err := GetDocument(d, a)
			if err != nil {
				return nil, GetDocumentOut{}, err
			}
			return nil, out, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "put_document",
		Annotations: annAdd(),
		Description: "Store a document as a NEW VERSION, keeping every earlier version readable. Call this to record a nutritionist's prescription, clinical notes or injury history, passing create_new for a document that does not exist yet. Do NOT write without first reading: updating an existing document requires expected_version from get_document, so a stale copy cannot overwrite newer content. Returns the new version number; a near-duplicate key or a stale expected_version is refused with an explanation rather than silently applied."},
		func(ctx context.Context, r *mcp.CallToolRequest, a PutDocumentArgs) (*mcp.CallToolResult, PutDocumentOut, error) {
			out, err := PutDocument(d, a)
			if err != nil {
				return nil, PutDocumentOut{}, err
			}
			return nil, out, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "document_history",
		Annotations: annRead(),
		Description: "List every stored version of one document, newest first. Call this to answer questions about what a document used to say — what the nutritionist prescribed in September, before the revision. Does NOT return bodies; pass a version number to get_document to read one. Returns each version's number, title and timestamp."},
		func(ctx context.Context, r *mcp.CallToolRequest, a GetDocumentArgs) (*mcp.CallToolResult, DocumentHistoryOut, error) {
			out, err := DocumentHistory(d, a.Profile, a.Key)
			if err != nil {
				return nil, DocumentHistoryOut{}, err
			}
			return nil, out, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "find_foods",
		Annotations: annRead(),
		Description: "Search the Indonesian food composition table (TKPI 2020) and return per-100g energy and macros for each match. Call this when someone names a food and you need its exact table entry before logging it, or when they ask what is in a food. Do NOT use it for composite dishes — the table lists ingredients, so nasi goreng and gado-gado are absent and must be logged as their parts. Returns each food's code, name, per-100g macros, its original source citation, and any verification flag meaning the value could not be confirmed across sources."},
		func(ctx context.Context, r *mcp.CallToolRequest, a FindFoodsArgs) (*mcp.CallToolResult, FoodsOut, error) {
			out, err := FindFoods(d, a)
			if err != nil {
				return nil, FoodsOut{}, err
			}
			return nil, out, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "name_foods",
		Annotations: annAdd(),
		Description: "Record English names for Indonesian foods, and get the next batch that still needs naming. Call it with no entries to fetch work, then call it again with the names you worked out; repeat until remaining reaches zero. Send an EMPTY name_en for foods with no English equivalent such as oncom or gembus — that is a real answer and stops them being asked about again; do NOT invent a literal translation. Returns how many were saved, any codes that do not exist, and the next batch, so you can work through the table without holding all 1,142 foods at once."},
		func(ctx context.Context, r *mcp.CallToolRequest, a NameFoodsArgs) (*mcp.CallToolResult, NameFoodsOut, error) {
			out, err := NameFoods(d, a)
			if err != nil {
				return nil, NameFoodsOut{}, err
			}
			return nil, out, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "log_food",
		Annotations: annAdd(),
		Description: "Record something eaten, scaling the table's per-100g values to the portion given in grams. Call this when someone says what they ate; pass a TKPI code from find_foods when the name is not unique. Do NOT invent a food or a portion — an unrecognised name is refused rather than guessed, and an ambiguous one comes back with the candidate codes for you to choose from. Returns the saved entry with its macros and provenance; an identical entry already logged today is treated as a retry and NOT logged twice unless allow_duplicate is set."},
		func(ctx context.Context, r *mcp.CallToolRequest, a LogFoodArgs) (*mcp.CallToolResult, LogFoodOut, error) {
			out, err := LogFood(d, a)
			if err != nil {
				return nil, LogFoodOut{}, err
			}
			return nil, out, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "log_weight",
		Annotations: annAdd(),
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
		Annotations: annRead(),
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
	// The SDK defaults MaxRequestBodyBytes to 4 MiB. Base64 inflates a file by 4/3, so
	// that default rejected any photo over roughly 3 MB with an opaque HTTP 413 — while
	// the media store itself accepts 25 MiB. A real phone photo is 2-5 MB, so the
	// feature was broken for its actual purpose.
	//
	// Derived from the media cap rather than hardcoded, so the two cannot drift. The
	// APPLICATION limit should be what refuses an oversized upload, with a message
	// naming the limit; the transport limit is only a backstop.
	h := mcp.NewStreamableHTTPHandler(
		func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{MaxRequestBodyBytes: MaxRequestBody})
	mux := http.NewServeMux()
	mux.Handle("/mcp", h)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})
	return http.ListenAndServe(addr, mux)
}

// Annotation defaults in the spec are PESSIMISTIC: an unannotated tool is assumed
// non-read-only, destructive, non-idempotent and open-world. Leaving the read tools
// unannotated therefore advertises them as dangerous, which costs real capability — some
// clients parallelise read-only tools and relax approval prompts for them.
//
// Every tool here is closed-world: the server touches only its own store.
// MaxRequestBody is the transport ceiling: the largest blob the media store will accept,
// grown by the 4/3 base64 expansion, plus room for the JSON envelope around it.
const MaxRequestBody = media.DefaultMaxBytes*4/3 + (2 << 20)

func annRead() *mcp.ToolAnnotations {
	f := false
	return &mcp.ToolAnnotations{ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &f}
}

// annAdd marks a tool that only ever ADDS. put_document qualifies because it writes a new
// version and leaves every earlier one readable — versioning is what earns the claim.
func annAdd() *mcp.ToolAnnotations {
	f := false
	return &mcp.ToolAnnotations{DestructiveHint: &f, OpenWorldHint: &f}
}

// annOverwrite marks a tool that replaces existing values in place.
func annOverwrite() *mcp.ToolAnnotations {
	t, f := true, false
	return &mcp.ToolAnnotations{DestructiveHint: &t, IdempotentHint: true, OpenWorldHint: &f}
}
