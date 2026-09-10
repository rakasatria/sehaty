package storage

import (
	"fmt"

	gonanoid "github.com/matoous/go-nanoid/v2"
)

// ProfileIDLength is 21 characters over a 36-symbol alphabet — roughly 108 bits. Far more
// than needed against guessing, and short enough to paste.
const ProfileIDLength = 21

// profileAlphabet is deliberately lowercase-only.
//
// A profile id becomes a DIRECTORY NAME under the media root and part of the AAD binding
// encrypted documents. Nanoid's default alphabet is case-sensitive, which on a
// case-insensitive filesystem (macOS, and community users will run this on macOS) would
// let two distinct profiles collide on one directory. Dropping to 36 symbols costs
// entropy nobody will miss and removes that failure entirely.
const profileAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// NewProfileID generates an unguessable profile id.
//
// The server has no authentication, so the id IS the access control: anything that can
// reach the port can name any profile it can guess. "raka" is guessable; this is not.
// That is a stopgap and not a substitute for real auth — see docs/design.
func NewProfileID() (string, error) {
	id, err := gonanoid.Generate(profileAlphabet, ProfileIDLength)
	if err != nil {
		return "", fmt.Errorf("generate profile id: %w", err)
	}
	return id, nil
}
