package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// The standing rule is that this data never trains anything, and it is enforced by a
// single field in the outgoing body. That field now travels through eino's config rather
// than a hand-built request, so this asserts it against the actual wire — not against the
// struct we hoped it came from.
//
// Getting this wrong does not fail loudly. OpenRouter would simply route to a provider
// that trains on the request, and everything would look like it was working.
func TestEveryRequestForbidsTrainingOnThisData(t *testing.T) {
	var mu sync.Mutex
	var bodies []map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","object":"chat.completion","choices":[{"index":0,` +
			`"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	c := New("test-key", "some/model", NewRegistry(Registry{}.deps))
	c.BaseURL = srv.URL

	if _, _, err := c.Respond(context.Background(), "p1", "", []Message{
		{Role: "user", Content: "halo"}}); err != nil {
		t.Fatalf("stubbed exchange failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) == 0 {
		t.Fatal("no request reached the server")
	}
	for i, b := range bodies {
		prov, ok := b["provider"].(map[string]any)
		if !ok {
			t.Fatalf("request %d carries no provider block: %v", i, b)
		}
		if prov["data_collection"] != "deny" {
			t.Errorf("request %d does not deny data collection: %v", i, prov)
		}
	}
}

// An upstream refusal must surface as an error rather than an empty-but-successful reply.
// OpenRouter returns 404 both for "no such model" and for "no provider matches your data
// policy", and the second is the one that will happen here.
func TestUpstreamFailureIsReportedNotSwallowed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"message":"No endpoints found matching your data policy"}}`))
	}))
	defer srv.Close()

	c := New("test-key", "some/model", NewRegistry(Registry{}.deps))
	c.BaseURL = srv.URL

	answer, _, err := c.Respond(context.Background(), "p1", "", []Message{
		{Role: "user", Content: "halo"}})
	if err == nil {
		t.Fatalf("a 404 was reported as success, answer=%q", answer)
	}
}

// A conversation with no identity bound must not reach a tool at all.
func TestToolsRefuseWhenNoProfileIsBound(t *testing.T) {
	ts, err := NewRegistry(Registry{}.deps).Tools()
	if err != nil {
		t.Fatal(err)
	}
	if len(ts) == 0 {
		t.Fatal("no tools were built")
	}
	out, err := ts[0].Execute(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("an unbound call errored instead of refusing: %v", err)
	}
	if !contains(out, "no profile is bound") {
		t.Errorf("an unbound tool call was not refused: %s", out)
	}
}
