package storage

import (
	"fmt"
	"strings"
	"time"
)

// A per-person schedule board.
//
// Sehaty is otherwise entirely reactive: nothing happens unless somebody opens
// Telegram and types. That is what makes it feel like a chatbot rather than a
// process — a practitioner has a cadence, and comes to you.
//
// Every row is opt-in, owned by one profile, and carries its own timezone, because
// "07:00" means nothing on a server that thinks in UTC and a person who lives in
// Jakarta.

// Reminder is one scheduled touch.
type Reminder struct {
	ID       int64
	Profile  string
	Kind     string // checkin, weigh_in, review
	Hour     int    // 0-23, in Zone
	Minute   int
	Weekday  int    // 0-6 Sunday..Saturday; -1 means every day
	Zone     string // IANA name, e.g. "Asia/Jakarta"
	Enabled  bool
	LastSent string // RFC3339, so a restart cannot double-send
}

// Kinds a reminder may have. Kept small and named for what the person gets, not
// for what the system does.
const (
	KindCheckin = "checkin"  // "anything to log today?"
	KindWeighIn = "weigh_in" // a morning weight
	KindReview  = "review"   // the week, once a week
)

func ValidKind(k string) bool {
	return k == KindCheckin || k == KindWeighIn || k == KindReview
}

// SaveReminder inserts or updates one. A person gets at most ONE reminder of each
// kind — the alternative is three daily check-ins nobody asked for, and a schedule
// board that can nag is worse than no schedule board.
func (d *DB) SaveReminder(r Reminder) error {
	if !ValidKind(r.Kind) {
		return fmt.Errorf("unknown reminder kind %q", r.Kind)
	}
	if r.Hour < 0 || r.Hour > 23 || r.Minute < 0 || r.Minute > 59 {
		return fmt.Errorf("%02d:%02d is not a time of day", r.Hour, r.Minute)
	}
	if r.Weekday < -1 || r.Weekday > 6 {
		return fmt.Errorf("weekday must be 0-6, or -1 for every day")
	}
	if _, err := time.LoadLocation(r.Zone); err != nil {
		return fmt.Errorf("unknown timezone %q", r.Zone)
	}
	_, err := d.Exec(`
		INSERT INTO reminder (profile_id, kind, hour, minute, weekday, zone, enabled, last_sent)
		VALUES (?,?,?,?,?,?,?,'')
		ON CONFLICT(profile_id, kind) DO UPDATE SET
		    hour=excluded.hour, minute=excluded.minute, weekday=excluded.weekday,
		    zone=excluded.zone, enabled=excluded.enabled`,
		r.Profile, r.Kind, r.Hour, r.Minute, r.Weekday, r.Zone, r.Enabled)
	return err
}

// Reminders lists one person's board.
func (d *DB) Reminders(profileID string) ([]Reminder, error) {
	rows, err := d.Query(`
		SELECT id, profile_id, kind, hour, minute, weekday, zone, enabled, last_sent
		FROM reminder WHERE profile_id = ? ORDER BY hour, minute`, profileID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanReminders(rows)
}

// DeleteReminder removes one kind for one person.
func (d *DB) DeleteReminder(profileID, kind string) error {
	_, err := d.Exec(`DELETE FROM reminder WHERE profile_id = ? AND kind = ?`,
		profileID, kind)
	return err
}

// DueReminders returns everything that should have fired by now and has not.
//
// "Has not" is the important half. The check is against last_sent rather than a
// timer, so a restart, a redeploy, or an hour of downtime cannot produce three
// copies of this morning's message — and cannot silently skip it either.
func (d *DB) DueReminders(now time.Time) ([]Reminder, error) {
	rows, err := d.Query(`
		SELECT id, profile_id, kind, hour, minute, weekday, zone, enabled, last_sent
		FROM reminder WHERE enabled = 1`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all, err := scanReminders(rows)
	if err != nil {
		return nil, err
	}

	var due []Reminder
	for _, r := range all {
		loc, err := time.LoadLocation(r.Zone)
		if err != nil {
			continue
		}
		local := now.In(loc)
		if r.Weekday >= 0 && int(local.Weekday()) != r.Weekday {
			continue
		}
		fireAt := time.Date(local.Year(), local.Month(), local.Day(),
			r.Hour, r.Minute, 0, 0, loc)
		if local.Before(fireAt) {
			continue
		}
		// Missed by more than three hours is not delivered at all. A check-in that
		// arrives at midnight because the server was down since morning is worse
		// than one that quietly waits for tomorrow.
		if local.Sub(fireAt) > 3*time.Hour {
			continue
		}
		if r.LastSent != "" {
			if t, err := time.Parse(time.RFC3339, r.LastSent); err == nil {
				if !t.In(loc).Before(fireAt) {
					continue
				}
			}
		}
		due = append(due, r)
	}
	return due, nil
}

// MarkSent records delivery, so the same occurrence never fires twice.
func (d *DB) MarkSent(id int64, at time.Time) error {
	_, err := d.Exec(`UPDATE reminder SET last_sent = ? WHERE id = ?`,
		at.UTC().Format(time.RFC3339), id)
	return err
}

func scanReminders(rows interface {
	Next() bool
	Scan(...any) error
	Err() error
}) ([]Reminder, error) {
	var out []Reminder
	for rows.Next() {
		var r Reminder
		var enabled int
		if err := rows.Scan(&r.ID, &r.Profile, &r.Kind, &r.Hour, &r.Minute,
			&r.Weekday, &r.Zone, &enabled, &r.LastSent); err != nil {
			return nil, err
		}
		r.Enabled = enabled == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

// Describe renders a reminder the way a person would say it.
func (r Reminder) Describe() string {
	when := fmt.Sprintf("%02d:%02d", r.Hour, r.Minute)
	day := "setiap hari"
	if r.Weekday >= 0 {
		day = "setiap " + []string{"Minggu", "Senin", "Selasa", "Rabu",
			"Kamis", "Jumat", "Sabtu"}[r.Weekday]
	}
	what := map[string]string{
		KindCheckin: "tanya kabar hari ini",
		KindWeighIn: "tanya berat badan",
		KindReview:  "ringkasan minggu ini",
	}[r.Kind]
	if !r.Enabled {
		return fmt.Sprintf("%s — %s, %s (mati)", what, day, when)
	}
	return fmt.Sprintf("%s — %s, %s", what, day, when)
}

var _ = strings.TrimSpace
