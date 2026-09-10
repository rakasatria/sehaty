package crypto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFromEnvPrefersKeyFile(t *testing.T) {
	fileKey, _ := GenerateKey()
	envKey, _ := GenerateKey()
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte(fileKey), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SEHATY_KEY_FILE", path)
	t.Setenv("SEHATY_KEY", envKey)

	c, err := FromEnv()
	if err != nil {
		t.Fatal(err)
	}
	// Prove it used the FILE key: seal here, open with a cipher built from fileKey.
	ct, _ := c.Seal([]byte("x"), []byte("a"))
	fromFile, _ := New(fileKey)
	if _, err := fromFile.Open(ct, []byte("a")); err != nil {
		t.Error("FromEnv used SEHATY_KEY when SEHATY_KEY_FILE was set")
	}
}

// echo writes a trailing newline. Without trimming, every key file fails to decode and
// the error blames the key rather than the newline.
func TestKeyFileTolerateseTrailingNewline(t *testing.T) {
	k, _ := GenerateKey()
	path := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(path, []byte(k+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SEHATY_KEY_FILE", path)
	t.Setenv("SEHATY_KEY", "")
	if _, err := FromEnv(); err != nil {
		t.Errorf("trailing newline broke key loading: %v", err)
	}
}

func TestFromEnvFallsBackToVariable(t *testing.T) {
	k, _ := GenerateKey()
	t.Setenv("SEHATY_KEY_FILE", "")
	t.Setenv("SEHATY_KEY", k)
	if _, err := FromEnv(); err != nil {
		t.Errorf("fallback to SEHATY_KEY failed: %v", err)
	}
}

func TestFromEnvMissingFileIsLoud(t *testing.T) {
	t.Setenv("SEHATY_KEY_FILE", "/nonexistent/key")
	if _, err := FromEnv(); err == nil {
		t.Error("a missing key file must fail, never fall back silently")
	}
}

func TestFromEnvEmptyFileIsLoud(t *testing.T) {
	path := filepath.Join(t.TempDir(), "key")
	os.WriteFile(path, []byte("   \n"), 0o600)
	t.Setenv("SEHATY_KEY_FILE", path)
	if _, err := FromEnv(); err == nil {
		t.Error("an empty key file must fail")
	}
}

func TestFromEnvNoKeyAtAll(t *testing.T) {
	t.Setenv("SEHATY_KEY_FILE", "")
	t.Setenv("SEHATY_KEY", "")
	if _, err := FromEnv(); err != ErrNoKey {
		t.Errorf("got %v, want ErrNoKey", err)
	}
}
