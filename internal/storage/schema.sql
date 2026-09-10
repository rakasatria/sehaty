CREATE TABLE IF NOT EXISTS profile (
  id                TEXT PRIMARY KEY,
  equipment_json    TEXT NOT NULL DEFAULT '["body weight"]',
  goal              TEXT NOT NULL DEFAULT 'general',
  sessions_per_week INTEGER NOT NULL DEFAULT 3,
  session_minutes   INTEGER NOT NULL DEFAULT 50,
  experience        TEXT NOT NULL DEFAULT 'beginner',
  max_difficulty    INTEGER NOT NULL DEFAULT 3,
  limitations_json  TEXT NOT NULL DEFAULT '[]',
  display_name      TEXT NOT NULL DEFAULT '',
  -- Who this person is, for CONTEXT. Deliberately not used to compute energy targets:
  -- Sehaty refuses to set a calorie or macro goal at all, and having age and height on
  -- file must not quietly become permission to start.
  age               INTEGER NOT NULL DEFAULT 0,
  height_cm         INTEGER NOT NULL DEFAULT 0,
  sex               TEXT NOT NULL DEFAULT '',
  allergies_json    TEXT NOT NULL DEFAULT '[]',
  dislikes_json     TEXT NOT NULL DEFAULT '[]',
  diet_notes        TEXT NOT NULL DEFAULT '',
  diet_preference   TEXT NOT NULL DEFAULT '',
  -- Which questions they have actually answered. Inferring this from the values does not
  -- work: someone who genuinely trains 3 times a week is indistinguishable from someone
  -- who never answered, because 3 is also the default — so they get asked forever.
  answered_json     TEXT NOT NULL DEFAULT '[]',
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


CREATE INDEX IF NOT EXISTS idx_training_profile_date ON training_log(profile_id, date);
CREATE INDEX IF NOT EXISTS idx_cardio_profile_date   ON cardio_log(profile_id, date);
CREATE INDEX IF NOT EXISTS idx_weight_profile_date   ON weight_log(profile_id, date);
CREATE INDEX IF NOT EXISTS idx_food_profile_date     ON food_log(profile_id, date);

-- Documents: versioned Markdown per profile. Replaces secret_blob, which was a flat
-- key/value store with no history — and a nutritionist revising a plan means the old one
-- must stay readable, because "what was I told in September" gets asked.
CREATE TABLE IF NOT EXISTS document (
  profile_id  TEXT NOT NULL REFERENCES profile(id),
  key         TEXT NOT NULL,
  version     INTEGER NOT NULL,
  title       TEXT NOT NULL,
  kind        TEXT NOT NULL,
  body        BLOB NOT NULL,
  encrypted   INTEGER NOT NULL DEFAULT 1,
  updated_at  TEXT NOT NULL,
  PRIMARY KEY (profile_id, key, version)
);

-- Media: metadata here, bytes in the MediaStore. Content-addressed by sha256, so the
-- same photo sent twice stores once.
CREATE TABLE IF NOT EXISTS media (
  hash        TEXT PRIMARY KEY,
  profile_id  TEXT NOT NULL REFERENCES profile(id),
  kind        TEXT NOT NULL,
  mime        TEXT NOT NULL,
  bytes       INTEGER NOT NULL,
  source      TEXT NOT NULL,
  created_at  TEXT NOT NULL,
  transcript  TEXT,
  caption     TEXT
);
CREATE INDEX IF NOT EXISTS idx_media_profile ON media(profile_id, created_at);

-- English names for TKPI foods, supplied by the assistant and correctable by hand.
-- Search metadata only: an alias never affects a nutrition value. An empty name_en is
-- meaningful and records that the food has no common English name.
CREATE TABLE IF NOT EXISTS food_alias (
  code       TEXT PRIMARY KEY,
  name_en    TEXT NOT NULL DEFAULT '',
  source     TEXT NOT NULL DEFAULT 'agent',
  updated_at TEXT NOT NULL
);

-- API credentials. The plaintext token is NEVER stored, only its SHA-256 - anyone who
-- reads a backup gets hashes, not working credentials. profile_id empty means an admin
-- token, which registration needs: a person has no profile until register creates one.
-- Revoked rows are KEPT so the history of what once had access survives.
CREATE TABLE IF NOT EXISTS api_token (
  id          TEXT PRIMARY KEY,
  token_hash  TEXT NOT NULL UNIQUE,
  profile_id  TEXT NOT NULL DEFAULT '',
  label       TEXT NOT NULL DEFAULT '',
  created_at  TEXT NOT NULL,
  revoked_at  TEXT
);
CREATE INDEX IF NOT EXISTS api_token_hash ON api_token(token_hash);

-- What a voice note said. Kept apart from the recording so the audio stays encrypted on
-- disk while the text becomes searchable. source records WHO produced it: a machine
-- transcription is a guess about what was said, a user correction is what was said, and
-- confusing the two would let a guess harden into a record.
CREATE TABLE IF NOT EXISTS media_transcript (
  profile_id TEXT NOT NULL REFERENCES profile(id),
  hash       TEXT NOT NULL,
  text       TEXT NOT NULL,
  source     TEXT NOT NULL DEFAULT '',
  language   TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL,
  PRIMARY KEY (profile_id, hash)
);
