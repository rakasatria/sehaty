package telegram

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"testing"
	"time"
)

const testBotToken = "123456:AAH-test-token-for-signing"

// signLaunch builds an initData string the way Telegram does.
//
// Written out rather than mocked because the whole point of these tests is that a real
// signature is accepted and a real signature that is merely OLD is not. A stub verifier
// would prove neither.
func signLaunch(t *testing.T, botToken string, userID int64, authDate time.Time) string {
	t.Helper()
	v := url.Values{}
	v.Set("user", fmt.Sprintf(`{"id":%d,"first_name":"Raka"}`, userID))
	v.Set("auth_date", fmt.Sprint(authDate.Unix()))
	v.Set("query_id", "AAtest")

	keys := make([]string, 0, len(v))
	for k := range v {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, k := range keys {
		pairs = append(pairs, k+"="+v.Get(k))
	}

	secret := hmac.New(sha256.New, []byte("WebAppData"))
	secret.Write([]byte(botToken))
	mac := hmac.New(sha256.New, secret.Sum(nil))
	mac.Write([]byte(strings.Join(pairs, "\n")))
	v.Set("hash", hex.EncodeToString(mac.Sum(nil)))
	return v.Encode()
}

func TestAGenuineLaunchIsAccepted(t *testing.T) {
	now := time.Now()
	got, err := VerifyWebApp(testBotToken, signLaunch(t, testBotToken, 8412, now), now)
	if err != nil {
		t.Fatalf("a genuine launch was refused: %v", err)
	}
	if got != "8412" {
		t.Errorf("resolved to %q, want 8412", got)
	}
}

// THE test. ValidateWebAppData proves a launch came from Telegram; it says nothing about
// when. An initData string captured today validates cleanly next year, so it is a bearer
// credential for someone's health record unless something checks the clock.
//
// If this test ever fails, a screenshot of a URL becomes permanent access.
func TestAnOldLaunchIsRefusedEvenThoughItsSignatureIsPerfect(t *testing.T) {
	now := time.Now()
	stale := signLaunch(t, testBotToken, 8412, now.Add(-WebAppMaxAge-time.Minute))

	if _, err := VerifyWebApp(testBotToken, stale, now); err == nil {
		t.Fatal("a launch older than the window was accepted on its signature alone")
	}

	// And the boundary holds: comfortably inside the window still works, so the check is
	// a window and not an accident that refuses everything.
	fresh := signLaunch(t, testBotToken, 8412, now.Add(-WebAppMaxAge/2))
	if _, err := VerifyWebApp(testBotToken, fresh, now); err != nil {
		t.Errorf("a launch inside the window was refused: %v", err)
	}
}

// A launch stamped in the future is a clock problem or a forgery, and neither is a thing
// to open a health record on. Ordinary drift must still pass.
func TestAFutureLaunchIsRefusedButDriftIsTolerated(t *testing.T) {
	now := time.Now()
	if _, err := VerifyWebApp(testBotToken,
		signLaunch(t, testBotToken, 8412, now.Add(10*time.Minute)), now); err == nil {
		t.Error("a launch stamped ten minutes ahead was accepted")
	}
	if _, err := VerifyWebApp(testBotToken,
		signLaunch(t, testBotToken, 8412, now.Add(20*time.Second)), now); err != nil {
		t.Errorf("twenty seconds of clock drift was treated as an attack: %v", err)
	}
}

func TestATamperedLaunchIsRefused(t *testing.T) {
	now := time.Now()
	good := signLaunch(t, testBotToken, 8412, now)

	// Swap the user id for someone else's, keeping the original signature.
	tampered := strings.Replace(good, url.QueryEscape(`{"id":8412,"first_name":"Raka"}`),
		url.QueryEscape(`{"id":9999,"first_name":"Raka"}`), 1)
	if tampered == good {
		t.Fatal("the test did not actually change the payload")
	}
	if _, err := VerifyWebApp(testBotToken, tampered, now); err == nil {
		t.Fatal("a launch edited to name another person was accepted")
	}
}

// A launch signed by a different bot must not open this bot's records. This is what stops
// somebody standing up their own bot and pointing its Mini App at this server.
func TestALaunchSignedByAnotherBotIsRefused(t *testing.T) {
	now := time.Now()
	other := signLaunch(t, "999999:BBB-someone-elses-bot", 8412, now)
	if _, err := VerifyWebApp(testBotToken, other, now); err == nil {
		t.Fatal("a launch signed by another bot was accepted")
	}
}

func TestGarbageAndEmptyLaunchesAreRefused(t *testing.T) {
	now := time.Now()
	for _, bad := range []string{"", "hash=", "user=%7B%7D&hash=deadbeef", "&&&", "%zz"} {
		if _, err := VerifyWebApp(testBotToken, bad, now); err == nil {
			t.Errorf("%q was accepted", bad)
		}
	}
	// And no token at all means the door does not open, rather than opening for anyone.
	if _, err := VerifyWebApp("", signLaunch(t, testBotToken, 1, now), now); err == nil {
		t.Error("an unconfigured server accepted a launch")
	}
}
