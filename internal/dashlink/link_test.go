package dashlink

import (
	"errors"
	"strings"
	"testing"
	"time"
)

var key = []byte("0123456789abcdef0123456789abcdef")

func TestMintedLinkVerifiesAndCarriesItsProfile(t *testing.T) {
	now := time.Unix(1_757_000_000, 0)
	tok, exp := Mint(key, "iap73ghvgcf5d6z206fwl", now)
	if !exp.After(now) {
		t.Fatal("expiry is not in the future")
	}
	if exp.Sub(now) != TTL {
		t.Errorf("ttl = %v, want %v", exp.Sub(now), TTL)
	}
	got, err := Verify(key, tok, now.Add(30*time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if got != "iap73ghvgcf5d6z206fwl" {
		t.Errorf("profile = %q", got)
	}
}

// The whole point of the hour.
func TestLinkStopsWorkingAfterAnHour(t *testing.T) {
	now := time.Unix(1_757_000_000, 0)
	tok, _ := Mint(key, "raka", now)
	if _, err := Verify(key, tok, now.Add(59*time.Minute)); err != nil {
		t.Errorf("rejected at 59 minutes: %v", err)
	}
	if _, err := Verify(key, tok, now.Add(TTL+time.Second)); !errors.Is(err, ErrExpired) {
		t.Errorf("still valid past the hour: %v", err)
	}
}

func TestTamperedLinkIsRejected(t *testing.T) {
	now := time.Unix(1_757_000_000, 0)
	tok, _ := Mint(key, "raka", now)
	body, mac, _ := strings.Cut(tok, ".")

	// Swap the payload for another profile, keeping the signature.
	forged, _ := Mint(key, "someone-else", now)
	otherBody, _, _ := strings.Cut(forged, ".")
	if _, err := Verify(key, otherBody+"."+mac, now); !errors.Is(err, ErrMalformed) {
		t.Error("a swapped payload was accepted")
	}
	// Flip a bit in the signature.
	bad := []byte(mac)
	bad[0] ^= 0x01
	if _, err := Verify(key, body+"."+string(bad), now); !errors.Is(err, ErrMalformed) {
		t.Error("a corrupted signature was accepted")
	}
	// A link minted with a different key must not open this one.
	other, _ := Mint([]byte("ffffffffffffffffffffffffffffffff"), "raka", now)
	if _, err := Verify(key, other, now); !errors.Is(err, ErrMalformed) {
		t.Error("a link signed with another key was accepted")
	}
}

func TestMalformedInputsAreRejected(t *testing.T) {
	now := time.Unix(1_757_000_000, 0)
	for _, tok := range []string{"", ".", "nodot", "!!!.###", "aGVsbG8.", ".abc"} {
		if _, err := Verify(key, tok, now); err == nil {
			t.Errorf("accepted %q", tok)
		}
	}
}

// An expired link must not be extendable by re-signing an old payload with a new expiry —
// the expiry is inside what is signed.
func TestExpiryIsCoveredBySignature(t *testing.T) {
	now := time.Unix(1_757_000_000, 0)
	tok, _ := Mint(key, "raka", now)
	body, mac, _ := strings.Cut(tok, ".")
	raw, _ := enc.DecodeString(body)
	tampered := strings.Replace(string(raw), "|", "|9", 1) // push the expiry far out
	if _, err := Verify(key, enc.EncodeToString([]byte(tampered))+"."+mac, now); err == nil {
		t.Error("an extended expiry kept the original signature valid")
	}
}
