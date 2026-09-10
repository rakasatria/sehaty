package crypto

import (
	"bytes"
	"strings"
	"testing"
)

func newTest(t *testing.T) *Cipher {
	t.Helper()
	k, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	c, err := New(k)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestSealOpenRoundTrip(t *testing.T) {
	c := newTest(t)
	pt := []byte("telur putih 3 atau ayam fillet 100g, tanpa tumis")
	aad := []byte("raka|diet-plan|1")
	ct, err := c.Seal(pt, aad)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ct, pt) {
		t.Fatal("plaintext visible in ciphertext")
	}
	got, err := c.Open(ct, aad)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pt) {
		t.Errorf("got %q, want %q", got, pt)
	}
}

// The attack this defends: someone with database write access copies one profile's
// encrypted document into another profile's row. Without AAD binding it decrypts fine.
func TestCiphertextCannotBeMovedBetweenProfiles(t *testing.T) {
	c := newTest(t)
	ct, _ := c.Seal([]byte("raka's prescription"), []byte("raka|diet-plan|1"))
	if _, err := c.Open(ct, []byte("other|diet-plan|1")); err == nil {
		t.Fatal("ciphertext opened under a different profile — AAD binding is broken")
	}
}

func TestTamperedCiphertextFails(t *testing.T) {
	c := newTest(t)
	aad := []byte("raka|diet-plan|1")
	ct, _ := c.Seal([]byte("100g"), aad)
	ct[len(ct)-1] ^= 0x01
	if _, err := c.Open(ct, aad); err == nil {
		t.Fatal("altered ciphertext decrypted — not authenticated")
	}
}

func TestNoncesAreNotReused(t *testing.T) {
	c := newTest(t)
	aad := []byte("raka|diet-plan|1")
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		ct, _ := c.Seal([]byte("same plaintext"), aad)
		n := string(ct[:12])
		if seen[n] {
			t.Fatal("nonce reused — catastrophic for GCM")
		}
		seen[n] = true
	}
}

func TestRejectsShortOrMissingKey(t *testing.T) {
	if _, err := New(""); err != ErrNoKey {
		t.Errorf("empty key: got %v, want ErrNoKey", err)
	}
	if _, err := New("c2hvcnQ="); err == nil {
		t.Error("short key accepted — should be rejected, never padded")
	} else if !strings.Contains(err.Error(), "AES-256") {
		t.Errorf("error should name the requirement, got: %v", err)
	}
}

func TestGeneratedKeysDiffer(t *testing.T) {
	a, _ := GenerateKey()
	b, _ := GenerateKey()
	if a == b {
		t.Fatal("GenerateKey returned the same key twice")
	}
}
