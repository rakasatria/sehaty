package crypto

import (
	"encoding/base64"
	"encoding/hex"
	"testing"
)

// The file cipher (Adiantum) and the document cipher (AES-256-GCM) must never run under
// the same bytes. If HKDF were dropped or misapplied, this is what would catch it.
func TestSubkeysDifferFromMasterAndFromEachOther(t *testing.T) {
	master, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.StdEncoding.DecodeString(master)
	if err != nil {
		t.Fatal(err)
	}

	dbHex, err := DBKeyHex(master)
	if err != nil {
		t.Fatal(err)
	}
	if dbHex == hex.EncodeToString(raw) {
		t.Fatal("database key is the master key verbatim — HKDF is not being applied")
	}
	docKey, err := derive(raw, infoDocs)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(docKey) == dbHex {
		t.Fatal("database and document subkeys are identical — key reuse across two ciphers")
	}
}

// Derivation must be deterministic. A random salt here would produce a different key on
// every start, and the database would be unopenable after the first restart — data loss
// that would not show up until a reboot.
func TestDBKeyIsDeterministic(t *testing.T) {
	master, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	a, err := DBKeyHex(master)
	if err != nil {
		t.Fatal(err)
	}
	b, err := DBKeyHex(master)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatal("DBKeyHex is not deterministic — the database would not reopen after a restart")
	}
	if len(a) != 64 {
		t.Fatalf("database key is %d hex digits, want 64", len(a))
	}
}

func TestShortMasterKeyIsRejected(t *testing.T) {
	short := base64.StdEncoding.EncodeToString([]byte("too-short"))
	if _, err := DBKeyHex(short); err == nil {
		t.Fatal("accepted a master key that is not 32 bytes")
	}
}
