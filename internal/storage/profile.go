package storage

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/rakasatria/sehaty/internal/capability"
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

// Have is what is actually on file about this person, as the capability registry counts
// it. weighed comes from the weight log, which is a fact rather than an answer.
//
// Only four fields can be told from their own zero value; for the rest, a default that
// is also a valid answer makes the value useless as evidence, so having been asked is
// the only signal there is. See capability.Question.Detectable.
func (p Profile) Have(weighed bool) capability.Have {
	h := capability.Have{}
	if weighed {
		h[capability.FieldWeightLog] = true
	}
	asked := p.Asked()
	for _, q := range capability.AskOrder() {
		if !q.Detectable {
			h[q.Field] = asked[q.Field]
			continue
		}
		switch q.Field {
		case capability.FieldAge:
			h[q.Field] = p.Age > 0
		case capability.FieldHeight:
			h[q.Field] = p.HeightCm > 0
		case capability.FieldSex:
			h[q.Field] = p.Sex != ""
		}
	}
	return h
}

// Asked is every question that has been put to this person, declines included.
func (p Profile) Asked() capability.Asked {
	a := capability.Asked{}
	for _, q := range p.Answered {
		if f, ok := capability.Canonical(q); ok {
			a[f] = true
		}
	}
	return a
}

// Missing lists the questions still worth putting, in the order to put them.
//
// It reads the answered set, NOT the values, for every field whose default is also a
// valid answer. Inferring from those cannot work: someone who trains three times a week
// and someone who has never been asked both hold 3, so the value heuristic would ask the
// second question forever. A declined question counts as answered — being asked once is
// a question, being asked every day is nagging.
func (p Profile) Missing() []string {
	// weighed is false here deliberately: Missing answers "what should I ask", and
	// nobody is asked for a weight — log_weight settles it. Passing false cannot add
	// weight_log to the result because it is not one of the questions.
	out := []string{}
	for _, r := range capability.ToAsk(p.Have(false), p.Asked()) {
		out = append(out, string(r.Field))
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
		f, ok := capability.Canonical(q)
		if !ok {
			continue
		}
		if !have[string(f)] {
			p.Answered = append(p.Answered, string(f))
			have[string(f)] = true
		}
	}
}
