package media

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// Blob is one stored file, as seen from outside.
type Blob struct {
	Hash  string `json:"hash"`
	Kind  Kind   `json:"kind"`
	Bytes int64  `json:"bytes"` // ciphertext size on disk, slightly larger than the original
}

// List returns a profile's blobs of one kind, newest first.
//
// Without this a photo can be stored and never found again: content addressing means the
// hash IS the name, and nobody remembers a hash. Listing walks the profile's own subtree
// only — one person's directory is never opened while listing another's.
func (l *Local) List(ctx context.Context, profileID string, kind Kind) ([]Blob, error) {
	if err := check(profileID, kind); err != nil {
		return nil, err
	}
	root := filepath.Join(l.root, profileID, string(kind))
	var out []Blob
	type stamped struct {
		Blob
		mod int64
	}
	var all []stamped
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil // nothing stored yet is not an error
			}
			return err
		}
		if d.IsDir() || len(d.Name()) != 64 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		all = append(all, stamped{Blob{Hash: d.Name(), Kind: kind, Bytes: info.Size()},
			info.ModTime().UnixNano()})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].mod > all[j].mod })
	for _, s := range all {
		out = append(out, s.Blob)
	}
	return out, nil
}
