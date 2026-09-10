package tools

import (
	"encoding/base64"
	"strings"
	"testing"

	"github.com/rakasatria/sehaty/internal/media"
)

func mediaDeps(t *testing.T) (Deps, string) {
	t.Helper()
	d := testDeps(t)
	c, err := cryptoMedia()
	if err != nil {
		t.Fatal(err)
	}
	store, err := media.NewLocal(t.TempDir(), c, 0)
	if err != nil {
		t.Fatal(err)
	}
	d.Media = store
	p, err := Register(d, "mcp", "acct-1", "Raka", pass, pass)
	if err != nil {
		t.Fatal(err)
	}
	return d, p.ID
}

func TestAttachMediaStoresAndReturnsAHash(t *testing.T) {
	d, id := mediaDeps(t)
	out, err := AttachMedia(d, AttachMediaArgs{Profile: id, Kind: "photo",
		Data: b64("pretend this is a jpeg")})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Hash) != 64 {
		t.Fatalf("hash %q is not a sha256", out.Hash)
	}
	if out.MIMEType == "" {
		t.Error("no mime type reported")
	}

	back, kind, err := GetMediaBytes(d, GetMediaArgs{Profile: id, Kind: "photo", Hash: out.Hash})
	if err != nil {
		t.Fatal(err)
	}
	if string(back) != "pretend this is a jpeg" {
		t.Errorf("round trip returned %q", back)
	}
	if kind != media.KindPhoto {
		t.Errorf("kind = %q", kind)
	}
}

func TestAttachMediaRejectsEmptyDataAndBadKind(t *testing.T) {
	d, id := mediaDeps(t)
	if _, err := AttachMedia(d, AttachMediaArgs{Profile: id, Kind: "photo"}); err == nil {
		t.Error("accepted an empty upload")
	}
	if _, err := AttachMedia(d, AttachMediaArgs{Profile: id, Kind: "video",
		Data: b64("x")}); err == nil {
		t.Error("accepted an unsupported kind")
	}
}

// A dangling hash on a meal would look like evidence and resolve to nothing.
func TestLogFoodRejectsAPhotoThatWasNeverStored(t *testing.T) {
	d, id := mediaDeps(t)
	_, err := LogFood(d, LogFoodArgs{Profile: id, Food: "AR001", Grams: 100,
		Photo: strings.Repeat("a", 64)})
	if err == nil {
		t.Fatal("logged a meal against a photo that does not exist")
	}
	if !strings.Contains(err.Error(), "attach_media") {
		t.Errorf("error does not say how to fix it: %v", err)
	}
}

func TestLogFoodAttachesAStoredPhoto(t *testing.T) {
	d, id := mediaDeps(t)
	att, err := AttachMedia(d, AttachMediaArgs{Profile: id, Data: b64("a meal photo")})
	if err != nil {
		t.Fatal(err)
	}
	out, err := LogFood(d, LogFoodArgs{Profile: id, Food: "AR001", Grams: 150,
		Meal: "makan siang", Photo: att.Hash})
	if err != nil {
		t.Fatal(err)
	}
	if out.Photo != att.Hash {
		t.Errorf("photo = %q, want %q", out.Photo, att.Hash)
	}
	entries, err := d.DB.Foods(id, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].PhotoHash != att.Hash {
		t.Errorf("photo hash did not persist onto the food entry: %+v", entries)
	}
}

// One person's media must never appear in another's listing.
func TestListMediaIsScopedToTheProfile(t *testing.T) {
	d, id := mediaDeps(t)
	other, err := Register(d, "mcp", "acct-2", "Dina", pass, pass)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := AttachMedia(d, AttachMediaArgs{Profile: id, Data: b64("mine")}); err != nil {
		t.Fatal(err)
	}
	mine, err := ListMedia(d, ListMediaArgs{Profile: id})
	if err != nil {
		t.Fatal(err)
	}
	if mine.Count != 1 {
		t.Fatalf("own listing has %d", mine.Count)
	}
	theirs, err := ListMedia(d, ListMediaArgs{Profile: other.ID})
	if err != nil {
		t.Fatal(err)
	}
	if theirs.Count != 0 {
		t.Fatalf("another profile's listing shows %d of my blobs", theirs.Count)
	}
}

// The AAD binds each blob to its profile, so a hash alone is not enough to read it.
func TestGetMediaRefusesAnotherProfilesHash(t *testing.T) {
	d, id := mediaDeps(t)
	other, _ := Register(d, "mcp", "acct-2", "Dina", pass, pass)
	att, err := AttachMedia(d, AttachMediaArgs{Profile: id, Data: b64("private photo")})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := GetMediaBytes(d, GetMediaArgs{Profile: other.ID, Hash: att.Hash}); err == nil {
		t.Fatal("another profile read the blob using only its hash")
	}
}

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// The schema generator maps []byte to an array of integers, which rejects the base64 every
// client actually sends. This pins that `data` stays a plain string.
func TestAttachMediaRejectsNonBase64(t *testing.T) {
	d, id := mediaDeps(t)
	if _, err := AttachMedia(d, AttachMediaArgs{Profile: id, Data: "not base64!!"}); err == nil {
		t.Fatal("accepted data that is not base64")
	}
}

// The stored bytes are the truth about what a file is. Defaulting the type from the kind
// reported image/jpeg for a PNG.
func TestSniffMIMEUsesTheBytesNotTheKind(t *testing.T) {
	png := []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 32))
	if got := sniffMIME(png, media.KindPhoto); got != "image/png" {
		t.Errorf("PNG sniffed as %q, want image/png", got)
	}
	jpeg := append([]byte{0xff, 0xd8, 0xff}, make([]byte, 32)...)
	if got := sniffMIME(jpeg, media.KindPhoto); got != "image/jpeg" {
		t.Errorf("JPEG sniffed as %q, want image/jpeg", got)
	}
	// Unrecognised bytes fall back to what the kind implies rather than guessing.
	if got := sniffMIME([]byte{0x01, 0x02, 0x03}, media.KindVoice); got != "audio/ogg" {
		t.Errorf("unknown bytes sniffed as %q, want the voice default audio/ogg", got)
	}
	if got := sniffMIME(nil, media.KindPhoto); got != "image/jpeg" {
		t.Errorf("empty input sniffed as %q", got)
	}
}
