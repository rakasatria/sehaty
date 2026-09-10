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
	Locale          string
}

func (d *DB) SaveProfile(p Profile) error {
	if !ValidID(p.ID) {
		return fmt.Errorf("invalid profile id %q", p.ID)
	}
	eq, err := json.Marshal(p.Equipment)
	if err != nil {
		return fmt.Errorf("marshal equipment: %w", err)
	}
	_, err = d.Exec(`
		INSERT INTO profile (id, equipment_json, goal, sessions_per_week,
		    session_minutes, experience, max_difficulty, locale, created_at)
		VALUES (?,?,?,?,?,?,?,?,?)
		ON CONFLICT(id) DO UPDATE SET
		    equipment_json=excluded.equipment_json, goal=excluded.goal,
		    sessions_per_week=excluded.sessions_per_week,
		    session_minutes=excluded.session_minutes,
		    experience=excluded.experience,
		    max_difficulty=excluded.max_difficulty, locale=excluded.locale`,
		p.ID, string(eq), p.Goal, p.SessionsPerWeek, p.SessionMinutes,
		p.Experience, p.MaxDifficulty, p.Locale,
		time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save profile %s: %w", p.ID, err)
	}
	return nil
}

type scanner interface{ Scan(...any) error }

func scanProfile(sc scanner) (Profile, error) {
	var p Profile
	var eq string
	err := sc.Scan(&p.ID, &eq, &p.Goal, &p.SessionsPerWeek, &p.SessionMinutes,
		&p.Experience, &p.MaxDifficulty, &p.Locale)
	if err != nil {
		return p, err
	}
	if err := json.Unmarshal([]byte(eq), &p.Equipment); err != nil {
		return p, fmt.Errorf("profile %s: bad equipment json: %w", p.ID, err)
	}
	return p, nil
}

const profileCols = `id, equipment_json, goal, sessions_per_week, session_minutes,
	experience, max_difficulty, locale`

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
