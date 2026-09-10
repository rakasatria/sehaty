package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rakasatria/sehaty/internal/agent"
	"github.com/rakasatria/sehaty/internal/catalog"
	"github.com/rakasatria/sehaty/internal/crypto"
	"github.com/rakasatria/sehaty/internal/food"
	"github.com/rakasatria/sehaty/internal/media"
	"github.com/rakasatria/sehaty/internal/storage"
	"github.com/rakasatria/sehaty/internal/tools"
)

// chatCmd is the conversation, on a terminal instead of in Telegram.
//
// It exists so the agent can be exercised against the real database and the real tools
// without sending anything through a chat app — which means a change to the prompt or the
// registry can be checked before a person ever sees it, and checked again afterwards
// against the same words.
//
//	sehatyctl chat <profile-id>            read messages from stdin, one per line
//	sehatyctl chat <profile-id> "message"  a single exchange
func chatCmd(db *storage.DB, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: sehatyctl chat <profile-id> [message]")
	}
	profileID := args[0]
	p, err := db.GetProfile(profileID)
	if err != nil {
		return fmt.Errorf("no such profile %q: %w", profileID, err)
	}

	master, err := crypto.ReadKeyEnv()
	if err != nil {
		return err
	}
	cipher, err := crypto.New(master)
	if err != nil {
		return err
	}
	mediaCipher, err := crypto.NewMedia(master)
	if err != nil {
		return err
	}
	data := env("SEHATY_DATA", "/var/lib/sehaty")
	blobs, err := media.NewLocal(env("SEHATY_MEDIA_DIR", data+"/media"),
		mediaCipher, media.DefaultMaxBytes)
	if err != nil {
		return err
	}
	cat, err := catalog.Load(env("SEHATY_EXERCISES", data+"/exercises.json"))
	if err != nil {
		return err
	}
	foods, err := food.Load(env("SEHATY_FOODS", data+"/tkpi-2020.json"))
	if err != nil {
		return err
	}

	key := os.Getenv("SEHATY_OPENROUTER_KEY")
	if f := os.Getenv("SEHATY_OPENROUTER_KEY_FILE"); f != "" {
		if raw, err := os.ReadFile(f); err == nil {
			key = strings.TrimSpace(string(raw))
		}
	}
	if key == "" {
		return fmt.Errorf("no OpenRouter key, so there is nothing to talk to")
	}

	deps := tools.Deps{DB: db, Cat: cat, Cipher: cipher, Media: blobs, Food: foods}
	c := agent.New(key, env("SEHATY_CHAT_MODEL", agent.DefaultModel), agent.NewRegistry(deps))

	say := func(line string) error {
		// The brief is rebuilt every turn from the database, exactly as the bot does it,
		// so what is tested here is what a person would actually get.
		fresh, err := db.GetProfile(profileID)
		if err != nil {
			return err
		}
		answer, hist, err := c.Respond(context.Background(), profileID, agent.Brief(deps, fresh),
			append(history, agent.Message{Role: "user", Content: line}))
		if err != nil {
			return err
		}
		history = hist
		fmt.Printf("\n%s\n\n", answer)
		return nil
	}

	if len(args) > 1 {
		return say(strings.Join(args[1:], " "))
	}
	fmt.Printf("talking to %s as %s. ctrl-d to stop.\n", c.Model, p.DisplayName)
	sc := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !sc.Scan() {
			return nil
		}
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if err := say(line); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
	}
}

var history []agent.Message
