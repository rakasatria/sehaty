// Package storage owns the database. Every query lives here, and every query is filtered
// by profile_id — that isolation is what stops one person reading another's health data.
// No SQL outside this package.
//
// THE FILE ITSELF IS ENCRYPTED. The driver is github.com/ncruces/go-sqlite3 (a cgo-free
// SQLite: the real C library compiled to WebAssembly and translated to Go) opened through
// its Adiantum VFS, which encrypts every 4 KiB page — main database, WAL and journal
// alike — before it touches the disk.
//
// WHY THIS AND NOT PLAIN SQLITE, since that is what this package used until now. Under
// modernc.org/sqlite there is no SQLCipher, and the LXC containers have no FUSE for
// gocryptfs, so the weight log, the food log and every training note sat in PLAINTEXT
// inside a file that ships to the NAS nightly. Only document bodies were protected. That
// gap is now closed without giving up SQL or taking on cgo.
//
// TWO LAYERS, DIFFERENT JOBS, and the second is not redundant:
//
//	Adiantum (here)      confidentiality of the whole file. Length-preserving, so it is
//	                     NOT authenticated — it hides content, it does not detect tampering.
//	AES-256-GCM (docs)   authenticated, and bound by AAD to one profile/key/version, which
//	                     is what stops a ciphertext being MOVED between profiles.
//
// WHAT IT DOES NOT DEFEND. The key is in the server's environment, so a running server can
// always decrypt. This protects BACKUPS, a STOLEN DISK and a copied container image. It
// does not protect against someone who has compromised the running process.
//
// ONE PRACTICAL CONSEQUENCE: `sqlite3 sehaty.db` no longer works. The file is unreadable
// without the key, including to you while debugging. That is the trade that was accepted.
package storage

import (
	"database/sql"
	_ "embed"
	"fmt"

	_ "github.com/ncruces/go-sqlite3/driver"       // registers database/sql driver "sqlite3"
	_ "github.com/ncruces/go-sqlite3/vfs/adiantum" // registers the "adiantum" VFS
)

//go:embed schema.sql
var schema string

// HexKeyDigits is the length of the Adiantum key: 32 bytes as hex.
const HexKeyDigits = 64

type DB struct{ *sql.DB }

// Open opens (creating if needed) the encrypted database at path and applies the schema.
//
// hexKey comes from crypto.DBKeyFromEnv(). It is REQUIRED and there is no plaintext mode:
// an optional key here would mean a database that silently stores health data in the
// clear, which is the exact failure this driver was adopted to prevent.
//
// Opening an existing database with the WRONG key fails with "file is not a database" —
// the pages decrypt to noise and the header no longer matches. That is expected, and it
// is why the key must be backed up somewhere other than the server it protects.
func Open(path, hexKey string) (*DB, error) {
	if len(hexKey) != HexKeyDigits {
		return nil, fmt.Errorf("database key must be %d hex digits, got %d",
			HexKeyDigits, len(hexKey))
	}
	sqlDB, err := sql.Open("sqlite3", "file:"+path+"?vfs=adiantum&hexkey="+hexKey)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	// WAL lets readers continue during a write, which a dashboard polling while a tool
	// logs a set will do constantly. Foreign keys are load-bearing: the identity table
	// relies on one to reject a link to a profile that does not exist.
	for _, pragma := range []string{
		`PRAGMA busy_timeout=5000`,
		`PRAGMA journal_mode=WAL`,
		`PRAGMA foreign_keys=ON`,
	} {
		if _, err := sqlDB.Exec(pragma); err != nil {
			sqlDB.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}
	if _, err := sqlDB.Exec(schema); err != nil {
		sqlDB.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	for _, m := range migrations {
		if err := ensureColumn(sqlDB, m.table, m.column, m.definition); err != nil {
			sqlDB.Close()
			return nil, err
		}
	}
	return &DB{sqlDB}, nil
}

// migrations are columns added after the first release.
//
// CREATE TABLE IF NOT EXISTS does NOTHING to a table that already exists, so a column added
// to schema.sql never reaches a database created by an earlier build. Deploying such a
// build against a live database fails at the first query with "no such column" — which is
// precisely the state srvdev01 would have been in.
var migrations = []struct{ table, column, definition string }{
	{"profile", "limitations_json", "TEXT NOT NULL DEFAULT '[]'"},
	{"profile", "display_name", "TEXT NOT NULL DEFAULT ''"},
	{"profile", "age", "INTEGER NOT NULL DEFAULT 0"},
	{"profile", "height_cm", "INTEGER NOT NULL DEFAULT 0"},
	{"profile", "sex", "TEXT NOT NULL DEFAULT ''"},
	{"profile", "allergies_json", "TEXT NOT NULL DEFAULT '[]'"},
	{"profile", "dislikes_json", "TEXT NOT NULL DEFAULT '[]'"},
	{"profile", "diet_notes", "TEXT NOT NULL DEFAULT ''"},
	{"profile", "diet_preference", "TEXT NOT NULL DEFAULT ''"},
	{"profile", "answered_json", "TEXT NOT NULL DEFAULT '[]'"},
}

// ensureColumn adds a column if the table lacks it. Idempotent: safe on every startup.
func ensureColumn(db *sql.DB, table, column, definition string) error {
	rows, err := db.Query(`SELECT name FROM pragma_table_info(?)`, table)
	if err != nil {
		return fmt.Errorf("inspect %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	// Identifiers cannot be bound as parameters; they are constants in `migrations`,
	// never user input.
	if _, err := db.Exec("ALTER TABLE " + table + " ADD COLUMN " + column + " " + definition); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, column, err)
	}
	return nil
}
