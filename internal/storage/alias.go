package storage

import (
	"fmt"
	"strings"
	"time"
)

// FoodAlias is an English name for a TKPI food, plus where it came from.
//
// TKPI carries an English gloss for only a handful of its 1,142 foods, so searching in
// English mostly fails. These fill that gap. They are SEARCH METADATA and never touch a
// nutrition value — a wrong alias costs a ranking, not a wrong number in someone's log.
type FoodAlias struct {
	Code      string
	NameEN    string // empty means "this food has no common English name"
	Source    string // agent | user — a machine guess must stay distinguishable from a correction
	UpdatedAt string
}

// PutFoodAlias records or replaces the English name for one food.
//
// An EMPTY NameEN is meaningful and is stored: it records that the food has no English
// name, so nothing re-asks about oncom or gembus on every future pass.
func (d *DB) PutFoodAlias(code, nameEN, source string) error {
	code = strings.ToUpper(strings.TrimSpace(code))
	if code == "" {
		return fmt.Errorf("food code is required")
	}
	if source == "" {
		source = "agent"
	}
	_, err := d.Exec(`INSERT INTO food_alias (code, name_en, source, updated_at)
		VALUES (?,?,?,?)
		ON CONFLICT(code) DO UPDATE SET
		    name_en=excluded.name_en, source=excluded.source, updated_at=excluded.updated_at`,
		code, strings.TrimSpace(nameEN), source, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("save food alias %s: %w", code, err)
	}
	return nil
}

func (d *DB) FoodAliases() (map[string]FoodAlias, error) {
	rows, err := d.Query(`SELECT code, name_en, source, updated_at FROM food_alias`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]FoodAlias{}
	for rows.Next() {
		var a FoodAlias
		if err := rows.Scan(&a.Code, &a.NameEN, &a.Source, &a.UpdatedAt); err != nil {
			return nil, err
		}
		out[a.Code] = a
	}
	return out, rows.Err()
}
