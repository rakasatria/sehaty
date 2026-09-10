package telegram

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/rakasatria/sehaty/internal/agent"
	"github.com/rakasatria/sehaty/internal/storage"
	"github.com/rakasatria/sehaty/internal/tools"
)

// cbPrefix marks the callback data this bot owns.
const cbPrefix = "c:"

// skipIndex is the button that declines a question.
const skipIndex = -1

// Callback data is STATELESS, and that is the point.
//
// It encodes "c:<question index>:<choice index>" against two fixed tables — the order of
// agent.ChoiceQuestions and the order of that question's Choices. Nothing is remembered in
// memory between drawing a button and someone tapping it, so a button tapped tomorrow, or
// after a restart, or after a redeploy, still works.
//
// The alternative — registering a handler per keyboard — means every button ever drawn
// either leaks a handler or stops working when the process restarts. This has neither
// problem, and it stays inside Telegram's 64-byte limit with room to spare.
func callbackData(question string, choiceIdx int) string {
	qi := -1
	for i, q := range agent.ChoiceQuestions {
		if q == question {
			qi = i
			break
		}
	}
	return fmt.Sprintf("%s%d:%d", cbPrefix, qi, choiceIdx)
}

func parseCallback(data string) (question string, choiceIdx int, ok bool) {
	parts := strings.Split(strings.TrimPrefix(data, cbPrefix), ":")
	if len(parts) != 2 {
		return "", 0, false
	}
	qi, err1 := strconv.Atoi(parts[0])
	ci, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil || qi < 0 || qi >= len(agent.ChoiceQuestions) {
		return "", 0, false
	}
	question = agent.ChoiceQuestions[qi]
	if ci == skipIndex {
		return question, skipIndex, true
	}
	// Bounds are checked against the catalogue, not trusted from the wire. A tap is
	// whatever the phone chose to send.
	cs, exists := choices[question]
	if !exists || ci < 0 || ci >= len(cs.Choices) {
		return "", 0, false
	}
	return question, ci, true
}

// askWithChoices draws the buttons for one assessment question under the model's reply.
//
// The model asked for this question by name and knows nothing else about it: every label,
// every value written, and what happens on a press come from the catalogue in choices.go.
// A press therefore cannot carry a value — Telegram returns an index into a table this
// process built — so a forged callback can at worst pick a different one of these buttons.
func (b *Bot) askWithChoices(ctx context.Context, chat int64, text, question string) bool {
	cs, ok := choices[question]
	if !ok {
		return false
	}
	perRow := cs.PerRow
	if perRow < 1 {
		perRow = 2
	}

	buttons := make([]telego.InlineKeyboardButton, 0, len(cs.Choices))
	for i, c := range cs.Choices {
		buttons = append(buttons, telego.InlineKeyboardButton{
			Text: c.Label, CallbackData: callbackData(question, i),
		})
	}
	rows := tu.InlineKeyboardCols(perRow, buttons...)
	// Every question is declinable, and the decline is recorded so it is never asked
	// again. A button is the least effortful way to say no, which is the point.
	rows = append(rows, tu.InlineKeyboardRow(telego.InlineKeyboardButton{
		Text: "Skip", CallbackData: callbackData(question, skipIndex),
	}))

	if _, err := b.tg.SendMessage(ctx, tu.Message(tu.ID(chat), clamp(text)).
		WithReplyMarkup(tu.InlineKeyboard(rows...))); err != nil {
		log.Printf("telegram: sending buttons for %s failed: %v", question, err)
		return false
	}
	return true
}

// onCallback handles a button press.
func (b *Bot) onCallback(ctx *th.Context, q telego.CallbackQuery) error {
	// Answered FIRST, before any slow work. Telegram spins the button until this
	// arrives, and a model round-trip takes seconds.
	if err := b.tg.AnswerCallbackQuery(ctx, tu.CallbackQuery(q.ID)); err != nil {
		log.Printf("telegram: answering a callback failed: %v", err)
	}

	p, registered := b.whoIs(&q.From)
	if !registered {
		return nil
	}

	chat := int64(0)
	if q.Message != nil {
		chat = q.Message.GetChat().ID
		// Retire the buttons so the question cannot be answered twice.
		if _, err := b.tg.EditMessageReplyMarkup(ctx, &telego.EditMessageReplyMarkupParams{
			ChatID: tu.ID(chat), MessageID: q.Message.GetMessageID(),
		}); err != nil {
			log.Printf("telegram: clearing buttons failed: %v", err)
		}
	}
	if chat == 0 {
		return nil
	}

	switch q.Data {
	case cbRestartYes:
		b.handleRestartCallback(ctx, chat, p, true)
		return nil
	case cbRestartNo:
		b.handleRestartCallback(ctx, chat, p, false)
		return nil
	}

	question, idx, ok := parseCallback(q.Data)
	if !ok {
		return nil
	}
	if idx == skipIndex {
		b.declineChoice(ctx, chat, p, question)
		return nil
	}
	b.applyChoice(ctx, chat, p, question, choices[question].Choices[idx])
	return nil
}

// applyChoice writes the answer, then lets the model acknowledge it in its own voice.
//
// The write happens HERE, in Go, not by asking the model to save what was tapped. A button
// press is an unambiguous answer and it should not depend on a model choosing to record
// it. The model is then told it is already saved, so it acknowledges rather than writing
// it a second time.
func (b *Bot) applyChoice(ctx context.Context, chat int64, p storage.Profile,
	question string, c choice) {

	args := tools.UpdateProfileArgs{Profile: p.ID}
	c.apply(&args)
	if _, err := tools.UpdateProfile(b.Deps, args); err != nil {
		b.reply(ctx, chat, "Nggak bisa disimpan: "+err.Error())
		return
	}
	b.converse(ctx, chat, mustProfile(b.Deps, p.ID), c.Label, fmt.Sprintf(
		"They answered the %s question by tapping %q, and it is ALREADY SAVED. Do not "+
			"call update_profile for it. Acknowledge in a few words and carry on.",
		question, c.Label))
}

func (b *Bot) declineChoice(ctx context.Context, chat int64, p storage.Profile, question string) {
	if _, err := tools.UpdateProfile(b.Deps, tools.UpdateProfileArgs{
		Profile: p.ID, Declined: []string{question}}); err != nil {
		log.Printf("telegram: recording a decline failed: %v", err)
	}
	b.converse(ctx, chat, mustProfile(b.Deps, p.ID), "skip", fmt.Sprintf(
		"They declined the %s question by tapping Skip. It is already recorded as "+
			"declined and will not be asked again. Say something brief and unbothered — "+
			"two to six words — and carry on with whatever else is going on.", question))
}

// mustProfile re-reads the profile so the brief reflects what was just written. A stale
// copy here is what makes an assistant ask for something it was told a second ago.
func mustProfile(d tools.Deps, id string) storage.Profile {
	p, err := d.DB.GetProfile(id)
	if err != nil {
		return storage.Profile{ID: id}
	}
	return p
}
