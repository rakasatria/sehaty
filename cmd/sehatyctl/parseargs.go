package main

import "strings"

// parseDelete pulls the profile id and the confirmation out of the argument list.
//
// Go's flag package — top level AND FlagSet — stops parsing at the first positional
// argument, so "delete-profile raka --yes-really-delete" leaves the flag unset. The
// command then prints what it would delete and does nothing, while looking like it
// accepted the confirmation. That is the worst failure mode a delete tool can have, so
// the confirmation is scanned for explicitly and accepted in ANY position.
func parseDelete(args []string) (id string, confirm bool) {
	for _, a := range args {
		switch {
		case a == "--yes-really-delete" || a == "-yes-really-delete":
			confirm = true
		case strings.HasPrefix(a, "-"):
			// ignore unknown flags rather than silently treating them as an id
		case id == "":
			id = a
		}
	}
	return id, confirm
}
