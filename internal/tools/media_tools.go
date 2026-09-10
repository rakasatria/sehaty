package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/rakasatria/sehaty/internal/crypto"
	"github.com/rakasatria/sehaty/internal/media"
)

// mimeFor maps a stored kind to a default MIME type when the caller did not say.
var defaultMIME = map[media.Kind]string{
	media.KindPhoto: "image/jpeg",
	media.KindVoice: "audio/ogg",
}

func parseKind(s string) (media.Kind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "photo", "":
		return media.KindPhoto, nil
	case "voice":
		return media.KindVoice, nil
	default:
		return "", fmt.Errorf("kind must be photo or voice, got %q", s)
	}
}

type AttachMediaArgs struct {
	Profile  string `json:"profile"`
	Kind     string `json:"kind,omitempty" jsonschema:"photo or voice; defaults to photo"`
	Data     []byte `json:"data" jsonschema:"the file itself, base64 encoded"`
	MIMEType string `json:"mime_type,omitempty" jsonschema:"e.g. image/jpeg or audio/ogg"`
}

type AttachMediaOut struct {
	Profile  string `json:"profile"`
	Kind     string `json:"kind"`
	Hash     string `json:"hash"`
	Bytes    int    `json:"bytes"`
	MIMEType string `json:"mime_type"`
	Note     string `json:"note"`
}

// AttachMedia stores a photo or voice note and returns its content hash.
//
// The hash IS the name — the same photo sent twice stores once. Pass it to log_food to
// attach the photo to a meal.
func AttachMedia(d Deps, a AttachMediaArgs) (AttachMediaOut, error) {
	if d.Media == nil {
		return AttachMediaOut{}, fmt.Errorf("media storage is not configured")
	}
	if _, err := requireProfile(d, a.Profile); err != nil {
		return AttachMediaOut{}, err
	}
	kind, err := parseKind(a.Kind)
	if err != nil {
		return AttachMediaOut{}, err
	}
	if len(a.Data) == 0 {
		return AttachMediaOut{}, fmt.Errorf("no data received; send the file base64 encoded in `data`")
	}
	hash, err := d.Media.Put(context.Background(), a.Profile, kind, bytes.NewReader(a.Data))
	if err != nil {
		return AttachMediaOut{}, err
	}
	mime := a.MIMEType
	if mime == "" {
		mime = defaultMIME[kind]
	}
	note := "Stored. Pass this hash to log_food as `photo` to attach it to a meal."
	if kind == media.KindVoice {
		note = "Stored. Voice notes are kept as recorded; nothing transcribes them yet."
	}
	return AttachMediaOut{Profile: a.Profile, Kind: string(kind), Hash: hash,
		Bytes: len(a.Data), MIMEType: mime, Note: note}, nil
}

type GetMediaArgs struct {
	Profile string `json:"profile"`
	Kind    string `json:"kind,omitempty" jsonschema:"photo or voice; defaults to photo"`
	Hash    string `json:"hash" jsonschema:"the content hash returned by attach_media or stored on a food entry"`
}

// GetMediaBytes fetches and decrypts one blob.
func GetMediaBytes(d Deps, a GetMediaArgs) ([]byte, media.Kind, error) {
	if d.Media == nil {
		return nil, "", fmt.Errorf("media storage is not configured")
	}
	if _, err := requireProfile(d, a.Profile); err != nil {
		return nil, "", err
	}
	kind, err := parseKind(a.Kind)
	if err != nil {
		return nil, "", err
	}
	rc, err := d.Media.Get(context.Background(), a.Profile, kind, a.Hash)
	if err != nil {
		return nil, "", err
	}
	defer rc.Close()
	raw, err := io.ReadAll(rc)
	if err != nil {
		return nil, "", err
	}
	return raw, kind, nil
}

type ListMediaArgs struct {
	Profile string `json:"profile"`
	Kind    string `json:"kind,omitempty" jsonschema:"photo or voice; defaults to photo"`
	Limit   int    `json:"limit,omitempty" jsonschema:"defaults to 20"`
}

type ListMediaOut struct {
	Profile string       `json:"profile"`
	Kind    string       `json:"kind"`
	Count   int          `json:"count"`
	Media   []media.Blob `json:"media"`
	Note    string       `json:"note,omitempty"`
}

func ListMedia(d Deps, a ListMediaArgs) (ListMediaOut, error) {
	if d.Media == nil {
		return ListMediaOut{}, fmt.Errorf("media storage is not configured")
	}
	if _, err := requireProfile(d, a.Profile); err != nil {
		return ListMediaOut{}, err
	}
	kind, err := parseKind(a.Kind)
	if err != nil {
		return ListMediaOut{}, err
	}
	blobs, err := d.Media.List(context.Background(), a.Profile, kind)
	if err != nil {
		return ListMediaOut{}, err
	}
	limit := a.Limit
	if limit <= 0 {
		limit = 20
	}
	if len(blobs) > limit {
		blobs = blobs[:limit]
	}
	out := ListMediaOut{Profile: a.Profile, Kind: string(kind), Count: len(blobs),
		Media: blobs}
	if out.Media == nil {
		out.Media = []media.Blob{}
	}
	if len(blobs) == 0 {
		out.Note = fmt.Sprintf("No %s stored for this profile yet.", kind)
	}
	return out, nil
}

// cryptoMedia builds the media cipher from the environment. Exposed for tests; the server
// builds it in main and passes the store in.
func cryptoMedia() (*crypto.Cipher, error) {
	key, err := crypto.ReadKeyEnv()
	if err != nil {
		// Tests run without a configured key; a fixed one is fine because the store they
		// build lives in a temp directory that is deleted when the test ends.
		return crypto.NewMedia("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	}
	return crypto.NewMedia(key)
}
