// Command sehatyctl is the human administration surface.
//
// Everything here is deliberately NOT an MCP tool. Destructive operations should not be
// reachable by a language model that might read "delete that entry" as "delete
// everything" — a person runs these from a shell, where the blast radius is visible and
// the confirmation is theirs.
//
//	sehatyctl profiles
//	sehatyctl delete-profile <id> --yes-really-delete
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rakasatria/sehaty/internal/crypto"
	"github.com/rakasatria/sehaty/internal/storage"
)

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func open() (*storage.DB, error) {
	master, err := crypto.ReadKeyEnv()
	if err != nil {
		return nil, fmt.Errorf("no key: %w", err)
	}
	dbKey, err := crypto.DBKeyHex(master)
	if err != nil {
		return nil, err
	}
	data := env("SEHATY_DATA", "/var/lib/sehaty")
	return storage.Open(env("SEHATY_DB", data+"/sehaty.db"), dbKey)
}

func main() {
	// Flags are parsed PER SUBCOMMAND. Go's top-level flag.Parse stops at the first
	// positional argument, so "delete-profile raka --yes-really-delete" left the flag
	// unparsed and the command silently did nothing while appearing to accept it —
	// the worst possible failure mode for a delete tool.
	args := os.Args[1:]
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage:\n"+
			"  sehatyctl profiles\n"+
			"  sehatyctl delete-profile <id> --yes-really-delete\n"+
			"  sehatyctl token issue <profile-id|--admin> [label]\n"+
			"  sehatyctl token list\n"+
			"  sehatyctl token revoke <token-id>\n"+
			"  sehatyctl token reset <profile-id>   (revoke every token for one person)")
		os.Exit(2)
	}

	db, err := open()
	if err != nil {
		fmt.Fprintln(os.Stderr, "sehatyctl:", err)
		os.Exit(1)
	}
	defer db.Close()

	switch args[0] {
	case "profiles":
		ps, err := db.ListProfiles()
		if err != nil {
			fail(err)
		}
		for _, p := range ps {
			fmt.Printf("%-22s %-16s goal=%-12s equipment=%v\n",
				p.ID, p.DisplayName, p.Goal, p.Equipment)
		}
		fmt.Printf("%d profile(s)\n", len(ps))

	case "delete-profile":
		id, confirmed := parseDelete(args[1:])
		if id == "" {
			fail(fmt.Errorf("usage: sehatyctl delete-profile <id> [--yes-really-delete]"))
		}
		p, err := db.GetProfile(id)
		if err != nil {
			fail(err)
		}
		// Say what is about to go, before it goes.
		sets, _ := db.Sets(id, 36500)
		weights, _ := db.Weights(id, 36500)
		foods, _ := db.Foods(id, 36500)
		cardio, _ := db.Cardio(id, 36500)
		docs, _ := db.ListDocuments(id)
		fmt.Printf("profile      %s (%s)\n", p.ID, p.DisplayName)
		fmt.Printf("training     %d entries\nweight       %d entries\nfood         %d entries\n"+
			"cardio       %d entries\ndocuments    %d (all versions)\n",
			len(sets), len(weights), len(foods), len(cardio), len(docs))
		if !confirmed {
			fmt.Println("\nnothing deleted. Re-run with --yes-really-delete to proceed.")
			return
		}
		if err := db.DeleteProfile(id); err != nil {
			fail(err)
		}
		fmt.Println("\ndeleted.")

	case "token":
		if len(args) < 2 {
			fail(fmt.Errorf("token needs a subcommand: issue, list, revoke or reset"))
		}
		switch args[1] {
		case "issue":
			if len(args) < 3 {
				fail(fmt.Errorf("token issue needs a profile id, or --admin"))
			}
			profile, label := args[2], ""
			if profile == "--admin" {
				profile = ""
			}
			if len(args) > 3 {
				label = strings.Join(args[3:], " ")
			}
			secret, tok, err := db.IssueToken(profile, label)
			if err != nil {
				fail(err)
			}
			who := tok.ProfileID
			if who == "" {
				who = "(admin — may act on any profile)"
			}
			fmt.Printf("token id   %s\nprofile    %s\nlabel      %s\n\n%s\n\n",
				tok.ID, who, tok.Label, secret)
			// Only moment this string exists outside the caller's hands: the database
			// stores its hash, so a lost token is reissued, never recovered.
			fmt.Println("Copy it now — only the hash is stored, so it cannot be shown again.")

		case "list":
			toks, err := db.ListTokens()
			if err != nil {
				fail(err)
			}
			for _, t := range toks {
				who := t.ProfileID
				if who == "" {
					who = "(admin)"
				}
				state := "active"
				if t.Revoked() {
					state = "revoked " + t.RevokedAt[:10]
				}
				fmt.Printf("%-14s %-24s %-9s %-10s %s\n",
					t.ID, who, state, t.CreatedAt[:10], t.Label)
			}
			fmt.Printf("%d token(s)\n", len(toks))

		case "revoke":
			if len(args) < 3 {
				fail(fmt.Errorf("token revoke needs a token id (see: sehatyctl token list)"))
			}
			if err := db.RevokeToken(args[2]); err != nil {
				fail(err)
			}
			fmt.Println("revoked.")

		case "reset":
			if len(args) < 3 {
				fail(fmt.Errorf("token reset needs a profile id"))
			}
			n, err := db.RevokeProfileTokens(args[2])
			if err != nil {
				fail(err)
			}
			fmt.Printf("revoked %d token(s) for %s. Issue a new one with: "+
				"sehatyctl token issue %s\n", n, args[2], args[2])

		default:
			fail(fmt.Errorf("unknown token subcommand %q", args[1]))
		}

	default:
		fail(fmt.Errorf("unknown command %q", args[0]))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "sehatyctl:", err)
	os.Exit(1)
}
