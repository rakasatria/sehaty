package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"
)

var ErrNoProfile = errors.New("no such profile")

// Profile ids become part of media paths and log keys, so they are constrained
// deliberately — an unchecked id here is a path-traversal bug.
var idRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,30}$`)

func ValidID(s string) bool { return idRe.MatchString(s) }

type Profile struct {
	ID              string
	Equipment       []string
	Goal            string
	SessionsPerWeek int
	SessionMinutes  int
	Experience      string
	MaxDifficulty   int
	// DisplayName is what a human is called. The ID is an opaque nanoid, so without this
	// every reply would address someone as "k3f9x2mq…".
	DisplayName string
	// Limitations are injuries and medical conditions stated by the person, in their own
	// words. They gate planning: see internal/guardrails.
	Limitations []string
	Locale      string

	// Who they are. Held for CONTEXT — so a plan can suit a 44-year-old rather than a
	// generic adult, and so a clinician reading this has the basics. NOT used to compute
	// calorie or macro targets: Sehaty refuses to set those regardless of what it knows.
	Age       int
	HeightCm  int
	Sex       string
	Allergies []string
	Dislikes  []string
	DietNotes string

	// DietPreference is vegetarian, non_vegetarian or vegan. Structured rather than left
	// to DietNotes because it is a filter — "never suggest a food this person will not
	// eat" is a lookup, and free text cannot be looked up reliably.
	DietPreference string

	// Answered names the questions they have actually responded to, including the ones
	// they declined. Without it, an answer equal to the default reads as no answer.
	Answered []string
}

func (d *DB) SaveProfile(p Profile) error {
	if !ValidID(p.ID) {
		return fmt.Errorf("invalid profile id %q", p.ID)
	}
	eq, err := json.Marshal(p.Equipment)
	if err != nil {
		return fmt.Errorf("marshal equipment: %w", err)
	}
	if p.Limitations == nil {
		p.Limitations = []string{} // never write SQL NULL into a NOT NULL column
	}
	lim, err := json.Marshal(p.Limitations)
	if err != nil {
		return fmt.Errorf("marshal limitations: %w", err)
	}
	if p.Allergies == nil {
		p.Allergies = []string{}
	}
	if p.Dislikes == nil {
		p.Dislikes = []string{}
	}
	alg, err := json.Marshal(p.Allergies)
	if err != nil {
		return fmt.Errorf("marshal allergies: %w", err)
	}
	dis, err := json.Marshal(p.Dislikes)
	if err != nil {
		return fmt.Errorf("marshal dislikes: %w", err)
	}
	if p.Answered == nil {
		p.Answered = []string{}
	}
	ans, err := json.Marshal(p.Answered)
	if err != nil {
		return fmt.Errorf("marshal answered: %w", err)
	}
	_, err = d.Exec(`
		INSERT INTO profile (id, equipment_json, goal, sessions_per_week,
		    session_minutes, experience, max_difficulty, locale, limitations_json,
		    display_name, age, height_cm, sex, allergies_json, dislikes_json,
		    diet_notes, diet_preference, answered_json, created_at)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
		    equipment_json=excluded.equipment_json, goal=excluded.goal,
		    sessions_per_week=excluded.sessions_per_week,
		    session_minutes=excluded.session_minutes,
		    experience=excluded.experience,
		    max_difficulty=excluded.max_difficulty, locale=excluded.locale,
		    limitations_json=excluded.limitations_json,
		    display_name=excluded.display_name, age=excluded.age,
		    height_cm=excluded.height_cm, sex=excluded.sex,
		    allergies_json=excluded.allergies_json,
		    dislikes_json=excluded.dislikes_json, diet_notes=excluded.diet_notes,
		    diet_preference=excluded.diet_preference,
		    answered_json=excluded.answered_json`,
		p.ID, string(eq), p.Goal, p.SessionsPerWeek, p.SessionMinutes,
		p.Experience, p.MaxDifficulty, p.Locale, string(lim), p.DisplayName,
		p.Age, p.HeightCm, p.Sex, string(alg), string(dis), p.DietNotes,
		p.DietPreference, string(ans),
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save profile %s: %w", p.ID, err)
	}
	return nil
}

type scanner interface{ Scan(...any) error }

func scanProfile(sc scanner) (Profile, error) {
	var p Profile
	var eq, lim, alg, dis, ans string
	err := sc.Scan(&p.ID, &eq, &p.Goal, &p.SessionsPerWeek, &p.SessionMinutes,
		&p.Experience, &p.MaxDifficulty, &p.Locale, &lim, &p.DisplayName,
		&p.Age, &p.HeightCm, &p.Sex, &alg, &dis, &p.DietNotes,
		&p.DietPreference, &ans)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal([]byte(eq), &p.Equipment); err != nil {
		return p, fmt.Errorf("profile %s: bad equipment json: %w", p.ID, err)
	}
	if err := json.Unmarshal([]byte(lim), &p.Limitations); err != nil {
		return p, fmt.Errorf("profile %s: bad limitations json: %w", p.ID, err)
	}
	if err := json.Unmarshal([]byte(alg), &p.Allergies); err != nil {
		return p, fmt.Errorf("profile %s: bad allergies json: %w", p.ID, err)
	}
	if err := json.Unmarshal([]byte(dis), &p.Dislikes); err != nil {
		return p, fmt.Errorf("profile %s: bad dislikes json: %w", p.ID, err)
	}
	if err := json.Unmarshal([]byte(ans), &p.Answered); err != nil {
		return p, fmt.Errorf("profile %s: bad answered json: %w", p.ID, err)
	}
	return p, nil
}

const profileCols = `id, equipment_json, goal, sessions_per_week, session_minutes,
	experience, max_difficulty, locale, limitations_json, display_name,
	age, height_cm, sex, allergies_json, dislikes_json, diet_notes,
	diet_preference, answered_json`

func (d *DB) GetProfile(id string) (Profile, error) {
	row := d.QueryRow(`SELECT `+profileCols+` FROM profile WHERE id=?`, id)
	p, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, fmt.Errorf("%q: %w", id, ErrNoProfile)
	}
	return p, err
}

func (d *DB) ListProfiles() ([]Profile, error) {
	rows, err := d.Query(`SELECT ` + profileCols + ` FROM profile ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Profile
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ProfileSummary is what list_profiles returns — identity and settings, never logs.
type ProfileSummary struct {
	ID              string   `json:"id"`
	DisplayName     string   `json:"display_name,omitempty"`
	Goal            string   `json:"goal"`
	Equipment       []string `json:"equipment"`
	SessionsPerWeek int      `json:"sessions_per_week"`
}

// Assessment is every question the record wants answered, in the order to ask.
//
// The order is not the order the fields sit in the struct. Equipment first: it answers in
// one second from a phone, has no sensitivity at all, and teaches the pattern that
// questions here are short and obviously practical. Injuries wait until a session or two
// exists to protect. Age waits until height has landed without friction.
//
// Schedule earns its place because a plan is built on it. It defaulted to three sessions
// of fifty minutes and was never once asked about — so every plan rested on two numbers
// nobody had confirmed.
var Assessment = []string{
	"equipment",
	"goal",
	"experience",
	"training schedule",
	"injuries or conditions",
	"height",
	"age",
	"diet preference",
	"food allergies",
	"sex",
}

// questionAliases maps the finer-grained names a UI may use onto the assessment question
// they answer.
//
// "Berapa kali seminggu, dan berapa lama?" is one question to a person and two sets of
// buttons on a phone, and the record should not care which way it arrived.
var questionAliases = map[string]string{
	"sessions per week": "training schedule",
	"session minutes":   "training schedule",
}

// Canonical resolves a question name to the assessment entry it answers.
func Canonical(q string) (string, bool) {
	if c, ok := questionAliases[q]; ok {
		return c, true
	}
	for _, v := range Assessment {
		if v == q {
			return q, true
		}
	}
	return "", false
}

// Missing lists the questions still unanswered, in the order to ask them.
//
// It reads the answered set, NOT the values. Inferring from values cannot work: someone
// who trains three times a week and someone who has never been asked both hold 3, so the
// second heuristic would ask them the same question every session forever. A declined
// question counts as answered — being asked once is a question, being asked every day is
// nagging.
func (p Profile) Missing() []string {
	answered := make(map[string]bool, len(p.Answered))
	for _, a := range p.Answered {
		answered[a] = true
	}
	out := make([]string, 0, len(Assessment))
	for _, q := range Assessment {
		if !answered[q] {
			out = append(out, q)
		}
	}
	return out
}

// MarkAnswered records that a question has been put and answered, declines included.
func (p *Profile) MarkAnswered(questions ...string) {
	have := make(map[string]bool, len(p.Answered))
	for _, a := range p.Answered {
		have[a] = true
	}
	for _, q := range questions {
		c, ok := Canonical(q)
		if !ok {
			continue
		}
		if !have[c] {
			p.Answered = append(p.Answered, c)
			have[c] = true
		}
	}
}

// Gated names the unknowns that must not be asked cold.
//
// Asked out of nowhere, these three are the ones that read as data harvesting rather than
// as a health record doing its job. Each has a moment that makes it obvious instead:
// height belongs beside a weight that just arrived, allergies beside food, and sex is
// usually settled in passing before it ever needs asking.
func Gated(field string) (moment string, ok bool) {
	switch field {
	case "height":
		return "only when a weight has just been logged", true
	case "food allergies":
		return "only when food is being logged", true
	case "diet preference":
		return "only when food is being logged — vegetarian, non-vegetarian or vegan", true
	case "sex":
		return "prefer never asking — it usually surfaces on its own. Ask only when a " +
			"reference range or an exercise choice makes it concretely relevant", true
	}
	return "", false
}
