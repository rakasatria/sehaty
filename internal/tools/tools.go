// Package tools exposes storage and catalog as MCP tools.
//
// Handlers are plain functions taking explicit arguments, and the MCP wiring is a thin
// shell over them. That means behaviour is testable without standing up a transport,
// which is the difference between a fast test suite and a slow one.
package tools

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/rakasatria/sehaty/internal/catalog"
	"github.com/rakasatria/sehaty/internal/crypto"
	"github.com/rakasatria/sehaty/internal/food"
	"github.com/rakasatria/sehaty/internal/guardrails"
	"github.com/rakasatria/sehaty/internal/media"
	"github.com/rakasatria/sehaty/internal/storage"
	"github.com/rakasatria/sehaty/internal/transcribe"
)

func today() string { return time.Now().Format("2006-01-02") }

// Deps is what every handler needs. Passed explicitly rather than held in package
// state so tests can build one per test with a temp database.
type Deps struct {
	DB       *storage.DB
	Cat      *catalog.Catalog
	Cipher   *crypto.Cipher // document bodies; never nil, the key is required at startup
	Media    media.Store    // voice notes and meal photos; consumed from Plan 3 onward
	Food     *food.Table    // Indonesian food composition (TKPI 2020)
	DashKey  []byte         // signs time-limited dashboard links
	DashBase string         // public base URL of the dashboard

	// Transcriber reads voice notes. Nil when unconfigured, and the tool says so rather
	// than failing obscurely — this is the only component that sends data off the machine.
	Transcriber *transcribe.Client
}

// Goal drives sets, reps and rest. Fat loss keeps density high; strength trades volume
// for load. These are prescriptions, not preferences, so they live in code rather than
// being asked of a model every time.
var goals = map[string]struct {
	Sets             int
	Reps, Rest, Note string
}{
	"fat_loss":    {3, "10-15", "45-60s", "Keep density high. Intake decides body composition, not volume."},
	"strength":    {4, "4-6", "2-3 min", "Heavy, long rests. Add load before reps."},
	"hypertrophy": {3, "8-12", "60-90s", "Most sets close to failure."},
	"general":     {3, "8-12", "60s", "Sustainable beats optimal."},
}

var blocks = map[string][]struct {
	Part string
	N    int
}{
	"full":  {{"upper legs", 2}, {"chest", 1}, {"back", 2}, {"shoulders", 1}, {"waist", 1}},
	"upper": {{"chest", 2}, {"back", 2}, {"shoulders", 2}, {"upper arms", 2}},
	"lower": {{"upper legs", 3}, {"lower legs", 1}, {"waist", 2}},
}

type PlanOut struct {
	Profile    string             `json:"profile"`
	Date       string             `json:"date"`
	Focus      string             `json:"focus"`
	Minutes    int                `json:"minutes"`
	Goal       string             `json:"goal"`
	Sets       int                `json:"sets"`
	Reps       string             `json:"reps"`
	Rest       string             `json:"rest"`
	Note       string             `json:"note"`
	Exercises  []catalog.Exercise `json:"exercises"`
	RotatedOut int                `json:"rotated_out"`
	// ExcludedForSafety summarises movements removed because of a stated limitation.
	// Reported rather than silently dropped: an exercise that vanishes without explanation
	// looks like a broken app, and invites the person to go do it unsupervised.
	ExcludedForSafety *guardrails.Summary `json:"excluded_for_safety,omitempty"`
}

// fnv is a small deterministic hash so a plan is stable for a whole day and different
// tomorrow. A plan that reshuffles on every call cannot be followed.
func fnv(s string) int64 {
	var h uint64 = 14695981039346656037
	for _, c := range []byte(s) {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return int64(h >> 1)
}

func PlanSession(d Deps, profileID, focus string, minutes int) (PlanOut, error) {
	blk, ok := blocks[focus]
	if !ok {
		return PlanOut{}, fmt.Errorf("focus must be full, upper or lower; got %q", focus)
	}
	p, err := requireProfile(d, profileID)
	if err != nil {
		return PlanOut{}, err
	}
	// Some situations are not "train around it" situations. Refuse before building
	// anything, so no plan exists to be partially followed.
	if ref, blocked := guardrails.Screen(p.Limitations); blocked {
		return PlanOut{}, errors.New(ref.Message)
	}
	if minutes <= 0 {
		minutes = p.SessionMinutes
	}
	recent := map[string]bool{}
	if sets, err := d.DB.Sets(profileID, 10); err == nil {
		for _, s := range sets {
			recent[strings.ToLower(s.Exercise)] = true
		}
	}
	// Contraindicated movements are removed from the pool BEFORE selection, so the
	// rotation logic never has to know about injuries and cannot accidentally pick one.
	usable, dropped := guardrails.Filter(p.Limitations, d.Cat.For(p.Equipment, p.MaxDifficulty))
	byPart := map[string][]catalog.Exercise{}
	for _, e := range usable {
		k := strings.ToLower(e.BodyPart)
		byPart[k] = append(byPart[k], e)
	}
	rng := rand.New(rand.NewSource(fnv(today() + focus + profileID)))
	var picked []catalog.Exercise
	for _, b := range blk {
		pool := byPart[b.Part]
		var fresh []catalog.Exercise
		for _, e := range pool {
			if !recent[strings.ToLower(e.Name)] {
				fresh = append(fresh, e)
			}
		}
		if len(fresh) == 0 {
			fresh = pool
		}
		rng.Shuffle(len(fresh), func(i, j int) { fresh[i], fresh[j] = fresh[j], fresh[i] })
		for i := 0; i < b.N && i < len(fresh); i++ {
			picked = append(picked, fresh[i])
		}
	}
	if capN := minutes / 5; capN >= 4 && len(picked) > capN {
		picked = picked[:capN]
	}
	g := goals[p.Goal]
	return PlanOut{Profile: profileID, Date: today(), Focus: focus, Minutes: minutes,
		Goal: p.Goal, Sets: g.Sets, Reps: g.Reps, Rest: g.Rest, Note: g.Note,
		Exercises: picked, RotatedOut: len(recent),
		ExcludedForSafety: guardrails.Summarise(dropped)}, nil
}

type LogOut struct {
	Profile  string  `json:"profile"`
	Exercise string  `json:"exercise"`
	Sets     int     `json:"sets"`
	Reps     int     `json:"reps"`
	WeightKg float64 `json:"weight_kg"`
	VolumeKg float64 `json:"volume_kg"`
}

func LogSet(d Deps, profileID, exercise string, sets, reps int, weightKg float64) (LogOut, error) {
	p, err := requireProfile(d, profileID)
	if err != nil {
		return LogOut{}, err
	}
	ex, ok := d.Cat.ByName(exercise)
	if !ok {
		return LogOut{}, fmt.Errorf("%q is not in the exercise catalog", exercise)
	}
	allowed := false
	for _, eq := range p.Equipment {
		if strings.EqualFold(eq, ex.Equipment) {
			allowed = true
			break
		}
	}
	if !allowed {
		return LogOut{}, fmt.Errorf("%q needs %s, which %s does not have",
			ex.Name, ex.Equipment, profileID)
	}
	vol := float64(sets*reps) * weightKg
	if err := d.DB.LogSet(profileID, storage.SetEntry{Date: today(), Exercise: ex.Name,
		BodyPart: ex.BodyPart, Sets: sets, Reps: reps, WeightKg: weightKg,
		VolumeKg: vol}); err != nil {
		return LogOut{}, err
	}
	return LogOut{Profile: profileID, Exercise: ex.Name, Sets: sets, Reps: reps,
		WeightKg: weightKg, VolumeKg: vol}, nil
}

type ProgressOut struct {
	Profile         string         `json:"profile"`
	Days            int            `json:"days"`
	LiftingSessions int            `json:"lifting_sessions"`
	TargetSessions  int            `json:"target_sessions"`
	SetsLogged      int            `json:"sets_logged"`
	TotalVolumeKg   float64        `json:"total_volume_kg"`
	CardioSessions  int            `json:"cardio_sessions"`
	CardioMinutes   float64        `json:"cardio_minutes"`
	FoodDaysLogged  int            `json:"food_days_logged"`
	MuscleCoverage  map[string]int `json:"muscle_coverage"`
	// Pointer, not float: no weigh-ins must read as "unknown", never as 0 kg. A
	// fabricated zero in a weight log is worse than an absent one.
	LatestWeightKg *float64 `json:"latest_weight_kg"`
	Note           string   `json:"note,omitempty"`
}

func Progress(d Deps, profileID string, days int) (ProgressOut, error) {
	p, err := requireProfile(d, profileID)
	if err != nil {
		return ProgressOut{}, err
	}
	sets, err := d.DB.Sets(profileID, days)
	if err != nil {
		return ProgressOut{}, err
	}
	out := ProgressOut{Profile: profileID, Days: days, MuscleCoverage: map[string]int{},
		TargetSessions: days * p.SessionsPerWeek / 7}
	dates := map[string]bool{}
	for _, s := range sets {
		dates[s.Date] = true
		out.TotalVolumeKg += s.VolumeKg
		out.MuscleCoverage[s.BodyPart]++
	}
	out.LiftingSessions, out.SetsLogged = len(dates), len(sets)
	if c, err := d.DB.Cardio(profileID, days); err == nil {
		out.CardioSessions = len(c)
		for _, x := range c {
			out.CardioMinutes += x.Minutes
		}
	}
	if f, err := d.DB.Foods(profileID, days); err == nil {
		fd := map[string]bool{}
		for _, x := range f {
			fd[x.Date] = true
		}
		out.FoodDaysLogged = len(fd)
	}
	if w, err := d.DB.Weights(profileID, days); err == nil && len(w) > 0 {
		kg := w[len(w)-1].WeightKg
		out.LatestWeightKg = &kg
	}
	if out.FoodDaysLogged == 0 && p.Goal == "fat_loss" {
		out.Note = "no food logged — for fat loss this is the half that decides it"
	}
	return out, nil
}

func FindExercises(d Deps, profileID, muscle string, limit int) ([]catalog.Exercise, error) {
	p, err := requireProfile(d, profileID)
	if err != nil {
		return nil, err
	}
	// Search obeys the same limits as prescription. Handing someone a movement through
	// search that plan_session would refuse to give them defeats the point.
	usable, _ := guardrails.Filter(p.Limitations, d.Cat.For(p.Equipment, p.MaxDifficulty))
	var out []catalog.Exercise
	for _, e := range usable {
		hay := strings.ToLower(e.Name + " " + e.BodyPart + " " + e.Target)
		if muscle != "" && !strings.Contains(hay, strings.ToLower(muscle)) {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// Register binds an external account to a profile, creating the profile if new.
// The passphrase is the registration gate — see spec §8c. It is compared in full and
// a mismatch reveals nothing about how close the attempt was.
func Register(d Deps, channel, externalID, displayName, passphrase, want string) (storage.Profile, error) {
	if want == "" {
		return storage.Profile{}, fmt.Errorf("registration is closed: no passphrase configured")
	}
	if passphrase != want {
		return storage.Profile{}, fmt.Errorf("incorrect passphrase")
	}
	if strings.TrimSpace(channel) == "" || strings.TrimSpace(externalID) == "" {
		return storage.Profile{}, fmt.Errorf("channel and external_id are both required")
	}

	// Idempotent: an account that is already linked gets its existing profile back. A
	// retry — or a second registration attempt by the same person — must not mint a
	// duplicate profile and orphan the first one's history.
	if existing, err := d.DB.ResolveIdentity(channel, externalID); err == nil {
		return d.DB.GetProfile(existing)
	}

	// The id is GENERATED, never chosen. The server has no authentication, so the id is
	// the access control: anything reaching the port can name any profile it can guess,
	// and a chosen name like "raka" is guessable.
	id, err := storage.NewProfileID()
	if err != nil {
		return storage.Profile{}, err
	}
	name := strings.TrimSpace(displayName)
	if name == "" {
		name = "Someone"
	}
	p := freshProfile(id, name)
	if err := d.DB.SaveProfile(p); err != nil {
		return storage.Profile{}, err
	}
	if err := d.DB.LinkIdentity(channel, externalID, id); err != nil {
		return storage.Profile{}, err
	}
	return p, nil
}

// experienceCap maps stated experience onto a difficulty ceiling. Someone calling
// themselves a beginner should not have to know what "max_difficulty 3" means.
var experienceCap = map[string]int{"beginner": 3, "intermediate": 4, "advanced": 5}

// nonCatalogEquipment is accepted even though no dataset exercise uses it.
//
// The treadmill is real equipment Raka owns, but cardio is served by cardio_protocol and
// log_cardio rather than the exercise catalog, so it appears in no exercise record.
// Rejecting it as "unknown" would be technically true and practically absurd.
var nonCatalogEquipment = map[string]bool{"treadmill": true}

// UpdateProfile changes ONLY the fields supplied.
//
// A zero value means "leave alone", which is why this cannot be done by writing a whole
// Profile: a caller setting just equipment would otherwise silently blank the goal and
// session length. Registration deliberately creates a minimal profile, so without this
// tool a person is stuck as a body-weight beginner forever.
func UpdateProfile(d Deps, a UpdateProfileArgs) (storage.Profile, error) {
	p, err := requireProfile(d, a.Profile)
	if err != nil {
		return p, err
	}

	if len(a.Equipment) > 0 {
		known := make(map[string]bool, 40)
		for _, e := range d.Cat.Equipment() {
			known[e] = true
		}
		clean := make([]string, 0, len(a.Equipment))
		for _, raw := range a.Equipment {
			k := strings.ToLower(strings.TrimSpace(raw))
			if !known[k] && !nonCatalogEquipment[k] {
				return p, fmt.Errorf("unknown equipment %q; the dataset uses: %s",
					raw, strings.Join(d.Cat.Equipment(), ", "))
			}
			clean = append(clean, k)
		}
		p.Equipment = clean
	}

	// Limitations are replaced wholesale, not merged: recovering from an injury has to be
	// expressible, and a merge-only field could never be cleared.
	if a.Limitations != nil {
		clean := make([]string, 0, len(a.Limitations))
		for _, l := range a.Limitations {
			if t := strings.TrimSpace(l); t != "" {
				clean = append(clean, t)
			}
		}
		p.Limitations = clean
	}

	if a.Goal != "" {
		if _, ok := goals[a.Goal]; !ok {
			return p, fmt.Errorf("unknown goal %q; use fat_loss, strength, hypertrophy or general", a.Goal)
		}
		p.Goal = a.Goal
	}

	if a.Experience != "" {
		ceiling, ok := experienceCap[a.Experience]
		if !ok {
			return p, fmt.Errorf("unknown experience %q; use beginner, intermediate or advanced", a.Experience)
		}
		p.Experience = a.Experience
		// Experience implies a ceiling, but an explicit max_difficulty in the same call wins.
		if a.MaxDifficulty == 0 {
			p.MaxDifficulty = ceiling
		}
	}
	if a.MaxDifficulty != 0 {
		if a.MaxDifficulty < 1 || a.MaxDifficulty > 5 {
			return p, fmt.Errorf("max_difficulty must be 1..5, got %d", a.MaxDifficulty)
		}
		p.MaxDifficulty = a.MaxDifficulty
	}
	if a.SessionsPerWeek != 0 {
		if a.SessionsPerWeek < 1 || a.SessionsPerWeek > 14 {
			return p, fmt.Errorf("sessions_per_week must be 1..14, got %d", a.SessionsPerWeek)
		}
		p.SessionsPerWeek = a.SessionsPerWeek
	}
	if a.Age != 0 {
		if a.Age < 13 || a.Age > 100 {
			return p, fmt.Errorf("age must be between 13 and 100, got %d", a.Age)
		}
		p.Age = a.Age
	}
	if a.HeightCm != 0 {
		if a.HeightCm < 100 || a.HeightCm > 230 {
			return p, fmt.Errorf("height must be between 100 and 230 cm, got %d", a.HeightCm)
		}
		p.HeightCm = a.HeightCm
	}
	if a.Sex != "" {
		sx := strings.ToLower(strings.TrimSpace(a.Sex))
		if sx != "male" && sx != "female" && sx != "other" {
			return p, fmt.Errorf("sex must be male, female or other")
		}
		p.Sex = sx
	}
	if a.Allergies != nil {
		p.Allergies = clean(a.Allergies)
	}
	if a.Dislikes != nil {
		p.Dislikes = clean(a.Dislikes)
	}
	if a.DietNotes != "" {
		p.DietNotes = strings.TrimSpace(a.DietNotes)
	}
	if a.DietPreference != "" {
		dp := strings.ToLower(strings.TrimSpace(a.DietPreference))
		if dp != "vegetarian" && dp != "non_vegetarian" && dp != "vegan" {
			return p, fmt.Errorf("diet_preference must be vegetarian, non_vegetarian or vegan")
		}
		p.DietPreference = dp
	}

	// A question that was put and turned down is answered. Asking it again tomorrow is
	// how an assistant stops being useful and starts being a form that follows you around.
	for _, q := range a.Declined {
		if _, ok := storage.Canonical(q); !ok {
			return p, fmt.Errorf("declined %q is not one of: %s",
				q, strings.Join(storage.Assessment, ", "))
		}
		p.MarkAnswered(q)
	}
	p.MarkAnswered(answeredBy(a)...)
	if a.SessionMinutes != 0 {
		if a.SessionMinutes < 10 || a.SessionMinutes > 180 {
			return p, fmt.Errorf("session_minutes must be 10..180, got %d", a.SessionMinutes)
		}
		p.SessionMinutes = a.SessionMinutes
	}

	if err := d.DB.SaveProfile(p); err != nil {
		return p, err
	}
	return p, nil
}

// Available counts what a profile can genuinely be prescribed: equipment, difficulty AND
// limitations. Reporting the pre-guardrail figure told someone with a knee injury they had
// 586 exercises when hundreds had already been ruled out — a number that reads as
// reassurance and is simply wrong.
func Available(d Deps, p storage.Profile) int {
	usable, _ := guardrails.Filter(p.Limitations, d.Cat.For(p.Equipment, p.MaxDifficulty))
	return len(usable)
}

// requireProfile resolves a profile and, when it does not exist, says which ones do.
//
// The profile argument is a PRIVACY BOUNDARY between different people's health data, and
// it arrives as a string chosen by a language model. A model can typo it or carry a stale
// one from earlier in a conversation. An opaque failure invites a retry with the same bad
// value; naming the registered profiles turns it into a one-step self-correction.
//
// This is a guard, not authentication — the server has none, so anything that can reach
// the port can name any profile. See docs/design for that gap.
func requireProfile(d Deps, id string) (storage.Profile, error) {
	p, err := d.DB.GetProfile(id)
	if err == nil {
		return p, nil
	}
	if !errors.Is(err, storage.ErrNoProfile) {
		return p, err
	}
	all, lerr := d.DB.ListProfiles()
	if lerr != nil || len(all) == 0 {
		return p, fmt.Errorf("unknown profile %q; none are registered yet — use register first", id)
	}
	names := make([]string, 0, len(all))
	for _, x := range all {
		names = append(names, x.ID)
	}
	return p, fmt.Errorf("unknown profile %q; registered profiles are: %s",
		id, strings.Join(names, ", "))
}

// clean drops blanks and trims. A list arriving from a conversation is full of stray
// whitespace and the occasional empty string.
func clean(in []string) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		if t := strings.TrimSpace(v); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// freshProfile is what someone looks like before they have told us anything.
//
// One definition, two callers: registration, and ResetAssessment. When they drifted apart
// a reset would leave a profile in a state registration never produces.
func freshProfile(id, name string) storage.Profile {
	return storage.Profile{ID: id, DisplayName: name,
		Equipment: []string{"body weight"}, Goal: "general", SessionsPerWeek: 3,
		SessionMinutes: 50, Experience: "beginner", MaxDifficulty: 3}
}

// ResetAssessment puts the profile answers back to their starting state so the questions
// can be asked again.
//
// It clears ANSWERS ONLY. Training, food, weight, cardio, documents and media are all
// untouched, and so are the identity link and any issued tokens — this is somebody
// redoing an interview, not somebody erasing their history. That distinction is the whole
// reason this is safe enough for the assistant to call at all: the worst case is that a
// person answers eight questions again.
//
// Deleting the record itself remains sehatyctl's job, where a human runs it and the blast
// radius is visible.
func ResetAssessment(d Deps, profileID string) (storage.Profile, error) {
	p, err := requireProfile(d, profileID)
	if err != nil {
		return storage.Profile{}, err
	}
	fresh := freshProfile(p.ID, p.DisplayName)
	fresh.Locale = p.Locale
	if err := d.DB.SaveProfile(fresh); err != nil {
		return storage.Profile{}, err
	}
	return fresh, nil
}

// answeredBy maps the fields this call actually set onto the questions they answer.
//
// It runs after validation, so a rejected value never marks its question answered — a
// person who said "seratus tahun" has not answered the age question, and must be asked
// again rather than left permanently unknown and permanently unasked.
func answeredBy(a UpdateProfileArgs) []string {
	var out []string
	if len(a.Equipment) > 0 {
		out = append(out, "equipment")
	}
	if a.Goal != "" {
		out = append(out, "goal")
	}
	if a.Experience != "" {
		out = append(out, "experience")
	}
	if a.SessionsPerWeek != 0 || a.SessionMinutes != 0 {
		out = append(out, "training schedule")
	}
	if a.Limitations != nil {
		out = append(out, "injuries or conditions")
	}
	if a.HeightCm != 0 {
		out = append(out, "height")
	}
	if a.Age != 0 {
		out = append(out, "age")
	}
	if a.DietPreference != "" {
		out = append(out, "diet preference")
	}
	if a.Allergies != nil {
		out = append(out, "food allergies")
	}
	if a.Sex != "" {
		out = append(out, "sex")
	}
	return out
}
