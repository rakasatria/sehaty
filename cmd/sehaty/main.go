// Command sehaty serves the Sehaty MCP tools over Streamable HTTP.
//
// MCP is a TOOL-EXECUTION surface: anything that reaches it can invoke these tools. It
// binds loopback by default and must never be published. The dashboard — which is safe to
// publish behind an auth proxy — is a separate port.
//
// ON-DISK LAYOUT. Everything mutable lives under ONE root so that "back up Sehaty" is a
// single path, and the key lives OUTSIDE it so that backing up Sehaty never backs up the
// means to read it:
//
//	SEHATY_DATA       /var/lib/sehaty            root of all mutable state  (BACKED UP)
//	                  ├── sehaty.db              encrypted SQLite + -wal + -shm
//	                  └── media/                 SEHATY_MEDIA
//	                      └── <profile_id>/      one directory per person
//	                          ├── photo/<ab>/<sha256>
//	                          └── voice/<ab>/<sha256>
//	SEHATY_KEY_FILE   /etc/sehaty/key            mode 0400  (NOT under SEHATY_DATA)
//	SEHATY_EXERCISES  /opt/sehaty/exercises.json read-only dataset, ships with the build
//
// <profile_id> is constrained by storage.ValidID to ^[a-z0-9][a-z0-9_-]{0,30}$ — an
// unchecked id here would be a path-traversal bug, since the id becomes a directory name.
// <ab> is the first two hex digits of the content hash, sharding so no directory grows to
// hundreds of thousands of entries.
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rakasatria/sehaty/internal/agent"
	"github.com/rakasatria/sehaty/internal/catalog"
	"github.com/rakasatria/sehaty/internal/crypto"
	"github.com/rakasatria/sehaty/internal/dashboard"
	"github.com/rakasatria/sehaty/internal/food"
	"github.com/rakasatria/sehaty/internal/media"
	"github.com/rakasatria/sehaty/internal/storage"
	"github.com/rakasatria/sehaty/internal/telegram"
	"github.com/rakasatria/sehaty/internal/tools"
	"github.com/rakasatria/sehaty/internal/transcribe"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// assertKeyOutsideData refuses to start if the key file sits inside the data root.
//
// This is the one configuration mistake that silently voids the entire scheme: the data
// root is what vzdump ships to the NAS every night, so a key stored inside it travels in
// the same archive as the ciphertext it unlocks, and the encryption becomes decoration.
// It is far easier to make this mistake than to notice it, so it is checked rather than
// documented and hoped for.
func assertKeyOutsideData(keyPath, dataDir string) error {
	if keyPath == "" {
		return nil
	}
	key, err := filepath.Abs(keyPath)
	if err != nil {
		return err
	}
	data, err := filepath.Abs(dataDir)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(data, key)
	if err != nil {
		return nil // different volumes: not inside
	}
	if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return &keyInsideDataError{key: key, data: data}
	}
	return nil
}

type keyInsideDataError struct{ key, data string }

func (e *keyInsideDataError) Error() string {
	return "SEHATY_KEY_FILE (" + e.key + ") is inside SEHATY_DATA (" + e.data + "): " +
		"the key would be included in every backup of the data it encrypts. " +
		"Move it to /etc/sehaty/key"
}

func main() {
	dataDir := env("SEHATY_DATA", "/var/lib/sehaty")
	dbPath := env("SEHATY_DB", filepath.Join(dataDir, "sehaty.db"))
	mediaDir := env("SEHATY_MEDIA", filepath.Join(dataDir, "media"))

	if err := assertKeyOutsideData(os.Getenv("SEHATY_KEY_FILE"), dataDir); err != nil {
		log.Fatal(err)
	}
	for _, dir := range []string{filepath.Dir(dbPath)} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			log.Fatalf("create %s: %v", dir, err)
		}
	}

	// The key is REQUIRED and no longer only for documents: the database file itself is
	// encrypted now, so without it there is nothing to open. Starting anyway — as this
	// used to, logging "documents disabled" — would now mean starting with no storage at
	// all, or worse, silently creating a second empty database.
	master, err := crypto.ReadKeyEnv()
	if err != nil {
		log.Fatalf("no key: %v\n  set SEHATY_KEY_FILE=/etc/sehaty/key (preferred) or SEHATY_KEY", err)
	}
	dbKey, err := crypto.DBKeyHex(master)
	if err != nil {
		log.Fatalf("derive database key: %v", err)
	}
	cipher, err := crypto.New(master)
	if err != nil {
		log.Fatalf("derive document key: %v", err)
	}

	// A third subkey, independent of the other two. Media lives OUTSIDE the database
	// file, so the Adiantum VFS does not reach it — without this a meal photo would be a
	// plain JPEG. Built here rather than lazily so a bad media root fails at startup
	// instead of at the first photo, months later.
	mediaCipher, err := crypto.NewMedia(master)
	if err != nil {
		log.Fatalf("derive media key: %v", err)
	}
	blobs, err := media.NewLocal(mediaDir, mediaCipher, 0)
	if err != nil {
		log.Fatalf("open media store: %v", err)
	}

	db, err := storage.Open(dbPath, dbKey)
	if err != nil {
		log.Fatalf("open database: %v\n  a wrong key reads as \"file is not a database\"", err)
	}
	defer db.Close()

	cat, err := catalog.Load(env("SEHATY_EXERCISES", "/opt/sehaty/exercises.json"))
	if err != nil {
		log.Fatalf("load exercise catalog: %v", err)
	}

	// The Indonesian food table. Loaded at startup so a missing or corrupt file fails
	// loudly here rather than at the first meal someone tries to log.
	foods, err := food.Load(env("SEHATY_FOODS", "/opt/sehaty/tkpi-2020.json"))
	if err != nil {
		log.Fatalf("load food table: %v", err)
	}

	pass := os.Getenv("SEHATY_PASSPHRASE")
	if pass == "" {
		log.Print("registration CLOSED: SEHATY_PASSPHRASE is not set")
	}

	// The MCP token gates the whole tool surface. Read from a file by preference, for the
	// same reason the master key is: an environment variable shows up in `ps`, in
	// `systemctl show` and in most logging.
	token := os.Getenv("SEHATY_TOKEN")
	if f := os.Getenv("SEHATY_TOKEN_FILE"); f != "" {
		raw, err := os.ReadFile(f)
		if err != nil {
			log.Fatalf("read SEHATY_TOKEN_FILE %s: %v", f, err)
		}
		token = strings.TrimSpace(string(raw))
	}
	if token == "" {
		log.Print("WARNING: no SEHATY_TOKEN — the tool surface is OPEN to anything that " +
			"can reach this port, and every tool takes a profile id as an argument")
	}

	addr := env("SEHATY_MCP_HOST", "127.0.0.1") + ":" + env("SEHATY_MCP_PORT", "8765")
	log.Printf("sehaty on http://%s/mcp — db=%s (encrypted) media=%s — %d exercises (%d rated)",
		addr, dbPath, mediaDir, cat.Count(), cat.Rated())
	log.Printf("food table: %d foods (%d flagged as unverified)", foods.Count(), foods.Flagged())
	log.Printf("auth: %s", map[bool]string{true: "token required", false: "OPEN"}[token != ""])

	// The dashboard signs its links with its own subkey and listens on its OWN PORT.
	// MCP is tool execution — anything reaching it can WRITE to a health record — and
	// must never be published. The dashboard is read-only and safe behind a tunnel, so
	// the two must not share a listener.
	dashKey, err := crypto.DashboardKey(master)
	if err != nil {
		log.Fatalf("derive dashboard key: %v", err)
	}
	dashAddr := env("SEHATY_DASH_HOST", "0.0.0.0") + ":" + env("SEHATY_DASH_PORT", "8766")
	dashBase := env("SEHATY_DASH_URL", "http://"+env("SEHATY_DASH_ADVERTISE", "10.254.1.105")+":"+env("SEHATY_DASH_PORT", "8766"))

	// Transcription and conversation are the ONLY things that send data off this
	// machine. Both are off unless a key is configured, and the startup lines say which.
	orKey := os.Getenv("SEHATY_OPENROUTER_KEY")
	if f := os.Getenv("SEHATY_OPENROUTER_KEY_FILE"); f != "" {
		if raw, err := os.ReadFile(f); err == nil {
			orKey = strings.TrimSpace(string(raw))
		} else {
			log.Printf("openrouter key unreadable, transcription and chat disabled: %v", err)
		}
	}
	stt := transcribe.New(orKey, env("SEHATY_TRANSCRIBE_MODEL", transcribe.DefaultModel))
	if stt.Enabled() {
		log.Printf("voice: transcription via %s (audio leaves this machine)", stt.Model)
	} else {
		log.Print("voice: notes are stored but NOT transcribed (no OpenRouter key)")
	}

	d := tools.Deps{DB: db, Cat: cat, Cipher: cipher, Media: blobs, Food: foods,
		DashKey: dashKey, DashBase: dashBase, Transcriber: stt}

	// The Mini App resolver needs the bot, and the bot needs the dependencies built
	// below it, so this is a late-bound hook rather than a value. Nil until Telegram is
	// configured, which is exactly when the /app routes should not exist.
	var tgBot *telegram.Bot
	webAppResolver := func(initData string, now time.Time) (string, error) {
		if tgBot == nil {
			return "", fmt.Errorf("telegram is not configured on this server")
		}
		return tgBot.ResolveWebApp(initData, now)
	}

	go func() {
		log.Printf("dashboard on %s — links valid %s, no other way in", dashAddr, "1h")
		if err := http.ListenAndServe(dashAddr, dashboard.Handler(
			dashboard.Deps{DB: db, Key: dashKey, WebApp: webAppResolver})); err != nil {
			log.Printf("dashboard stopped: %v", err)
		}
	}()

	// English names the assistant supplied on earlier runs. Applied here so the work is
	// done once rather than repeated every restart.
	if n, err := tools.LoadFoodAliases(d); err != nil {
		log.Printf("food aliases: %v", err)
	} else if n > 0 {
		log.Printf("food aliases: %d English names restored", n)
	}

	// Telegram, if configured. LONG POLLING, not webhooks: a webhook needs a public
	// HTTPS endpoint, which would mean exposing this server to the internet — the one
	// thing the whole design avoids. Polling reaches out from behind the LAN.
	tgToken := os.Getenv("SEHATY_TELEGRAM_TOKEN")
	if f := os.Getenv("SEHATY_TELEGRAM_TOKEN_FILE"); f != "" {
		if raw, err := os.ReadFile(f); err == nil {
			tgToken = strings.TrimSpace(string(raw))
		} else {
			log.Printf("telegram disabled: %v", err)
		}
	}
	// The conversational layer. It reaches only the curated registry — twelve tools,
	// none of which takes a profile argument — so a model that hallucinates an id
	// cannot name somebody else's record.
	chat := agent.New(orKey, env("SEHATY_CHAT_MODEL", agent.DefaultModel), agent.NewRegistry(d))
	if chat.Enabled() {
		log.Printf("chat: %s (messages leave this machine)", chat.Model)
	} else {
		log.Print("chat: disabled, the bot answers commands only (no OpenRouter key)")
	}

	tgBot = &telegram.Bot{Token: tgToken, Deps: d, Passphrase: pass, Agent: chat,
		DashURL: dashBase}
	go tgBot.Run(context.Background())

	if err := tools.Serve(addr, d, pass, token); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
