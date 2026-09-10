# Sehaty Core Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Go MCP server exposing profile-isolated training, food and weight tools over Streamable HTTP, that Hermes can register and call.

**Architecture:** One binary. `storage` owns SQLite and enforces `profile_id` isolation in a single data-access layer nothing bypasses. `catalog` loads the exercise dataset and filters by profile equipment. `tools` registers MCP tools that call only those two. Media and AI arrive in later plans behind interfaces defined here.

**Tech Stack:** Go 1.27 · `github.com/modelcontextprotocol/go-sdk` · `modernc.org/sqlite` (cgo-free, so the binary stays static) · stdlib for everything else.

> **AMENDED 10 Sep, after this plan was executed.** The driver below is **superseded**:
> `modernc.org/sqlite` cannot encrypt, which left every log in plaintext inside a file that
> is backed up nightly. Replaced by `github.com/ncruces/go-sqlite3` opened through its
> **Adiantum VFS** — also cgo-free, also `database/sql`, but it encrypts the whole file.
> The schema and every query in this plan are unchanged; only `storage.Open` differs.
> See spec §8c. Steps below are kept as the historical record of what was built.

**Spec:** `docs/superpowers/specs/2026-09-10-sehaty.md`

## Global Constraints

- **Go 1.27.1** — the version on hrmdev01/02/03. Build there; the Mac has no Go.
- **`modernc.org/sqlite`, not `mattn/go-sqlite3`** — cgo-free keeps the single-static-binary property that motivated choosing Go.
- **Every query filtered by `profile_id`**, in `storage` only. No SQL outside that package.
- **SQLite in WAL mode, on local disk.** Never a network filesystem.
- **No tool may prescribe equipment a profile lacks.** Filter before selection; no override.
- **Never invent a number.** Absent data reports as absent.
- Module path: `github.com/rakasatria/sehaty` — set once in Task 1, unchanged after.
- Table and column names exactly as the spec's §3 data model.

---

### Task 1: Module skeleton and SQLite schema

**Files:**
- Create: `go.mod`, `internal/storage/storage.go`, `internal/storage/schema.sql`
- Test: `internal/storage/storage_test.go`

**Interfaces:**
- Consumes: nothing
- Produces: `storage.Open(path string) (*DB, error)`; `type DB struct{ *sql.DB }`; `(*DB).Close() error`. Schema tables per spec §3.

- [ ] **Step 1: Write the failing test**

```go
package storage

import "testing"

func TestOpenCreatesSchema(t *testing.T) {
	db, err := Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	for _, table := range []string{"profile", "identity", "training_log",
		"cardio_log", "weight_log", "food_log", "secret_blob"} {
		var name string
		err := db.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`,
			table).Scan(&name)
		if err != nil {
			t.Errorf("table %q missing: %v", table, err)
		}
	}
}

func TestOpenSetsWAL(t *testing.T) {
	db, err := Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer db.Close()

	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Errorf("journal_mode = %q, want wal", mode)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/ -run TestOpen -v`
Expected: FAIL — build error, `undefined: Open`

- [ ] **Step 3: Write the schema**

Create `internal/storage/schema.sql`:

```sql
CREATE TABLE IF NOT EXISTS profile (
  id                TEXT PRIMARY KEY,
  equipment_json    TEXT NOT NULL DEFAULT '["body weight"]',
  goal              TEXT NOT NULL DEFAULT 'general',
  sessions_per_week INTEGER NOT NULL DEFAULT 3,
  session_minutes   INTEGER NOT NULL DEFAULT 50,
  experience        TEXT NOT NULL DEFAULT 'beginner',
  max_difficulty    INTEGER NOT NULL DEFAULT 3,
  locale            TEXT NOT NULL DEFAULT 'en',
  created_at        TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS identity (
  channel     TEXT NOT NULL,
  external_id TEXT NOT NULL,
  profile_id  TEXT NOT NULL REFERENCES profile(id),
  PRIMARY KEY (channel, external_id)
);

CREATE TABLE IF NOT EXISTS training_log (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id TEXT NOT NULL REFERENCES profile(id),
  date       TEXT NOT NULL,
  exercise   TEXT NOT NULL,
  body_part  TEXT,
  sets       INTEGER NOT NULL,
  reps       INTEGER NOT NULL,
  weight_kg  REAL,
  volume_kg  REAL,
  note       TEXT
);

CREATE TABLE IF NOT EXISTS cardio_log (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id  TEXT NOT NULL REFERENCES profile(id),
  date        TEXT NOT NULL,
  minutes     REAL NOT NULL,
  speed_kmh   REAL,
  incline_pct REAL,
  distance_km REAL,
  note        TEXT
);

CREATE TABLE IF NOT EXISTS weight_log (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id TEXT NOT NULL REFERENCES profile(id),
  date       TEXT NOT NULL,
  weight_kg  REAL NOT NULL,
  note       TEXT
);

CREATE TABLE IF NOT EXISTS food_log (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  profile_id TEXT NOT NULL REFERENCES profile(id),
  date       TEXT NOT NULL,
  meal       TEXT,
  item       TEXT NOT NULL,
  grams      REAL,
  kcal       REAL,
  protein_g  REAL,
  carbs_g    REAL,
  fat_g      REAL,
  source     TEXT NOT NULL,
  photo_hash TEXT
);

CREATE TABLE IF NOT EXISTS secret_blob (
  profile_id TEXT NOT NULL REFERENCES profile(id),
  key        TEXT NOT NULL,
  ciphertext BLOB NOT NULL,
  PRIMARY KEY (profile_id, key)
);

CREATE INDEX IF NOT EXISTS idx_training_profile_date ON training_log(profile_id, date);
CREATE INDEX IF NOT EXISTS idx_cardio_profile_date   ON cardio_log(profile_id, date);
CREATE INDEX IF NOT EXISTS idx_weight_profile_date   ON weight_log(profile_id, date);
CREATE INDEX IF NOT EXISTS idx_food_profile_date     ON food_log(profile_id, date);
```

- [ ] **Step 4: Write the minimal implementation**

Create `internal/storage/storage.go`:

```go
// Package storage owns the database. Every query lives here, and every query is
// filtered by profile_id — that isolation, not encryption, is what stops one person
// reading another's health data. No SQL outside this package.
package storage

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type DB struct{ *sql.DB }

// Open opens (creating if needed) the database at path and applies the schema.
// WAL is set explicitly: it survives concurrent readers during a write, which a
// dashboard polling while a tool logs a set will do constantly.
func Open(path string) (*DB, error) {
	sqlDB, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	if _, err := sqlDB.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("set WAL: %w", err)
	}
	if _, err := sqlDB.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := sqlDB.Exec(schema); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return &DB{sqlDB}, nil
}
```

- [ ] **Step 5: Initialise the module and fetch dependencies**

```bash
cd ~/codes/sehaty
go mod init github.com/rakasatria/sehaty
go get modernc.org/sqlite
go mod tidy
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/storage/ -v`
Expected: PASS — both tests

- [ ] **Step 7: Commit**

```bash
git add go.mod go.sum internal/storage/
git commit -m "feat(storage): sqlite schema with profile isolation"
```

---

### Task 2: Profiles — create, read, list

**Files:**
- Create: `internal/storage/profile.go`
- Test: `internal/storage/profile_test.go`

**Interfaces:**
- Consumes: `storage.Open`, `*DB` from Task 1
- Produces:
  - `type Profile struct { ID string; Equipment []string; Goal string; SessionsPerWeek int; SessionMinutes int; Experience string; MaxDifficulty int; Locale string }`
  - `(*DB).SaveProfile(p Profile) error`
  - `(*DB).GetProfile(id string) (Profile, error)` — returns `ErrNoProfile` when absent
  - `(*DB).ListProfiles() ([]Profile, error)`
  - `var ErrNoProfile = errors.New(...)`
  - `func ValidID(s string) bool`

- [ ] **Step 1: Write the failing test**

```go
package storage

import (
	"errors"
	"testing"
)

func testDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSaveAndGetProfile(t *testing.T) {
	db := testDB(t)
	want := Profile{ID: "raka", Equipment: []string{"body weight", "dumbbell"},
		Goal: "fat_loss", SessionsPerWeek: 3, SessionMinutes: 50,
		Experience: "beginner", MaxDifficulty: 3, Locale: "id"}
	if err := db.SaveProfile(want); err != nil {
		t.Fatalf("SaveProfile: %v", err)
	}
	got, err := db.GetProfile("raka")
	if err != nil {
		t.Fatalf("GetProfile: %v", err)
	}
	if got.Goal != want.Goal || got.Locale != want.Locale {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if len(got.Equipment) != 2 || got.Equipment[1] != "dumbbell" {
		t.Errorf("equipment = %v", got.Equipment)
	}
}

func TestGetProfileMissing(t *testing.T) {
	db := testDB(t)
	if _, err := db.GetProfile("nobody"); !errors.Is(err, ErrNoProfile) {
		t.Errorf("err = %v, want ErrNoProfile", err)
	}
}

func TestSaveProfileIsUpsert(t *testing.T) {
	db := testDB(t)
	p := Profile{ID: "raka", Equipment: []string{"body weight"}, Goal: "general",
		SessionsPerWeek: 3, SessionMinutes: 50, Experience: "beginner",
		MaxDifficulty: 3, Locale: "en"}
	if err := db.SaveProfile(p); err != nil {
		t.Fatal(err)
	}
	p.Goal = "fat_loss"
	if err := db.SaveProfile(p); err != nil {
		t.Fatalf("second save: %v", err)
	}
	got, _ := db.GetProfile("raka")
	if got.Goal != "fat_loss" {
		t.Errorf("goal = %q, want fat_loss", got.Goal)
	}
	all, _ := db.ListProfiles()
	if len(all) != 1 {
		t.Errorf("ListProfiles = %d rows, want 1", len(all))
	}
}

func TestValidIDRejectsTraversal(t *testing.T) {
	for _, bad := range []string{"../etc", "a/b", "", "UPPER", "with space"} {
		if ValidID(bad) {
			t.Errorf("ValidID(%q) = true, want false", bad)
		}
	}
	for _, ok := range []string{"raka", "user_2", "a-b"} {
		if !ValidID(ok) {
			t.Errorf("ValidID(%q) = false, want true", ok)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/ -run Profile -v`
Expected: FAIL — `undefined: Profile`

- [ ] **Step 3: Write the implementation**

Create `internal/storage/profile.go`:

```go
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

func scanProfile(sc interface{ Scan(...any) error }) (Profile, error) {
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/storage/ -v`
Expected: PASS — all profile tests plus Task 1's

- [ ] **Step 5: Commit**

```bash
git add internal/storage/profile.go internal/storage/profile_test.go
git commit -m "feat(storage): profile create, read, list with id validation"
```

---

### Task 3: Logs — write and read, isolated by profile

**Files:**
- Create: `internal/storage/logs.go`
- Test: `internal/storage/logs_test.go`

**Interfaces:**
- Consumes: `*DB`, `Profile` from Tasks 1–2
- Produces:
  - `type SetEntry struct { Date, Exercise, BodyPart string; Sets, Reps int; WeightKg, VolumeKg float64; Note string }`
  - `type CardioEntry struct { Date string; Minutes, SpeedKmh, InclinePct, DistanceKm float64; Note string }`
  - `type WeightEntry struct { Date string; WeightKg float64; Note string }`
  - `type FoodEntry struct { Date, Meal, Item string; Grams, Kcal, ProteinG, CarbsG, FatG float64; Source, PhotoHash string }`
  - `(*DB).LogSet(profileID string, e SetEntry) error` and matching `LogCardio`, `LogWeight`, `LogFood`
  - `(*DB).Sets(profileID string, sinceDays int) ([]SetEntry, error)` and matching `Cardio`, `Weights`, `Foods`

- [ ] **Step 1: Write the failing test**

```go
package storage

import "testing"

func seed(t *testing.T, db *DB, ids ...string) {
	t.Helper()
	for _, id := range ids {
		err := db.SaveProfile(Profile{ID: id, Equipment: []string{"body weight"},
			Goal: "general", SessionsPerWeek: 3, SessionMinutes: 50,
			Experience: "beginner", MaxDifficulty: 3, Locale: "en"})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestLogSetRoundTrip(t *testing.T) {
	db := testDB(t)
	seed(t, db, "raka")
	e := SetEntry{Date: "2026-09-10", Exercise: "goblet squat", BodyPart: "upper legs",
		Sets: 3, Reps: 12, WeightKg: 20, VolumeKg: 720}
	if err := db.LogSet("raka", e); err != nil {
		t.Fatalf("LogSet: %v", err)
	}
	got, err := db.Sets("raka", 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Exercise != "goblet squat" || got[0].VolumeKg != 720 {
		t.Errorf("got %+v", got)
	}
}

// The isolation guarantee. If this ever fails, one person is reading another's
// health data, which is the single worst bug this system can have.
func TestLogsAreIsolatedByProfile(t *testing.T) {
	db := testDB(t)
	seed(t, db, "raka", "other")
	must(t, db.LogSet("raka", SetEntry{Date: "2026-09-10", Exercise: "push-up",
		Sets: 3, Reps: 10}))
	must(t, db.LogWeight("raka", WeightEntry{Date: "2026-09-10", WeightKg: 85}))
	must(t, db.LogSet("other", SetEntry{Date: "2026-09-10", Exercise: "burpee",
		Sets: 1, Reps: 5}))

	sets, _ := db.Sets("raka", 30)
	if len(sets) != 1 || sets[0].Exercise != "push-up" {
		t.Errorf("raka sets leaked: %+v", sets)
	}
	otherSets, _ := db.Sets("other", 30)
	if len(otherSets) != 1 || otherSets[0].Exercise != "burpee" {
		t.Errorf("other sets wrong: %+v", otherSets)
	}
	otherWeights, _ := db.Weights("other", 30)
	if len(otherWeights) != 0 {
		t.Errorf("other saw raka's weight: %+v", otherWeights)
	}
}

func TestSinceDaysExcludesOlder(t *testing.T) {
	db := testDB(t)
	seed(t, db, "raka")
	must(t, db.LogSet("raka", SetEntry{Date: "2020-01-01", Exercise: "push-up",
		Sets: 1, Reps: 1}))
	got, _ := db.Sets("raka", 7)
	if len(got) != 0 {
		t.Errorf("old row returned: %+v", got)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/ -run Log -v`
Expected: FAIL — `undefined: SetEntry`

- [ ] **Step 3: Write the implementation**

Create `internal/storage/logs.go`:

```go
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
	Date                                   string
	Minutes, SpeedKmh, InclinePct, DistanceKm float64
	Note                                   string
}

type WeightEntry struct {
	Date     string
	WeightKg float64
	Note     string
}

type FoodEntry struct {
	Date, Meal, Item              string
	Grams, Kcal, ProteinG, CarbsG, FatG float64
	Source, PhotoHash             string
}

// since converts a day count into an ISO date bound. Callers pass days because
// that is how people ask the question; SQL wants a date.
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/storage/ -v`
Expected: PASS — including `TestLogsAreIsolatedByProfile`

- [ ] **Step 5: Commit**

```bash
git add internal/storage/logs.go internal/storage/logs_test.go
git commit -m "feat(storage): training, cardio, weight and food logs with profile isolation"
```

---

### Task 4: Identity — one profile, many front doors

**Files:**
- Create: `internal/storage/identity.go`
- Test: `internal/storage/identity_test.go`

**Interfaces:**
- Consumes: `*DB`, `ErrNoProfile` from Tasks 1–2
- Produces:
  - `(*DB).LinkIdentity(channel, externalID, profileID string) error`
  - `(*DB).ResolveIdentity(channel, externalID string) (string, error)` — returns `ErrNoIdentity` when unlinked
  - `var ErrNoIdentity = errors.New(...)`

- [ ] **Step 1: Write the failing test**

```go
package storage

import (
	"errors"
	"testing"
)

func TestLinkAndResolveIdentity(t *testing.T) {
	db := testDB(t)
	seed(t, db, "raka")
	if err := db.LinkIdentity("telegram", "8412", "raka"); err != nil {
		t.Fatalf("LinkIdentity: %v", err)
	}
	got, err := db.ResolveIdentity("telegram", "8412")
	if err != nil {
		t.Fatalf("ResolveIdentity: %v", err)
	}
	if got != "raka" {
		t.Errorf("got %q, want raka", got)
	}
}

func TestResolveUnknownIdentity(t *testing.T) {
	db := testDB(t)
	if _, err := db.ResolveIdentity("telegram", "9999"); !errors.Is(err, ErrNoIdentity) {
		t.Errorf("err = %v, want ErrNoIdentity", err)
	}
}

// Same external id on two channels must not collide — a Telegram user 8412 and a
// Cloudflare Access email are unrelated namespaces.
func TestChannelsAreSeparateNamespaces(t *testing.T) {
	db := testDB(t)
	seed(t, db, "raka", "other")
	must(t, db.LinkIdentity("telegram", "8412", "raka"))
	must(t, db.LinkIdentity("access", "8412", "other"))
	a, _ := db.ResolveIdentity("telegram", "8412")
	b, _ := db.ResolveIdentity("access", "8412")
	if a != "raka" || b != "other" {
		t.Errorf("telegram=%q access=%q", a, b)
	}
}

func TestLinkRejectsUnknownProfile(t *testing.T) {
	db := testDB(t)
	if err := db.LinkIdentity("telegram", "1", "ghost"); err == nil {
		t.Error("linking to a nonexistent profile should fail")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/storage/ -run Identity -v`
Expected: FAIL — `undefined: ErrNoIdentity`

- [ ] **Step 3: Write the implementation**

Create `internal/storage/identity.go`:

```go
package storage

import (
	"database/sql"
	"errors"
	"fmt"
)

var ErrNoIdentity = errors.New("identity not linked to any profile")

// LinkIdentity binds an external account to a profile. Channels are separate
// namespaces: telegram/8412 and access/8412 are unrelated. The foreign key on
// profile_id is what rejects a link to a profile that does not exist — which is
// why PRAGMA foreign_keys=ON in Open() matters rather than being decoration.
func (d *DB) LinkIdentity(channel, externalID, profileID string) error {
	_, err := d.Exec(`INSERT INTO identity (channel, external_id, profile_id)
		VALUES (?,?,?)
		ON CONFLICT(channel, external_id) DO UPDATE SET profile_id=excluded.profile_id`,
		channel, externalID, profileID)
	if err != nil {
		return fmt.Errorf("link %s/%s -> %s: %w", channel, externalID, profileID, err)
	}
	return nil
}

func (d *DB) ResolveIdentity(channel, externalID string) (string, error) {
	var id string
	err := d.QueryRow(`SELECT profile_id FROM identity
		WHERE channel=? AND external_id=?`, channel, externalID).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("%s/%s: %w", channel, externalID, ErrNoIdentity)
	}
	return id, err
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/storage/ -v`
Expected: PASS — all identity tests

- [ ] **Step 5: Commit**

```bash
git add internal/storage/identity.go internal/storage/identity_test.go
git commit -m "feat(storage): identity mapping for multiple front doors"
```

---

### Task 5: Exercise catalog with equipment and difficulty filtering

**Files:**
- Create: `internal/catalog/catalog.go`, `internal/catalog/difficulty.json`
- Test: `internal/catalog/catalog_test.go`, `internal/catalog/testdata/exercises.json`

**Interfaces:**
- Consumes: `storage.Profile` from Task 2
- Produces:
  - `type Exercise struct { ID, Name, BodyPart, Equipment, Target string; Difficulty int }`
  - `catalog.Load(path string) (*Catalog, error)`
  - `(*Catalog).For(equipment []string, maxDifficulty int) []Exercise`
  - `(*Catalog).ByName(name string) (Exercise, bool)`
  - `(*Catalog).Count() int`

- [ ] **Step 1: Create the test fixture**

Create `internal/catalog/testdata/exercises.json` — a small stand-in so tests never
depend on the 169 MB submodule:

```json
[
  {"id":"0001","name":"push-up","body_part":"chest","equipment":"body weight","target":"pectorals"},
  {"id":"0002","name":"goblet squat","body_part":"upper legs","equipment":"dumbbell","target":"quads"},
  {"id":"0003","name":"barbell bench press","body_part":"chest","equipment":"barbell","target":"pectorals"},
  {"id":"0004","name":"single leg squat (pistol)","body_part":"upper legs","equipment":"body weight","target":"quads"}
]
```

Create `internal/catalog/difficulty.json` — the ratings the dataset lacks. Start with the
fixture's four; the full 1,324 are generated in a later plan:

```json
{
  "push-up": 2,
  "goblet squat": 2,
  "barbell bench press": 3,
  "single leg squat (pistol)": 5
}
```

- [ ] **Step 2: Write the failing test**

```go
package catalog

import "testing"

func load(t *testing.T) *Catalog {
	t.Helper()
	c, err := Load("testdata/exercises.json")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestForFiltersByEquipment(t *testing.T) {
	got := load(t).For([]string{"body weight"}, 5)
	for _, e := range got {
		if e.Equipment != "body weight" {
			t.Errorf("returned %q with equipment %q", e.Name, e.Equipment)
		}
	}
	if len(got) != 2 {
		t.Errorf("got %d exercises, want 2", len(got))
	}
}

// The prototype prescribed a pistol squat to a beginner. This is the test that
// stops that happening again.
func TestForRespectsMaxDifficulty(t *testing.T) {
	got := load(t).For([]string{"body weight"}, 3)
	for _, e := range got {
		if e.Name == "single leg squat (pistol)" {
			t.Error("pistol squat (difficulty 5) returned at maxDifficulty 3")
		}
		if e.Difficulty > 3 {
			t.Errorf("%q has difficulty %d > 3", e.Name, e.Difficulty)
		}
	}
}

func TestUnratedExerciseIsTreatedAsHard(t *testing.T) {
	c := load(t)
	e, ok := c.ByName("push-up")
	if !ok {
		t.Fatal("push-up missing")
	}
	if e.Difficulty == 0 {
		t.Error("rated exercise should not have difficulty 0")
	}
}

func TestEquipmentMatchIsCaseInsensitive(t *testing.T) {
	if len(load(t).For([]string{"BODY WEIGHT"}, 5)) != 2 {
		t.Error("equipment match should be case-insensitive")
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `go test ./internal/catalog/ -v`
Expected: FAIL — `undefined: Load`

- [ ] **Step 4: Write the implementation**

Create `internal/catalog/catalog.go`:

```go
// Package catalog loads the exercise dataset and answers "what can this person
// actually do?". The upstream dataset has no difficulty field, so ratings live
// here in difficulty.json — without them the planner happily prescribes pistol
// squats to beginners, which it did.
package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

//go:embed difficulty.json
var difficultyJSON []byte

// unratedDifficulty is what an exercise scores when difficulty.json has no entry.
// It is deliberately high: an unknown movement should be excluded from a
// beginner's plan, not included by default. Failing safe matters more here than
// coverage.
const unratedDifficulty = 4

type Exercise struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	BodyPart   string `json:"body_part"`
	Equipment  string `json:"equipment"`
	Target     string `json:"target"`
	Difficulty int    `json:"-"`
}

type Catalog struct {
	all   []Exercise
	byKey map[string]Exercise
}

func Load(path string) (*Catalog, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read exercises %s: %w "+
			"(run: git submodule update --init --depth 1)", path, err)
	}
	var list []Exercise
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("parse exercises: %w", err)
	}
	ratings := map[string]int{}
	if err := json.Unmarshal(difficultyJSON, &ratings); err != nil {
		return nil, fmt.Errorf("parse difficulty.json: %w", err)
	}
	c := &Catalog{byKey: make(map[string]Exercise, len(list))}
	for _, e := range list {
		if d, ok := ratings[strings.ToLower(e.Name)]; ok {
			e.Difficulty = d
		} else {
			e.Difficulty = unratedDifficulty
		}
		c.all = append(c.all, e)
		c.byKey[strings.ToLower(e.Name)] = e
	}
	return c, nil
}

func (c *Catalog) Count() int { return len(c.all) }

func (c *Catalog) ByName(name string) (Exercise, bool) {
	e, ok := c.byKey[strings.ToLower(name)]
	return e, ok
}

// For returns only what this person can perform. Equipment is filtered before
// anything else and there is no override at prescribe time — a plan someone
// cannot do is worse than no plan.
func (c *Catalog) For(equipment []string, maxDifficulty int) []Exercise {
	have := make(map[string]bool, len(equipment))
	for _, eq := range equipment {
		have[strings.ToLower(strings.TrimSpace(eq))] = true
	}
	var out []Exercise
	for _, e := range c.all {
		if !have[strings.ToLower(e.Equipment)] {
			continue
		}
		if maxDifficulty > 0 && e.Difficulty > maxDifficulty {
			continue
		}
		out = append(out, e)
	}
	return out
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/catalog/ -v`
Expected: PASS — all four tests

- [ ] **Step 6: Commit**

```bash
git add internal/catalog/
git commit -m "feat(catalog): equipment and difficulty filtering, unrated fails safe"
```

---

### Task 6: MCP server exposing the deterministic tools

**Files:**
- Create: `cmd/sehaty/main.go`, `internal/tools/tools.go`
- Test: `internal/tools/tools_test.go`

**Interfaces:**
- Consumes: `storage.*` (Tasks 1–4), `catalog.*` (Task 5)
- Produces: `tools.Register(s *mcp.Server, db *storage.DB, cat *catalog.Catalog)`; binary `sehaty` serving Streamable HTTP on `SEHATY_MCP_PORT` (default 8765)

- [ ] **Step 1: Write the failing test**

Tools are tested through their handler functions rather than over HTTP — the transport
is the SDK's job, not ours.

```go
package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/rakasatria/sehaty/internal/catalog"
	"github.com/rakasatria/sehaty/internal/storage"
)

func fixture(t *testing.T) (*storage.DB, *catalog.Catalog) {
	t.Helper()
	db, err := storage.Open(t.TempDir() + "/t.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	err = db.SaveProfile(storage.Profile{ID: "raka",
		Equipment: []string{"body weight", "dumbbell"}, Goal: "fat_loss",
		SessionsPerWeek: 3, SessionMinutes: 50, Experience: "beginner",
		MaxDifficulty: 3, Locale: "en"})
	if err != nil {
		t.Fatal(err)
	}
	cat, err := catalog.Load("../catalog/testdata/exercises.json")
	if err != nil {
		t.Fatal(err)
	}
	return db, cat
}

func TestPlanSessionRespectsEquipmentAndDifficulty(t *testing.T) {
	db, cat := fixture(t)
	out, err := PlanSession(context.Background(), db, cat, "raka", "full", 50)
	if err != nil {
		t.Fatalf("PlanSession: %v", err)
	}
	if len(out.Exercises) == 0 {
		t.Fatal("empty plan")
	}
	for _, e := range out.Exercises {
		if e.Equipment == "barbell" {
			t.Errorf("prescribed %q which needs a barbell", e.Name)
		}
		if strings.Contains(e.Name, "pistol") {
			t.Errorf("prescribed %q to a beginner", e.Name)
		}
	}
}

func TestLogSetRejectsUnavailableEquipment(t *testing.T) {
	db, cat := fixture(t)
	_, err := LogSet(context.Background(), db, cat, "raka",
		"barbell bench press", 3, 10, 60)
	if err == nil {
		t.Error("logging a barbell lift should fail for a bodyweight+dumbbell profile")
	}
}

func TestProgressReportsAbsenceHonestly(t *testing.T) {
	db, cat := fixture(t)
	p, err := Progress(context.Background(), db, "raka", 14)
	if err != nil {
		t.Fatal(err)
	}
	if p.LiftingSessions != 0 || p.TotalVolumeKg != 0 {
		t.Errorf("empty log should report zeros, got %+v", p)
	}
	if p.LatestWeightKg != nil {
		t.Error("no weigh-ins should give a nil weight, not a zero")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/tools/ -v`
Expected: FAIL — `undefined: PlanSession`

- [ ] **Step 3: Write the tool implementations**

Create `internal/tools/tools.go`. Handlers are plain functions so they are testable
without a transport; `Register` wires them to MCP.

```go
// Package tools exposes storage and catalog as MCP tools. Handlers are plain
// functions — the MCP wiring is a thin shell over them, so the behaviour can be
// tested without standing up a server.
package tools

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/rakasatria/sehaty/internal/catalog"
	"github.com/rakasatria/sehaty/internal/storage"
)

func today() string { return time.Now().Format("2006-01-02") }

// Goal drives sets, reps and rest. Fat loss keeps density high; strength trades
// volume for load.
var goals = map[string]struct{ Sets int; Reps, Rest, Note string }{
	"fat_loss":    {3, "10-15", "45-60s", "Keep density high. Intake decides body composition, not volume."},
	"strength":    {4, "4-6", "2-3 min", "Heavy, long rests, add load before reps."},
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
	Profile   string             `json:"profile"`
	Date      string             `json:"date"`
	Focus     string             `json:"focus"`
	Minutes   int                `json:"minutes"`
	Goal      string             `json:"goal"`
	Sets      int                `json:"sets"`
	Reps      string             `json:"reps"`
	Rest      string             `json:"rest"`
	Note      string             `json:"note"`
	Exercises []catalog.Exercise `json:"exercises"`
}

func PlanSession(ctx context.Context, db *storage.DB, cat *catalog.Catalog,
	profileID, focus string, minutes int) (PlanOut, error) {

	blk, ok := blocks[focus]
	if !ok {
		return PlanOut{}, fmt.Errorf("focus must be full, upper or lower")
	}
	p, err := db.GetProfile(profileID)
	if err != nil {
		return PlanOut{}, err
	}
	if minutes <= 0 {
		minutes = p.SessionMinutes
	}
	avail := cat.For(p.Equipment, p.MaxDifficulty)

	// Rotate away from anything done in the last 10 days.
	recent := map[string]bool{}
	if sets, err := db.Sets(profileID, 10); err == nil {
		for _, s := range sets {
			recent[strings.ToLower(s.Exercise)] = true
		}
	}
	byPart := map[string][]catalog.Exercise{}
	for _, e := range avail {
		k := strings.ToLower(e.BodyPart)
		byPart[k] = append(byPart[k], e)
	}
	// Seeded by date+focus+profile so the plan is stable all day and different
	// tomorrow — a plan that reshuffles on every call cannot be followed.
	rng := rand.New(rand.NewSource(hash(today() + focus + profileID)))
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
	if cap := minutes / 5; cap >= 4 && len(picked) > cap {
		picked = picked[:cap]
	}
	g := goals[p.Goal]
	return PlanOut{Profile: profileID, Date: today(), Focus: focus, Minutes: minutes,
		Goal: p.Goal, Sets: g.Sets, Reps: g.Reps, Rest: g.Rest, Note: g.Note,
		Exercises: picked}, nil
}

func hash(s string) int64 {
	var h int64 = 14695981039346656037
	for _, c := range s {
		h ^= int64(c)
		h *= 1099511628211
	}
	if h < 0 {
		h = -h
	}
	return h
}

type LogOut struct {
	Profile  string  `json:"profile"`
	Exercise string  `json:"exercise"`
	Sets     int     `json:"sets"`
	Reps     int     `json:"reps"`
	WeightKg float64 `json:"weight_kg"`
	VolumeKg float64 `json:"volume_kg"`
}

func LogSet(ctx context.Context, db *storage.DB, cat *catalog.Catalog,
	profileID, exercise string, sets, reps int, weightKg float64) (LogOut, error) {

	p, err := db.GetProfile(profileID)
	if err != nil {
		return LogOut{}, err
	}
	ex, ok := cat.ByName(exercise)
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
	err = db.LogSet(profileID, storage.SetEntry{Date: today(), Exercise: ex.Name,
		BodyPart: ex.BodyPart, Sets: sets, Reps: reps, WeightKg: weightKg,
		VolumeKg: vol})
	if err != nil {
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
	// Pointer, not float: no weigh-ins must read as "unknown", never as 0 kg.
	LatestWeightKg *float64 `json:"latest_weight_kg"`
	Note           string   `json:"note,omitempty"`
}

func Progress(ctx context.Context, db *storage.DB, profileID string,
	days int) (ProgressOut, error) {

	p, err := db.GetProfile(profileID)
	if err != nil {
		return ProgressOut{}, err
	}
	sets, err := db.Sets(profileID, days)
	if err != nil {
		return ProgressOut{}, err
	}
	out := ProgressOut{Profile: profileID, Days: days,
		MuscleCoverage: map[string]int{},
		TargetSessions: days * p.SessionsPerWeek / 7}
	dates := map[string]bool{}
	for _, s := range sets {
		dates[s.Date] = true
		out.TotalVolumeKg += s.VolumeKg
		out.MuscleCoverage[s.BodyPart]++
	}
	out.LiftingSessions = len(dates)
	out.SetsLogged = len(sets)

	if cardio, err := db.Cardio(profileID, days); err == nil {
		out.CardioSessions = len(cardio)
		for _, c := range cardio {
			out.CardioMinutes += c.Minutes
		}
	}
	if foods, err := db.Foods(profileID, days); err == nil {
		fd := map[string]bool{}
		for _, f := range foods {
			fd[f.Date] = true
		}
		out.FoodDaysLogged = len(fd)
	}
	if ws, err := db.Weights(profileID, days); err == nil && len(ws) > 0 {
		kg := ws[len(ws)-1].WeightKg
		out.LatestWeightKg = &kg
	}
	if out.FoodDaysLogged == 0 && p.Goal == "fat_loss" {
		out.Note = "no food logged — for fat loss this is the half that decides it"
	}
	return out, nil
}

// FindExercises returns what this person can do, sorted by name for stable output.
func FindExercises(ctx context.Context, db *storage.DB, cat *catalog.Catalog,
	profileID, muscle string, limit int) ([]catalog.Exercise, error) {

	p, err := db.GetProfile(profileID)
	if err != nil {
		return nil, err
	}
	var out []catalog.Exercise
	for _, e := range cat.For(p.Equipment, p.MaxDifficulty) {
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/tools/ -v`
Expected: PASS — all three tests

- [ ] **Step 5: Wire the MCP server**

Create `cmd/sehaty/main.go`. Consult the SDK's current README for the exact
registration call — pin the version in `go.mod` and follow its examples rather than
guessing at signatures:

```go
// Command sehaty serves the Sehaty MCP tools over Streamable HTTP.
//
// MCP is a tool-execution surface: anything that reaches it can invoke these tools.
// It binds loopback by default and must never be published. The dashboard, which is
// safe to publish behind Cloudflare Access, is a separate port in a later plan.
package main

import (
	"log"
	"os"
	"path/filepath"

	"github.com/rakasatria/sehaty/internal/catalog"
	"github.com/rakasatria/sehaty/internal/storage"
	"github.com/rakasatria/sehaty/internal/tools"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	dbPath := env("SEHATY_DB", "/var/lib/sehaty/sehaty.db")
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		log.Fatalf("create data dir: %v", err)
	}
	db, err := storage.Open(dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	exPath := env("SEHATY_EXERCISES", "vendor/exercises-dataset/data/exercises.json")
	cat, err := catalog.Load(exPath)
	if err != nil {
		log.Fatalf("load catalog: %v", err)
	}

	host := env("SEHATY_MCP_HOST", "127.0.0.1")
	port := env("SEHATY_MCP_PORT", "8765")
	log.Printf("sehaty MCP on http://%s:%s/mcp — %d exercises", host, port, cat.Count())

	// Register tools with the MCP server and serve Streamable HTTP.
	// See the go-sdk README for the current server construction API.
	if err := tools.Serve(host, port, db, cat); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
```

Add `Serve` to `internal/tools/tools.go`, wiring each handler above as an MCP tool
with its description taken from the spec's §6 tool list.

- [ ] **Step 6: Build and verify the binary starts**

```bash
cd ~/codes/sehaty
go build ./cmd/sehaty
SEHATY_DB=/tmp/t.db SEHATY_EXERCISES=internal/catalog/testdata/exercises.json ./sehaty &
sleep 2
curl -s -o /dev/null -w '%{http_code}\n' -X POST http://127.0.0.1:8765/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}'
```

Expected: `200`, and the response lists the registered tools.

- [ ] **Step 7: Register with Hermes and confirm discovery**

```bash
printf 'n\n' | hermes mcp add sehaty --url http://127.0.0.1:8765/mcp --connect-timeout 30
```

Expected: `✓ Connected! Found N tool(s)`

- [ ] **Step 8: Commit**

```bash
git add cmd/ internal/tools/
git commit -m "feat(mcp): serve deterministic tools over streamable http"
```

---

## What this plan deliberately leaves out

Each is its own plan, because each produces working software on its own and none can be
specified honestly until the interfaces above exist:

- **Plan 2 — AI tools.** `coach_review`, `adapt_session`, `explain_exercise`,
  `translate_exercise`, via OpenRouter. Includes generating `difficulty.json` for all
  1,324 exercises, which Task 5 stubs with four.
- **Plan 3 — voice and photo.** Transcription, vision estimation, the `MediaStore`
  interface with local and S3 backends, and the confirm-before-write rule.
- **Plan 4 — Telegram.** Via Hermes' bundled `telegram-platform`, using `identity` from
  Task 4.
- **Plan 5 — dashboard and publishing.** Separate port, `sehaty.homedev.app` behind
  Cloudflare Access.
