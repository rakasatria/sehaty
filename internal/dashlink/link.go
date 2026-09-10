// Package dashlink mints and checks the short-lived links that open the dashboard.
//
// A link is the ONLY way in. The dashboard carries one person's health record, so it must
// not sit behind a guessable URL, a password someone reuses, or a session that lasts
// forever. The agent mints a link when asked, it works for an hour, and then it does not.
//
// Stateless by design: the token carries its own profile and expiry, signed. Nothing is
// stored, so there is no table of live sessions to leak, grow, or forget to clean up — and
// a restart does not silently extend anyone's access.
package dashlink

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// TTL is how long a link works. An hour is long enough to read a dashboard and short
// enough that a link left in a chat log, a screenshot or a browser history is inert by the
// time anyone finds it.
const TTL = time.Hour

var (
	ErrMalformed = errors.New("link is not valid")
	ErrExpired   = errors.New("link has expired")
)

var enc = base64.RawURLEncoding

// Mint returns a token granting read access to one profile until now+TTL.
func Mint(key []byte, profileID string, now time.Time) (string, time.Time) {
	expiry := now.Add(TTL)
	payload := profileID + "|" + strconv.FormatInt(expiry.Unix(), 10)
	return enc.EncodeToString([]byte(payload)) + "." + sign(key, payload), expiry
}

func sign(key []byte, payload string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(payload))
	// Half the digest: 128 bits is far beyond forgeable and keeps the URL short enough
	// to paste into a chat without wrapping.
	return enc.EncodeToString(m.Sum(nil)[:16])
}

// Verify returns the profile a token grants access to.
//
// The signature is checked BEFORE the expiry, and both failures return distinct errors to
// the caller but the same message to a visitor: telling someone their forged link is
// merely "expired" would confirm the profile id inside it was real.
func Verify(key []byte, token string, now time.Time) (string, error) {
	body, mac, ok := strings.Cut(token, ".")
	if !ok {
		return "", ErrMalformed
	}
	raw, err := enc.DecodeString(body)
	if err != nil {
		return "", ErrMalformed
	}
	payload := string(raw)
	profileID, exp, ok := strings.Cut(payload, "|")
	if !ok || profileID == "" {
		return "", ErrMalformed
	}
	if !hmac.Equal([]byte(mac), []byte(sign(key, payload))) {
		return "", ErrMalformed
	}
	unix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil {
		return "", ErrMalformed
	}
	if now.After(time.Unix(unix, 0)) {
		return "", fmt.Errorf("%w", ErrExpired)
	}
	return profileID, nil
}
