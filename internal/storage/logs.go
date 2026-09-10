package storage

import (
	"fmt"
	"time"
)

type SetEntry struct {
	Date, Exercise, BodyPart string
	Sets, Reps               int
	WeightKg, VolumeKg       float64
	Note                     string
}

type CardioEntry struct {
	Date                                      string
	Minutes, SpeedKmh, InclinePct, DistanceKm float64
	Note                                      string
}

type WeightEntry struct {
	Date     string
	WeightKg float64
	Note     string
}

type FoodEntry struct {
	Date, Meal, Item                    string
	Grams, Kcal, ProteinG, CarbsG, FatG float64
	Source, PhotoHash                   string
}

// since converts a day count into an ISO date bound. Callers pass days because that
// is how people ask the question; SQL wants a date.
func since(days int) string {
	return time.Now().AddDate(0, 0, -days).Format("2006-01-02")
}

func (d *DB) LogSet(profileID string, e SetEntry) error {
	_, err := d.Exec(`INSERT INTO training_log
		(profile_id, date, exercise, body_part, sets, reps, weight_kg, volume_kg, note)
		VALUES (?,?,?,?,?,?,?,?,?)`,
		profileID, e.Date, e.Exercise, e.BodyPart, e.Sets, e.Reps,
		e.WeightKg, e.VolumeKg, e.Note)
	if err != nil {
		return fmt.Errorf("log set for %s: %w", profileID, err)
	}
	return nil
}

func (d *DB) LogCardio(profileID string, e CardioEntry) error {
	_, err := d.Exec(`INSERT INTO cardio_log
		(profile_id, date, minutes, speed_kmh, incline_pct, distance_km, note)
		VALUES (?,?,?,?,?,?,?)`,
		profileID, e.Date, e.Minutes, e.SpeedKmh, e.InclinePct, e.DistanceKm, e.Note)
	if err != nil {
		return fmt.Errorf("log cardio for %s: %w", profileID, err)
	}
	return nil
}

func (d *DB) LogWeight(profileID string, e WeightEntry) error {
	_, err := d.Exec(`INSERT INTO weight_log (profile_id, date, weight_kg, note)
		VALUES (?,?,?,?)`, profileID, e.Date, e.WeightKg, e.Note)
	if err != nil {
		return fmt.Errorf("log weight for %s: %w", profileID, err)
	}
	return nil
}

func (d *DB) LogFood(profileID string, e FoodEntry) error {
	_, err := d.Exec(`INSERT INTO food_log
		(profile_id, date, meal, item, grams, kcal, protein_g, carbs_g, fat_g,
		 source, photo_hash)
		VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		profileID, e.Date, e.Meal, e.Item, e.Grams, e.Kcal, e.ProteinG,
		e.CarbsG, e.FatG, e.Source, e.PhotoHash)
	if err != nil {
		return fmt.Errorf("log food for %s: %w", profileID, err)
	}
	return nil
}

func (d *DB) Sets(profileID string, sinceDays int) ([]SetEntry, error) {
	rows, err := d.Query(`SELECT date, exercise, COALESCE(body_part,''), sets, reps,
		COALESCE(weight_kg,0), COALESCE(volume_kg,0), COALESCE(note,'')
		FROM training_log WHERE profile_id=? AND date>=? ORDER BY date, id`,
		profileID, since(sinceDays))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SetEntry
	for rows.Next() {
		var e SetEntry
		if err := rows.Scan(&e.Date, &e.Exercise, &e.BodyPart, &e.Sets, &e.Reps,
			&e.WeightKg, &e.VolumeKg, &e.Note); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) Cardio(profileID string, sinceDays int) ([]CardioEntry, error) {
	rows, err := d.Query(`SELECT date, minutes, COALESCE(speed_kmh,0),
		COALESCE(incline_pct,0), COALESCE(distance_km,0), COALESCE(note,'')
		FROM cardio_log WHERE profile_id=? AND date>=? ORDER BY date, id`,
		profileID, since(sinceDays))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CardioEntry
	for rows.Next() {
		var e CardioEntry
		if err := rows.Scan(&e.Date, &e.Minutes, &e.SpeedKmh, &e.InclinePct,
			&e.DistanceKm, &e.Note); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) Weights(profileID string, sinceDays int) ([]WeightEntry, error) {
	rows, err := d.Query(`SELECT date, weight_kg, COALESCE(note,'')
		FROM weight_log WHERE profile_id=? AND date>=? ORDER BY date, id`,
		profileID, since(sinceDays))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WeightEntry
	for rows.Next() {
		var e WeightEntry
		if err := rows.Scan(&e.Date, &e.WeightKg, &e.Note); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (d *DB) Foods(profileID string, sinceDays int) ([]FoodEntry, error) {
	rows, err := d.Query(`SELECT date, COALESCE(meal,''), item, COALESCE(grams,0),
		COALESCE(kcal,0), COALESCE(protein_g,0), COALESCE(carbs_g,0),
		COALESCE(fat_g,0), source, COALESCE(photo_hash,'')
		FROM food_log WHERE profile_id=? AND date>=? ORDER BY date, id`,
		profileID, since(sinceDays))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []FoodEntry
	for rows.Next() {
		var e FoodEntry
		if err := rows.Scan(&e.Date, &e.Meal, &e.Item, &e.Grams, &e.Kcal,
			&e.ProteinG, &e.CarbsG, &e.FatG, &e.Source, &e.PhotoHash); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
