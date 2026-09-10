package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/rakasatria/sehaty/internal/agent"
	"github.com/rakasatria/sehaty/internal/storage"
)

// conversations holds the recent back-and-forth per chat, in memory only.
//
// Deliberately not persisted. Everything that matters — the meal, the weight, the sets —
// is written to the database by a tool at the moment it is agreed. The chat log is just
// the scaffolding around that, and keeping it on disk would mean a second, unencrypted
// copy of someone's health talk sitting beside the encrypted one. Losing it on restart
// costs a little context and nothing else.
type conversations struct {
	mu sync.Mutex
	m  map[int64][]agent.Message
}

func newConversations() *conversations {
	return &conversations{m: make(map[int64][]agent.Message)}
}

func (c *conversations) get(chat int64) []agent.Message {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]agent.Message(nil), c.m[chat]...)
}

func (c *conversations) set(chat int64, msgs []agent.Message) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[chat] = msgs
}

func (c *conversations) forget(chat int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.m, chat)
}

// converse sends one message through the agent and replies with what it says.
//
// note carries anything the model should know about how this message arrived — that it
// was transcribed from speech, say — and is not shown to the user.
func (b *Bot) converse(ctx context.Context, chat int64, p storage.Profile, text, note string) {
	if b.Agent == nil || !b.Agent.Enabled() {
		b.reply(ctx, chat, cannotConverse(text))
		return
	}
	b.typing(ctx, chat)

	history := b.chats.get(chat)
	if strings.TrimSpace(text) != "" {
		history = append(history, agent.Message{Role: "user", Content: text})
	} else if len(history) == 0 {
		// Opening the conversation ourselves, with nothing said yet. The
		// instruction is in the note; this turn exists because a model needs
		// something to answer, and an empty user message is rejected outright by
		// some providers. It is never shown to anyone.
		history = append(history, agent.Message{Role: "user", Content: "(mulai)"})
	}
	brief := agent.Brief(b.Deps, p)
	if note != "" {
		brief += "\n\n" + note
	}

	// The sink the offer_choices tool writes into. The model decides whether this
	// question is worth showing as buttons; Go decides what the buttons are.
	offer := &agent.Offer{}
	answer, history, err := b.Agent.Respond(agent.WithOffer(ctx, offer), p.ID, brief, history)
	if err != nil {
		// Whatever went wrong upstream is not this person's problem to parse.
		b.reply(ctx, chat, "Something went wrong on my side and I could not answer that. "+
			"Nothing was saved. Try again in a moment.")
		return
	}
	if strings.TrimSpace(answer) == "" {
		answer = "I am not sure how to answer that. Try telling me what you ate, " +
			"what you lifted, or ask what your week looked like."
	}
	b.chats.set(chat, history)

	if q := offer.Question(); q != "" && b.askWithChoices(ctx, chat, answer, q) {
		return
	}
	b.reply(ctx, chat, answer)
}

// cannotConverse is what a free-text message gets when no model is configured. The bot
// still works — it just cannot chat — and saying which is kinder than a shrug.
func cannotConverse(text string) string {
	if len(text) > 60 {
		text = text[:60] + "…"
	}
	return fmt.Sprintf(
		"I can't hold a conversation on this server — no language model is configured, "+
			"so “%s” goes past me.\n\nWhat still works: voice notes, meal photos, "+
			"berat 70.5 for your weight, /me for your month and /dash for the dashboard.",
		text)
}
