package media

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

func bigImage(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			// Varied content, so JPEG cannot compress it to nothing and the size
			// comparison stays meaningful.
			img.Set(x, y, color.RGBA{uint8(x % 251), uint8(y % 241), uint8((x * y) % 239), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestDownscaleShrinksALargePhoto(t *testing.T) {
	raw := bigImage(t, 3000, 2000)
	out, mime, changed := Downscale(raw, DefaultMaxDimension)
	if !changed {
		t.Fatal("a 3000x2000 photo was not reduced")
	}
	if mime != "image/jpeg" {
		t.Errorf("mime = %q", mime)
	}
	if len(out) >= len(raw) {
		t.Errorf("output %d bytes is not smaller than input %d", len(out), len(raw))
	}
	w, h, err := Dimensions(out)
	if err != nil {
		t.Fatal(err)
	}
	if w != DefaultMaxDimension {
		t.Errorf("width = %d, want %d", w, DefaultMaxDimension)
	}
	// Aspect ratio must survive: 3000x2000 -> 1024x683.
	if h != 682 && h != 683 {
		t.Errorf("height = %d; aspect ratio was not preserved", h)
	}
}

// Re-encoding a small image would lose fidelity for no gain.
func TestDownscaleLeavesSmallImagesAlone(t *testing.T) {
	raw := bigImage(t, 400, 300)
	out, _, changed := Downscale(raw, DefaultMaxDimension)
	if changed {
		t.Error("a 400x300 image was needlessly re-encoded")
	}
	if !bytes.Equal(out, raw) {
		t.Error("a small image came back altered")
	}
}

// Refusing to hand over a file because it could not be shrunk is worse than sending it whole.
func TestDownscalePassesNonImagesThrough(t *testing.T) {
	raw := []byte("this is a voice note, not an image")
	out, _, changed := Downscale(raw, DefaultMaxDimension)
	if changed {
		t.Error("non-image data was reported as downscaled")
	}
	if !bytes.Equal(out, raw) {
		t.Error("non-image data was altered")
	}
}

func TestDownscaleRespectsAPortraitOrientation(t *testing.T) {
	raw := bigImage(t, 2000, 3000)
	out, _, changed := Downscale(raw, 512)
	if !changed {
		t.Fatal("portrait photo not reduced")
	}
	w, h, err := Dimensions(out)
	if err != nil {
		t.Fatal(err)
	}
	if h != 512 {
		t.Errorf("height = %d, want 512 — the LONGEST edge should be bounded", h)
	}
	if w >= h {
		t.Errorf("%dx%d is no longer portrait", w, h)
	}
}
