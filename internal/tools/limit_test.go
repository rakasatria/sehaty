package tools

import (
	"testing"

	"github.com/rakasatria/sehaty/internal/media"
)

// The transport ceiling must exceed what the media store accepts, after base64 expansion.
// Otherwise an oversized upload is refused by the HTTP layer with an opaque 413 instead of
// by the store with a message naming the limit — and worse, a legitimate photo under the
// store's limit gets rejected outright, which is what happened with the SDK's 4 MiB
// default against a 3.5 MB photo.
func TestTransportLimitExceedsTheMediaCapAfterBase64(t *testing.T) {
	needed := int64(media.DefaultMaxBytes) * 4 / 3
	if MaxRequestBody <= needed {
		t.Fatalf("MaxRequestBody=%d does not cover a %d-byte blob base64 encoded (%d)",
			MaxRequestBody, media.DefaultMaxBytes, needed)
	}
	// A realistic phone photo must fit with room to spare.
	const phonePhoto = 5 << 20
	if MaxRequestBody < phonePhoto*4/3 {
		t.Fatalf("MaxRequestBody=%d cannot carry a 5MB photo", MaxRequestBody)
	}
}
