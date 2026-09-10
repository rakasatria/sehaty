package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// The model must have no argument through which to name another person. If `profile` ever
// appears in a schema, a hallucinated id becomes a cross-profile write.
func TestNoToolExposesAProfileArgument(t *testing.T) {
	full := NewRegistry(Registry{}.deps) // deps zero value is fine: schemas are static
	for _, s := range full.Schemas() {
		b, _ := json.Marshal(s.Function.Parameters)
		if strings.Contains(strings.ToLower(string(b)), "profile") {
			t.Errorf("tool %q exposes a profile argument: %s", s.Function.Name, b)
		}
	}
}

// The dangerous tools must be absent, not merely discouraged by the prompt.
func TestDangerousToolsAreNotRegistered(t *testing.T) {
	names := map[string]bool{}
	for _, n := range NewRegistry(Registry{}.deps).Names() {
		names[n] = true
	}
	for _, banned := range []string{"register", "put_document", "name_foods",
		"attach_media", "get_media", "delete_profile"} {
		if names[banned] {
			t.Errorf("%q is reachable from a conversation", banned)
		}
	}
	for _, want := range []string{"find_foods", "log_food", "progress", "plan_session"} {
		if !names[want] {
			t.Errorf("%q is missing", want)
		}
	}
}

// A refusal is information the model needs, not a failure. Returning it as a result lets
// the model ask a better question; returning an error makes it apologise instead.
func TestRefusalsComeBackAsResults(t *testing.T) {
	r := NewRegistry(Registry{}.deps)
	out := r.Invoke(context.Background(), "someone", "no_such_tool", `{}`)
	var parsed map[string]string
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invoke returned non-JSON: %q", out)
	}
	if parsed["refused"] == "" {
		t.Errorf("a missing tool did not report a refusal: %v", parsed)
	}
}

// A model calls tools with whatever it likes. A panic in one must come back as a refusal,
// not take down the conversation.
func TestAPanickingToolBecomesARefusal(t *testing.T) {
	r := NewRegistry(Registry{}.deps) // zero deps: any DB call will panic
	out := r.Invoke(context.Background(), "nobody", "get_profile", "")
	var parsed map[string]string
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("invoke returned non-JSON after a panic: %q", out)
	}
	if parsed["refused"] == "" {
		t.Errorf("a panicking tool did not report a refusal: %v", parsed)
	}
}

func TestMalformedArgumentsDoNotCrash(t *testing.T) {
	r := NewRegistry(Registry{}.deps)
	for _, args := range []string{"", "{", "null", `{"grams":"not a number"}`, "[]"} {
		out := r.Invoke(context.Background(), "nobody", "log_food", args)
		if !json.Valid([]byte(out)) {
			t.Errorf("args %q produced invalid JSON: %q", args, out)
		}
	}
}

// Health facts live in the database, so history exists only for conversational context.
// Unbounded history costs money to re-send every turn and buys nothing.
func TestHistoryIsTrimmed(t *testing.T) {
	var msgs []Message
	for i := 0; i < 80; i++ {
		msgs = append(msgs, Message{Role: "user", Content: "x"})
		msgs = append(msgs, Message{Role: "assistant", Content: "y"})
	}
	got := trim(msgs)
	if len(got) > 20 {
		t.Errorf("kept %d messages", len(got))
	}
	// The tail, not the head: the recent turns are the ones with context in them.
	if got[len(got)-1].Content != "y" {
		t.Error("trim kept the wrong end of the conversation")
	}
}

// Tool traffic must never enter the history. The results were written to the database by
// the tools themselves; re-sending them every turn pays for the same facts twice.
func TestHistoryHoldsOnlyWhatWasSaid(t *testing.T) {
	_, history, err := (&Client{}).Respond(context.Background(), "p", "", nil)
	if err == nil {
		t.Fatal("an unconfigured client answered")
	}
	for _, m := range history {
		if m.Role != "user" && m.Role != "assistant" {
			t.Errorf("history carries a %q message", m.Role)
		}
	}
}

// The prompt has to carry the rules the tools enforce, or the model fights them.
func TestSystemPromptStatesTheRulesThatMatter(t *testing.T) {
	for _, want := range []string{"Never invent a number", "find_foods", "ask",
		"not a doctor", "Composite dishes"} {
		if !strings.Contains(SystemPrompt, want) {
			t.Errorf("system prompt does not mention %q", want)
		}
	}
	for _, banned := range []string{"emoji", "!"} {
		if strings.Count(SystemPrompt, banned) > 2 {
			t.Errorf("system prompt is inconsistent about %q", banned)
		}
	}
}

func TestDisabledWithoutAKey(t *testing.T) {
	if New("", "", NewRegistry(Registry{}.deps)).Enabled() {
		t.Fatal("reported enabled with no key")
	}
	if !New("k", "", NewRegistry(Registry{}.deps)).Enabled() {
		t.Fatal("reported disabled with a key and a registry")
	}
	if New("k", "", nil).Enabled() {
		t.Fatal("reported enabled with no tools")
	}
}
