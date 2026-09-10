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
		fmt.Fprintln(os.Stderr, "usage: sehatyctl profiles | delete-profile <id> --yes-really-delete")
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

	default:
		fail(fmt.Errorf("unknown command %q", args[0]))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "sehatyctl:", err)
	os.Exit(1)
}
