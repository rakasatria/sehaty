// Package agent lets someone talk to Sehaty in ordinary language.
//
// It is a narrow loop, not a general assistant: a model is given a fixed set of Sehaty's
// own tools, and those tools keep their refusals. The model can ask to log a meal; it
// cannot invent what the meal contained, because log_food resolves the food against TKPI
// and refuses an ambiguous name rather than picking one.
//
// The loop is goai's. It was chosen over the alternatives on one hard requirement and one
// soft one. Hard: it can put provider.data_collection = "deny" at the top level of the
// request body, which decides whether this data may be used for training at all. Soft: it
// has two dependencies and no JIT, so the service keeps MemoryDenyWriteExecute — the
// previous SDK compiled assembly at startup and could not run under it.
//
// THE PROFILE IS NEVER THE MODEL'S TO CHOOSE. It is injected server-side from the verified
// channel identity and appears in no tool schema. See profileKey in goaitools.go.
package agent

import (
	"context"
	"fmt"
	"strings"

	"github.com/zendev-sh/goai"
	"github.com/zendev-sh/goai/provider"
	"github.com/zendev-sh/goai/provider/openrouter"
)

// DefaultModel needs four things at once: tool-calling, image input, Bahasa good enough
// to hold a mixed-language conversation, and — the constraint that actually decides it —
// at least one provider that honours provider.data_collection = "deny".
//
// That last one is not a preference. deepseek/deepseek-v4.1-flash was chosen first on
// price and capability and 404s on every request: its only endpoint trains on what it is
// sent, so the deny filter leaves nothing to route to. The model list is no help here —
// it advertises tools and vision and says nothing about data policy. The only way to know
// is to send a request with deny set and see whether anything answers.
//
// glm-5.3-flash answers, calls tools accurately from Indonesian, and costs a third of
// what the Gemini tier does.
const DefaultModel = "z-ai/glm-5.3-flash"

// maxSteps bounds the tool loop. A model that keeps calling tools without answering is a
// model burning money in a circle; six is generous for "look up a food, then log it".
const maxSteps = 6

// Message is one turn of the conversation as Sehaty keeps it.
//
// Only what was said, never tool traffic: the results were written to the database by the
// tools themselves, so re-sending them every turn would pay for the same facts twice.
type Message struct {
	Role    string
	Content string
}

type Client struct {
	Key     string
	Model   string
	BaseURL string
	Tools   *Registry
}

func New(key, model string, reg *Registry) *Client {
	if model == "" {
		model = DefaultModel
	}
	return &Client{Key: key, Model: model, Tools: reg}
}

func (c *Client) Enabled() bool { return c != nil && c.Key != "" && c.Tools != nil }

func (c *Client) model() provider.LanguageModel {
	opts := []openrouter.Option{openrouter.WithAPIKey(c.Key)}
	if c.BaseURL != "" {
		opts = append(opts, openrouter.WithBaseURL(c.BaseURL))
	}
	return openrouter.Chat(c.Model, opts...)
}

// Respond runs one exchange and returns what to say back.
//
// brief describes who this person is and what is still unknown about them; it is
// regenerated from the database on every message so the model never asks for something
// that was answered five minutes ago.
func (c *Client) Respond(ctx context.Context, profileID, brief string, history []Message) (string, []Message, error) {
	if !c.Enabled() {
		return "", history, fmt.Errorf("conversation is not configured on this server")
	}
	tools, err := c.Tools.Tools()
	if err != nil {
		return "", history, err
	}

	msgs := make([]provider.Message, 0, len(history)+2)
	if b := strings.TrimSpace(brief); b != "" {
		msgs = append(msgs, goai.SystemMessage(b))
	}
	for _, m := range history {
		switch m.Role {
		case "assistant":
			msgs = append(msgs, goai.AssistantMessage(m.Content))
		default:
			msgs = append(msgs, goai.UserMessage(m.Content))
		}
	}

	res, err := goai.GenerateText(withProfile(ctx, profileID), c.model(),
		goai.WithSystem(SystemPrompt),
		goai.WithMessages(msgs...),
		goai.WithTools(tools...),
		goai.WithMaxSteps(maxSteps),
		// The standing instruction, and the one that decides which models are usable at
		// all: this data must never train anything. OpenRouter takes it as a top-level
		// routing constraint on the request body.
		goai.WithProviderOptions(map[string]any{
			"provider": map[string]any{"data_collection": "deny"},
		}),
	)
	if err != nil {
		return "", history, err
	}

	answer := strings.TrimSpace(res.Text)
	return answer, trim(append(history, Message{Role: "assistant", Content: answer})), nil
}

// trim keeps recent context only. Health facts live in the database, not in the chat log,
// so an unbounded history costs money to re-send and buys nothing.
func trim(msgs []Message) []Message {
	const keep = 20
	if len(msgs) <= keep {
		return msgs
	}
	return msgs[len(msgs)-keep:]
}
