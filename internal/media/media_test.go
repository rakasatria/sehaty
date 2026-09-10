package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rakasatria/sehaty/internal/crypto"
)

func testMaster(t *testing.T) string {
	t.Helper()
	return base64.StdEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
}

func newLocal(t *testing.T) (*Local, string) {
	t.Helper()
	c, err := crypto.NewMedia(testMaster(t))
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	l, err := NewLocal(root, c, 0)
	if err != nil {
		t.Fatal(err)
	}
	return l, root
}

func TestPutGetRoundTrip(t *testing.T) {
	l, _ := newLocal(t)
	ctx := context.Background()
	want := []byte("a meal photo's bytes")

	hash, err := l.Put(ctx, "raka", KindPhoto, bytes.NewReader(want))
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(want)
	if hash != hex.EncodeToString(sum[:]) {
		t.Fatalf("hash %s is not the sha256 of the PLAINTEXT — dedup depends on that", hash)
	}

	rc, err := l.Get(ctx, "raka", KindPhoto, hash)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	got, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("round trip returned %q, want %q", got, want)
	}
}

// The same photo sent twice must store once. Content addressing is what makes that work,
// and it only works if the hash is of the plaintext, not the ciphertext — a fresh GCM
// nonce per write would make identical photos hash differently.
func TestSameBytesStoreOnce(t *testing.T) {
	l, root := newLocal(t)
	ctx := context.Background()
	blob := []byte("the same lunch, photographed twice")

	h1, err := l.Put(ctx, "raka", KindPhoto, bytes.NewReader(blob))
	if err != nil {
		t.Fatal(err)
	}
	h2, err := l.Put(ctx, "raka", KindPhoto, bytes.NewReader(blob))
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Fatalf("same bytes produced different hashes: %s vs %s", h1, h2)
	}
	var files int
	filepath.WalkDir(root, func(_ string, d os.DirEntry, _ error) error {
		if d != nil && !d.IsDir() {
			files++
		}
		return nil
	})
	if files != 1 {
		t.Fatalf("stored %d files for identical bytes, want 1", files)
	}
}

// Media lives OUTSIDE the database, so the Adiantum VFS does not protect it. Without the
// per-blob cipher a meal photo would be a plain JPEG on a disk that is backed up nightly.
func TestBlobIsNotPlaintextOnDisk(t *testing.T) {
	l, root := newLocal(t)
	const secret = "DISTINCTIVE-BYTES-THAT-MUST-NOT-APPEAR"

	if _, err := l.Put(context.Background(), "raka", KindVoice,
		strings.NewReader(secret)); err != nil {
		t.Fatal(err)
	}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if bytes.Contains(raw, []byte(secret)) {
			t.Fatalf("%s holds the blob in plaintext", p)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The AAD binds each blob to one profile. Without it, anyone who could write to the media
// directory could move another person's voice note into their own folder and it would
// decrypt perfectly — the bytes are valid, only the context changed.
func TestBlobCannotBeReadByAnotherProfile(t *testing.T) {
	l, root := newLocal(t)
	ctx := context.Background()
	blob := []byte("private voice note")

	hash, err := l.Put(ctx, "raka", KindVoice, bytes.NewReader(blob))
	if err != nil {
		t.Fatal(err)
	}
	// Physically move the ciphertext into another profile's directory.
	src := filepath.Join(root, "raka", "voice", hash[:2], hash)
	dst := filepath.Join(root, "intruder", "voice", hash[:2], hash)
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("blob not at the documented path %s: %v", src, err)
	}
	if err := os.WriteFile(dst, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Get(ctx, "intruder", KindVoice, hash); err == nil {
		t.Fatal("another profile decrypted the blob — the AAD is not binding profile_id")
	}
}

// An unbounded read is an out-of-memory crash waiting for a large upload.
func TestOversizeIsRejected(t *testing.T) {
	c, err := crypto.NewMedia(testMaster(t))
	if err != nil {
		t.Fatal(err)
	}
	l, err := NewLocal(t.TempDir(), c, 16)
	if err != nil {
		t.Fatal(err)
	}
	_, err = l.Put(context.Background(), "raka", KindPhoto,
		bytes.NewReader(make([]byte, 17)))
	if err == nil {
		t.Fatal("accepted a blob larger than the configured limit")
	}
}

// Altered bytes must fail to decrypt, not return wrong data. GCM gives this for free, and
// the test exists so nobody replaces it with something that does not.
func TestCorruptBlobFailsLoudly(t *testing.T) {
	l, root := newLocal(t)
	ctx := context.Background()
	hash, err := l.Put(ctx, "raka", KindPhoto, strings.NewReader("original"))
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(root, "raka", "photo", hash[:2], hash)
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	raw[len(raw)-1] ^= 0xff
	if err := os.WriteFile(p, raw, 0o640); err != nil {
		t.Fatal(err)
	}
	if _, err := l.Get(ctx, "raka", KindPhoto, hash); err == nil {
		t.Fatal("a tampered blob decrypted successfully")
	}
}

// The profile id becomes a DIRECTORY NAME, so an unchecked id here is path traversal.
func TestInvalidProfileIDIsRejected(t *testing.T) {
	l, _ := newLocal(t)
	for _, bad := range []string{"../etc", "a/b", "", "UPPER", strings.Repeat("x", 40)} {
		if _, err := l.Put(context.Background(), bad, KindPhoto,
			strings.NewReader("x")); err == nil {
			t.Fatalf("accepted invalid profile id %q", bad)
		}
	}
}

func TestUnknownKindIsRejected(t *testing.T) {
	l, _ := newLocal(t)
	if _, err := l.Put(context.Background(), "raka", Kind("../../etc"),
		strings.NewReader("x")); err == nil {
		t.Fatal("accepted an unknown media kind")
	}
}
