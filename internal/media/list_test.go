package media

import (
	"bytes"
	"context"
	"testing"
)

func TestListFindsStoredBlobs(t *testing.T) {
	l, _ := newLocal(t)
	ctx := context.Background()

	h1, err := l.Put(ctx, "raka", KindPhoto, bytes.NewReader([]byte("first meal")))
	if err != nil {
		t.Fatal(err)
	}
	h2, err := l.Put(ctx, "raka", KindPhoto, bytes.NewReader([]byte("second meal")))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := l.Put(ctx, "raka", KindVoice, bytes.NewReader([]byte("a note"))); err != nil {
		t.Fatal(err)
	}

	photos, err := l.List(ctx, "raka", KindPhoto)
	if err != nil {
		t.Fatal(err)
	}
	if len(photos) != 2 {
		t.Fatalf("%d photos, want 2", len(photos))
	}
	seen := map[string]bool{}
	for _, b := range photos {
		seen[b.Hash] = true
		if b.Bytes <= 0 {
			t.Errorf("%s reports %d bytes", b.Hash, b.Bytes)
		}
		if b.Kind != KindPhoto {
			t.Errorf("%s has kind %q in the photo listing", b.Hash, b.Kind)
		}
	}
	if !seen[h1] || !seen[h2] {
		t.Error("a stored photo is missing from the listing")
	}

	// Kinds must not bleed into each other.
	voice, err := l.List(ctx, "raka", KindVoice)
	if err != nil {
		t.Fatal(err)
	}
	if len(voice) != 1 {
		t.Fatalf("%d voice notes, want 1", len(voice))
	}
}

// Listing one profile must never reach into another's directory.
func TestListIsScopedToOneProfile(t *testing.T) {
	l, _ := newLocal(t)
	ctx := context.Background()
	if _, err := l.Put(ctx, "raka", KindPhoto, bytes.NewReader([]byte("mine"))); err != nil {
		t.Fatal(err)
	}
	got, err := l.List(ctx, "someone-else", KindPhoto)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("listing another profile returned %d blobs", len(got))
	}
}

func TestListOfAnEmptyProfileIsNotAnError(t *testing.T) {
	l, _ := newLocal(t)
	got, err := l.List(context.Background(), "raka", KindPhoto)
	if err != nil {
		t.Fatalf("listing an empty profile errored: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("%d blobs", len(got))
	}
}

func TestListRejectsAnInvalidProfileID(t *testing.T) {
	l, _ := newLocal(t)
	if _, err := l.List(context.Background(), "../etc", KindPhoto); err == nil {
		t.Fatal("accepted a traversal attempt as a profile id")
	}
}
