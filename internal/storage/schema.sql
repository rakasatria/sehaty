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
