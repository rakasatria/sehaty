package telegram

import (
	"testing"

	"github.com/rakasatria/sehaty/internal/agent"
)

// Two people on one server must never inherit each other's thread.
func TestConversationsAreIsolatedPerChatAndCopied(t *testing.T) {
	c := newConversations()
	c.set(1, []agent.Message{{Role: "user", Content: "mine"}})
	c.set(2, []agent.Message{{Role: "user", Content: "theirs"}})

	if got := c.get(1); len(got) != 1 || got[0].Content != "mine" {
		t.Fatalf("chat 1 got %v", got)
	}
	if got := c.get(2); got[0].Content != "theirs" {
		t.Fatal("chat 2 sees chat 1's thread")
	}

	// A caller mutating what get returned must not reach into the store.
	got := c.get(1)
	got[0].Content = "tampered"
	if c.get(1)[0].Content != "mine" {
		t.Error("get handed out the live slice")
	}

	c.forget(1)
	if len(c.get(1)) != 0 {
		t.Error("forget left the thread behind")
	}
	if len(c.get(2)) != 1 {
		t.Error("forget cleared somebody else's thread")
	}
}
