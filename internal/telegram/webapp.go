package telegram

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	tu "github.com/mymmrac/telego/telegoutil"
)

// WebAppMaxAge is how old a Mini App launch may be before it is refused.
//
// Telegram signs initData when the Mini App opens and never expires it — the signature
// stays valid forever. So the string in a webview's URL is a bearer credential for that
// person's health record, and anything that captures one (a shared screen, a proxy log, a
// browser history sync) would hold it indefinitely.
//
// An hour matches the signed-link TTL, so both doors into the dashboard shut at the same
// speed. Reopening the Mini App mints a fresh launch, so the cost of being wrong here is
// a tap.
const WebAppMaxAge = time.Hour

// VerifyWebApp checks a Mini App launch and returns whose it is.
//
// Two checks, and the second is the one that is easy to miss. ValidateWebAppData verifies
// the HMAC-SHA256 chain Telegram signs initData with, which proves the payload came from
// Telegram and was not edited. It says NOTHING about when — an initData string captured
// today validates cleanly next year. auth_date is therefore checked here, against the
// clock passed in so a test can prove it.
func VerifyWebApp(botToken, initData string, now time.Time) (telegramUserID string, err error) {
	if botToken == "" {
		return "", fmt.Errorf("mini app is not configured on this server")
	}
	values, err := tu.ValidateWebAppData(botToken, initData)
	if err != nil {
		return "", fmt.Errorf("this launch did not come from Telegram")
	}

	raw := values.Get(tu.WebAppAuthDate)
	secs, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return "", fmt.Errorf("this launch carries no timestamp")
	}
	age := now.Sub(time.Unix(secs, 0))
	if age > WebAppMaxAge {
		return "", fmt.Errorf("this launch has expired")
	}
	// A launch stamped in the future is either a clock problem or a forgery attempt, and
	// neither is a thing to open a health record on. A minute of slack absorbs ordinary
	// drift between Telegram and this machine.
	if age < -time.Minute {
		return "", fmt.Errorf("this launch is stamped in the future")
	}

	var user struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal([]byte(values.Get(tu.WebAppUser)), &user); err != nil || user.ID == 0 {
		return "", fmt.Errorf("this launch names no user")
	}
	return strconv.FormatInt(user.ID, 10), nil
}

// ResolveWebApp turns a validated launch into a profile id.
//
// Returns one error for every failure — unverified, expired, and "verified but this
// account was never registered here" are indistinguishable to the caller. A page that
// said "you are not registered" would confirm to whoever holds a captured launch string
// that the signature itself was good.
func (b *Bot) ResolveWebApp(initData string, now time.Time) (string, error) {
	uid, err := VerifyWebApp(b.Token, initData, now)
	if err != nil {
		return "", err
	}
	profileID, err := b.Deps.DB.ResolveIdentity(Channel, uid)
	if err != nil {
		return "", fmt.Errorf("no record for this account")
	}
	return profileID, nil
}
