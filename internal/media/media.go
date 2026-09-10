// Package media stores voice notes and meal photos.
//
// These blobs live OUTSIDE the database, so the Adiantum VFS that encrypts sehaty.db does
// not reach them. Without the per-blob cipher here, a meal photo would be a plain JPEG on
// a disk that is backed up to the NAS nightly. Every blob is therefore sealed with
// AES-256-GCM under its own HKDF subkey (sehaty:media:aes256gcm:v1), independent of the
// database and document keys.
//
// gocryptfs may be mounted underneath this directory as a second layer — it also hides
// filenames, which content addressing does not. That is deployment hardening, not a
// substitute: it is Linux-and-FUSE only, so it cannot be the mechanism a community user on
// Docker, macOS or S3 relies on.
package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rakasatria/sehaty/internal/crypto"
	"github.com/rakasatria/sehaty/internal/storage"
)

type Kind string

const (
	KindPhoto Kind = "photo"
	KindVoice Kind = "voice"
)

func (k Kind) valid() bool { return k == KindPhoto || k == KindVoice }

// DefaultMaxBytes bounds a single blob. A phone photo is 2-5 MB and a voice note far less;
// 25 MiB leaves headroom without letting one upload decide the server's memory ceiling.
const DefaultMaxBytes = 25 << 20

var (
	ErrNotFound = errors.New("no such blob")
	ErrTooLarge = errors.New("blob exceeds the size limit")
)

// Store is the seam the S3 backend will implement. Nothing outside this package knows
// whether a blob is on a disk or in a bucket.
type Store interface {
	Put(ctx context.Context, profileID string, kind Kind, r io.Reader) (string, error)
	Get(ctx context.Context, profileID string, kind Kind, hash string) (io.ReadCloser, error)
	Exists(ctx context.Context, profileID string, kind Kind, hash string) (bool, error)
}

type Local struct {
	root string
	c    *crypto.Cipher
	max  int64
}

var _ Store = (*Local)(nil)

func NewLocal(root string, c *crypto.Cipher, maxBytes int64) (*Local, error) {
	if c == nil {
		return nil, crypto.ErrNoKey
	}
	if root == "" {
		return nil, errors.New("media root is required")
	}
	if maxBytes <= 0 {
		maxBytes = DefaultMaxBytes
	}
	if err := os.MkdirAll(root, 0o750); err != nil {
		return nil, fmt.Errorf("create media root %s: %w", root, err)
	}
	return &Local{root: root, c: c, max: maxBytes}, nil
}

// aad binds a blob to one profile and one kind. Without it the ciphertext could simply be
// MOVED into another profile's directory and would decrypt perfectly — the bytes are
// valid, only the context changed.
func aad(profileID string, kind Kind, hash string) []byte {
	return []byte(profileID + "|" + string(kind) + "|" + hash)
}

// check validates the addressing inputs. This is the ONLY place a profile id becomes a
// filesystem path, so it is the only place path traversal can enter.
func check(profileID string, kind Kind) error {
	if !storage.ValidID(profileID) {
		return fmt.Errorf("invalid profile id %q", profileID)
	}
	if !kind.valid() {
		return fmt.Errorf("unknown media kind %q", kind)
	}
	return nil
}

func validHash(hash string) error {
	if len(hash) != sha256.Size*2 {
		return fmt.Errorf("hash %q is %d chars, want %d", hash, len(hash), sha256.Size*2)
	}
	if _, err := hex.DecodeString(hash); err != nil {
		return fmt.Errorf("hash %q is not hex: %w", hash, err)
	}
	return nil
}

func (l *Local) path(profileID string, kind Kind, hash string) (string, error) {
	if err := check(profileID, kind); err != nil {
		return "", err
	}
	if err := validHash(hash); err != nil {
		return "", err
	}
	// <ab> shards the directory so no one profile's photo folder grows to hundreds of
	// thousands of entries, which is where ext4 lookups start to hurt.
	return filepath.Join(l.root, profileID, string(kind), hash[:2], hash), nil
}

// Put stores r and returns the SHA-256 of its PLAINTEXT.
//
// Hashing the plaintext rather than the ciphertext is what makes deduplication work: GCM
// uses a fresh nonce per seal, so the same photo encrypted twice produces different bytes
// and would otherwise be stored twice.
func (l *Local) Put(ctx context.Context, profileID string, kind Kind, r io.Reader) (string, error) {
	if err := check(profileID, kind); err != nil {
		return "", err
	}
	// LimitReader with max+1 so "exactly at the limit" is distinguishable from "over" —
	// an unbounded ReadAll here is an out-of-memory crash waiting for a large upload.
	plain, err := io.ReadAll(io.LimitReader(r, l.max+1))
	if err != nil {
		return "", fmt.Errorf("read blob: %w", err)
	}
	if int64(len(plain)) > l.max {
		return "", fmt.Errorf("%w: limit is %d bytes", ErrTooLarge, l.max)
	}

	sum := sha256.Sum256(plain)
	hash := hex.EncodeToString(sum[:])
	p, err := l.path(profileID, kind, hash)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(p); err == nil {
		return hash, nil // already stored — content addressing means it is the same blob
	}

	ct, err := l.c.Seal(plain, aad(profileID, kind, hash))
	if err != nil {
		return "", fmt.Errorf("encrypt blob: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		return "", err
	}
	// Write to a temporary file and rename. Rename is atomic within a filesystem, so a
	// crash mid-write can never leave a truncated blob at a path that content addressing
	// would treat as complete and correct.
	tmp, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(ct); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	if err := os.Chmod(tmp.Name(), 0o640); err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), p); err != nil {
		return "", fmt.Errorf("store blob: %w", err)
	}
	return hash, nil
}

func (l *Local) Get(ctx context.Context, profileID string, kind Kind, hash string) (io.ReadCloser, error) {
	p, err := l.path(profileID, kind, hash)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%s/%s/%s: %w", profileID, kind, hash, ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	plain, err := l.c.Open(raw, aad(profileID, kind, hash))
	if err != nil {
		return nil, fmt.Errorf("open blob %s: %w", hash, err)
	}
	return io.NopCloser(bytes.NewReader(plain)), nil
}

func (l *Local) Exists(ctx context.Context, profileID string, kind Kind, hash string) (bool, error) {
	p, err := l.path(profileID, kind, hash)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}
