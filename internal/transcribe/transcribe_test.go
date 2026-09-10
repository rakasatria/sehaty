package transcribe

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func stub(t *testing.T, handler http.HandlerFunc) (*Client, func()) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c := New("test-key", "")
	c.HTTP = srv.Client()
	// Point the client at the stub by rewriting the request URL in a transport.
	c.HTTP.Transport = rewrite{srv.URL, srv.Client().Transport}
	return c, srv.Close
}

type rewrite struct {
	base string
	next http.RoundTripper
}

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	u := *req.URL
	target := strings.TrimPrefix(r.base, "http://")
	u.Host, u.Scheme = target, "http"
	req.URL = &u
	n := r.next
	if n == nil {
		n = http.DefaultTransport
	}
	return n.RoundTrip(req)
}

func reply(text string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{map[string]any{"message": map[string]any{"content": text}}},
		})
	}
}

func TestTranscribeReturnsTheSpokenText(t *testing.T) {
	c, done := stub(t, reply("  tadi siang saya makan nasi goreng  "))
	defer done()
	got, err := c.Transcribe(context.Background(), []byte("fake ogg"), "audio/ogg")
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "tadi siang saya makan nasi goreng" {
		t.Errorf("text = %q", got.Text)
	}
	if got.Language != "id" {
		t.Errorf("language = %q, want id", got.Language)
	}
	if got.Model == "" {
		t.Error("no model recorded — a transcript without provenance cannot be judged")
	}
}

// Silence is a valid answer, not a failure. Erroring would make the caller retry a
// recording that will never contain words.
func TestSilenceIsNotAnError(t *testing.T) {
	c, done := stub(t, reply("[no speech]"))
	defer done()
	got, err := c.Transcribe(context.Background(), []byte("x"), "audio/ogg")
	if err != nil {
		t.Fatalf("silence returned an error: %v", err)
	}
	if got.Text != "" {
		t.Errorf("text = %q, want empty", got.Text)
	}
}

// The standing instruction is that this data must never be used for training.
func TestRequestRefusesDataCollection(t *testing.T) {
	var seen map[string]any
	c, done := stub(t, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &seen)
		reply("ok")(w, r)
	})
	defer done()
	if _, err := c.Transcribe(context.Background(), []byte("x"), "audio/ogg"); err != nil {
		t.Fatal(err)
	}
	p, _ := seen["provider"].(map[string]any)
	if p == nil || p["data_collection"] != "deny" {
		t.Fatalf("provider block = %v; data_collection must be deny", seen["provider"])
	}
	if seen["temperature"] != float64(0) {
		t.Errorf("temperature = %v; a transcript must not be creative", seen["temperature"])
	}
}

func TestAudioFormatIsDerivedFromTheMIMEType(t *testing.T) {
	for mime, want := range map[string]string{
		"audio/ogg": "ogg", "audio/opus": "ogg", "audio/mpeg": "mp3",
		"audio/wav": "wav", "audio/mp4": "m4a", "audio/flac": "flac",
		"application/octet-stream": "ogg",
	} {
		if got := formatFor(mime); got != want {
			t.Errorf("formatFor(%q) = %q, want %q", mime, got, want)
		}
	}
}

func TestDisabledWithoutAKey(t *testing.T) {
	c := New("", "")
	if c.Enabled() {
		t.Fatal("reported enabled with no key")
	}
	if _, err := c.Transcribe(context.Background(), []byte("x"), "audio/ogg"); err == nil {
		t.Fatal("transcribed without a key")
	}
}

func TestUpstreamFailureIsReportedNotSwallowed(t *testing.T) {
	c, done := stub(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	})
	defer done()
	_, err := c.Transcribe(context.Background(), []byte("x"), "audio/ogg")
	if err == nil {
		t.Fatal("a 429 was swallowed")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error hides the status: %v", err)
	}
}

func TestEmptyAudioIsRejectedBeforeSending(t *testing.T) {
	c := New("k", "")
	if _, err := c.Transcribe(context.Background(), nil, "audio/ogg"); err == nil {
		t.Fatal("sent an empty request upstream")
	}
}
