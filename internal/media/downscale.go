package media

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	_ "image/png"

	"golang.org/x/image/draw"
)

// DefaultMaxDimension is the longest edge a photo is reduced to before being handed to a
// model.
//
// A 4000x3000 phone photo costs roughly 16,000 tokens; at 1024 it is about 1,050. Telling
// a plate of nasi goreng from a plate of gado-gado does not need twelve megapixels, and
// that cost is paid on every single look.
const DefaultMaxDimension = 1024

// jpegQuality trades a little fidelity for a lot of bytes. 82 is visually clean for food
// photographs and roughly halves the output against 95.
const jpegQuality = 82

// Downscale reduces an image for transport and returns JPEG bytes.
//
// THE STORED FILE IS NEVER TOUCHED. This produces a smaller copy on the way out; the
// original stays on disk exactly as uploaded, so nothing is lost and a caller that wants
// the full image can still ask for it.
//
// Anything that is not a decodable image — a voice note, an unrecognised format — comes
// back unchanged rather than erroring. Refusing to hand over a file because it could not
// be shrunk would be a worse failure than sending it whole.
func Downscale(raw []byte, maxDim int) (out []byte, mime string, changed bool) {
	if maxDim <= 0 {
		maxDim = DefaultMaxDimension
	}
	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return raw, "", false
	}
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxDim && h <= maxDim {
		// Already small. Re-encoding would only lose fidelity for nothing.
		return raw, "", false
	}
	nw, nh := w, h
	if w >= h {
		nw, nh = maxDim, int(float64(h)*float64(maxDim)/float64(w))
	} else {
		nh, nw = maxDim, int(float64(w)*float64(maxDim)/float64(h))
	}
	if nw < 1 {
		nw = 1
	}
	if nh < 1 {
		nh = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	// CatmullRom: slower than nearest-neighbour, and the difference shows on food
	// photographs where texture is what distinguishes one dish from another.
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, dst, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return raw, "", false
	}
	if buf.Len() >= len(raw) {
		// Shrinking made it bigger, which happens with small or flat images.
		return raw, "", false
	}
	return buf.Bytes(), "image/jpeg", true
}

// Dimensions reports an image's size without decoding all of it.
func Dimensions(raw []byte) (w, h int, err error) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return 0, 0, fmt.Errorf("not a decodable image: %w", err)
	}
	return cfg.Width, cfg.Height, nil
}
