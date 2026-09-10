package crypto

import (
	"fmt"
	"os"
	"strings"
)

// ReadKeyEnv returns the base64 MASTER key from the environment, preferring a FILE over
// a variable.
//
//	SEHATY_KEY_FILE   path to a file containing the base64 key   (preferred)
//	SEHATY_KEY        the base64 key itself                      (fallback)
//
// The file is preferred because an environment variable is not a secret: it appears in
// `docker inspect`, in `ps` on the host, in shell history, and in a good deal of logging.
// A mounted file — Docker secret, tmpfs, or a host bind-mount — can live somewhere the
// encrypted volume's backup never reaches, which is the whole point of separating them.
func ReadKeyEnv() (string, error) {
	if path := os.Getenv("SEHATY_KEY_FILE"); path != "" {
		raw, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read SEHATY_KEY_FILE %s: %w", path, err)
		}
		// Trailing newline is the single most common way this fails: `echo key > file`
		// appends one, base64 decoding then rejects it, and the error blames the key
		// rather than the newline. Trim, and say so if nothing is left.
		key := strings.TrimSpace(string(raw))
		if key == "" {
			return "", fmt.Errorf("SEHATY_KEY_FILE %s is empty", path)
		}
		return key, nil
	}
	key := os.Getenv("SEHATY_KEY")
	if key == "" {
		return "", ErrNoKey
	}
	return key, nil
}

// FromEnv builds the document Cipher from the environment.
func FromEnv() (*Cipher, error) {
	key, err := ReadKeyEnv()
	if err != nil {
		return nil, err
	}
	return New(key)
}

// DBKeyFromEnv returns the Adiantum subkey for storage.Open.
func DBKeyFromEnv() (string, error) {
	key, err := ReadKeyEnv()
	if err != nil {
		return "", err
	}
	return DBKeyHex(key)
}
