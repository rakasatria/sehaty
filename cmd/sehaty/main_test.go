package main

import (
	"path/filepath"
	"testing"
)

// The one configuration mistake that silently voids the entire scheme: a key stored
// inside the directory that gets backed up travels in the same archive as the ciphertext
// it unlocks. Easy to make, hard to notice, so it is checked at startup rather than
// documented and hoped for.
func TestKeyInsideDataDirIsRefused(t *testing.T) {
	data := t.TempDir()
	sibling := filepath.Join(filepath.Dir(data), "key")

	for _, tc := range []struct {
		name      string
		key       string
		wantError bool
	}{
		{"directly inside the data dir", filepath.Join(data, "key"), true},
		{"nested inside the data dir", filepath.Join(data, "secrets", "key"), true},
		{"the data dir itself", data, true},
		{"a sibling directory", sibling, false},
		{"an absolute path elsewhere", "/etc/sehaty/key", false},
		// A directory whose name merely STARTS with the data dir's name is not inside
		// it. A naive strings.HasPrefix check gets this wrong.
		{"a path sharing the prefix", data + "-backup/key", false},
		{"no key file configured", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := assertKeyOutsideData(tc.key, data)
			if (err != nil) != tc.wantError {
				t.Fatalf("assertKeyOutsideData(%q, %q) = %v; want error: %v",
					tc.key, data, err, tc.wantError)
			}
		})
	}
}
