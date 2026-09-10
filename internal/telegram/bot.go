// Package telegram is the second front door.
//
// It uses LONG POLLING rather than webhooks. A webhook needs a public HTTPS endpoint,
// which would mean exposing this server to the internet — the one thing the whole design
// avoids. Polling reaches out from behind the LAN and needs nothing published.
//
// The Bot API client is telego. The transport, the retry behaviour and the update routing
// are not written here; what is written here is which buttons exist and what tapping one
// means — see keyboards.go, where the meaning lives in the callback DATA rather than in a
// handler registered in memory, so a button still works after a restart.
package telegram

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mymmrac/telego"
	th "github.com/mymmrac/telego/telegohandler"
	tu "github.com/mymmrac/telego/telegoutil"

	"github.com/rakasatria/sehaty/internal/agent"
	"github.com/rakasatria/sehaty/internal/storage"
	"github.com/rakasatria/sehaty/internal/tools"
)

// Channel is the identity namespace. telegram/8412 and mcp/8412 are different people, and
// the identity table keeps them apart.
const Channel = "telegram"

type Bot struct {
	Token      string
	Deps       tools.Deps
	Passphrase string

	// DashURL is the public https address the dashboard is published at. Telegram will
	// only open a Mini App over https, so without it the button is simply not offered.
	DashURL string

	// Agent turns ordinary sentences into tool calls. Optional: without it the bot keeps
	// working as a command interface and says so rather than ignoring what it cannot read.
	Agent *agent.Client

	tg    *telego.Bot
	chats *conversations
}

func (b *Bot) Enabled() bool { return b != nil && b.Token != "" }

// Run polls until the context is cancelled.
func (b *Bot) Run(ctx context.Context) {
	if !b.Enabled() {
		log.Print("telegram: not configured")
		return
	}
	b.chats = newConversations()

	tg, err := telego.NewBot(b.Token)
	if err != nil {
		log.Printf("telegram: cannot start: %v", err)
		return
	}
	b.tg = tg

	if me, err := tg.GetMe(ctx); err == nil {
		log.Printf("telegram: listening as @%s", me.Username)
	}
	// The command menu is the only part of the interface discoverable without reading a
	// help message — Telegram shows it as a tap-to-run list when someone types "/".
	if err := tg.SetMyCommands(ctx, &telego.SetMyCommandsParams{
		Commands: []telego.BotCommand{
			{Command: "me", Description: "Ringkasan 30 hari terakhir"},
			{Command: "dash", Description: "Link dashboard, berlaku 1 jam"},
			{Command: "berat", Description: "Catat berat badan — /berat 87.4"},
			{Command: "lupakan", Description: "Lupakan obrolan ini (catatan tetap aman)"},
			{Command: "ulang", Description: "Ulangi assessment dari awal"},
			{Command: "help", Description: "Apa saja yang bisa aku lakukan"},
		},
	}); err != nil {
		log.Printf("telegram: could not publish the command menu: %v", err)
	}

	updates, err := tg.UpdatesViaLongPolling(ctx, nil)
	if err != nil {
		log.Printf("telegram: cannot poll: %v", err)
		return
	}
	bh, err := th.NewBotHandler(tg, updates)
	if err != nil {
		log.Printf("telegram: cannot route updates: %v", err)
		return
	}
	defer func() { _ = bh.Stop() }()

	// Button presses first — they are the only updates with a prefix we own.
	bh.HandleCallbackQuery(b.onCallback, th.CallbackDataPrefix(cbPrefix))
	bh.Handle(b.onUpdate)

	if err := bh.Start(); err != nil && ctx.Err() == nil {
		log.Printf("telegram: stopped: %v", err)
	}
}

// onUpdate handles everything that is not a button press.
func (b *Bot) onUpdate(ctx *th.Context, u telego.Update) error {
	// One bad message must not stop the loop or leak a stack trace to a user.
	defer func() {
		if r := recover(); r != nil {
			log.Printf("telegram: panic handling update %d: %v", u.UpdateID, r)
		}
	}()
	if u.Message == nil {
		return nil
	}
	b.handle(ctx, u.Message)
	return nil
}

func (b *Bot) reply(ctx context.Context, chatID int64, text string) {
	if _, err := b.tg.SendMessage(ctx, tu.Message(tu.ID(chatID), clamp(text)).
		WithLinkPreviewOptions(&telego.LinkPreviewOptions{IsDisabled: true})); err != nil {
		log.Printf("telegram: send failed: %v", err)
	}
}

// clamp keeps a reply inside Telegram's limit. Over 4096 characters it is rejected
// outright, which turns a long answer into no answer at all.
func clamp(text string) string {
	if len(text) > 4000 {
		return text[:3990] + "\n…"
	}
	return text
}

// whoIs resolves a Telegram account to a profile, or reports that it is not registered.
func (b *Bot) whoIs(from *telego.User) (storage.Profile, bool) {
	if from == nil {
		return storage.Profile{}, false
	}
	id, err := b.Deps.DB.ResolveIdentity(Channel, strconv.FormatInt(from.ID, 10))
	if err != nil {
		return storage.Profile{}, false
	}
	p, err := b.Deps.DB.GetProfile(id)
	return p, err == nil
}

func (b *Bot) handle(ctx context.Context, m *telego.Message) {
	chat := m.Chat.ID
	profile, registered := b.whoIs(m.From)
	text := strings.TrimSpace(m.Text)
	if text == "" {
		text = strings.TrimSpace(m.Caption)
	}

	// Registration is the only thing an unknown account may do.
	if !registered {
		b.handleUnregistered(ctx, m, chat, text)
		return
	}

	switch {
	case m.Voice != nil || m.Audio != nil:
		b.handleVoice(ctx, chat, profile, m)
		return
	case len(m.Photo) > 0:
		b.handlePhoto(ctx, chat, profile, m)
		return
	}

	lower := strings.ToLower(text)
	switch {
	case lower == "/start", lower == "/help", lower == "/bantuan":
		b.reply(ctx, chat, help(profile))
	case lower == "/me", lower == "/profil", lower == "/profile":
		b.reply(ctx, chat, b.summary(profile))
	case lower == "/dash", lower == "/dashboard":
		b.handleDashboard(ctx, chat, profile)
	case lower == "/lupakan", lower == "/reset", lower == "/forget":
		b.chats.forget(chat)
		b.reply(ctx, chat, "Oke, obrolan tadi aku lupain. Catatanmu nggak kesentuh — "+
			"yang hilang cuma benang obrolannya.")
	case lower == "/ulang", lower == "/ulangi":
		b.handleRestart(ctx, chat, profile)
	case strings.HasPrefix(lower, "/berat"), strings.HasPrefix(lower, "/weight"),
		strings.HasPrefix(lower, "berat "), strings.HasPrefix(lower, "weight "):
		b.handleWeight(ctx, chat, profile, text)
	default:
		// Anything else is a sentence, not a command. Note that the tools it reaches
		// keep every refusal they have: the model can ask to log a meal, it cannot
		// invent what the meal contained.
		b.converse(ctx, chat, profile, text, "")
	}
}

// handleRestart is deliberately a command rather than something the model can be talked
// into. Clearing someone's answers on a misread sentence would be a bad afternoon.
func (b *Bot) handleRestart(ctx context.Context, chat int64, p storage.Profile) {
	kb := tu.InlineKeyboard(
		tu.InlineKeyboardRow(telego.InlineKeyboardButton{
			Text: "Ya, ulangi dari awal", CallbackData: cbRestartYes}),
		tu.InlineKeyboardRow(telego.InlineKeyboardButton{
			Text: "Nggak jadi", CallbackData: cbRestartNo}),
	)
	if _, err := b.tg.SendMessage(ctx, tu.Message(tu.ID(chat),
		"Mau ulangi assessment dari awal? Jawaban profilmu dihapus dan pertanyaannya "+
			"mulai lagi.\n\nLatihan, makanan, dan berat yang udah tercatat tetap aman — "+
			"itu nggak bisa dihapus dari sini sama sekali.").WithReplyMarkup(kb)); err != nil {
		log.Printf("telegram: restart prompt failed: %v", err)
	}
}

func (b *Bot) handleUnregistered(ctx context.Context, m *telego.Message, chat int64, text string) {
	if b.Passphrase == "" {
		b.reply(ctx, chat, "Registration is closed on this server. "+
			"The person running it has not set a passphrase.")
		return
	}
	if strings.TrimSpace(text) == b.Passphrase {
		name := "Someone"
		if m.From != nil && m.From.FirstName != "" {
			name = m.From.FirstName
		}
		p, err := tools.Register(b.Deps, Channel, strconv.FormatInt(m.From.ID, 10),
			name, b.Passphrase, b.Passphrase)
		if err != nil {
			b.reply(ctx, chat, "Could not register: "+err.Error())
			return
		}
		// Written to set the expectation that questions are coming, that each one has
		// a reason, and that "skip" is a whole answer — because software asking a
		// person their age has to earn it.
		b.reply(ctx, chat, fmt.Sprintf(
			"You're in, %s. This record is yours alone — nobody else on this server "+
				"can read it.\n\n"+
				"Tell me what happened — a meal, a session, this morning's weight — "+
				"however you'd say it to a person. I keep the numbers straight and "+
				"answer from them.\n\n"+
				"Now and then I'll ask you something: your height, an old injury, that "+
				"kind of thing. Always one question, always with a reason, and \"skip\" "+
				"is a complete answer. None of it ever becomes a calorie target — that "+
				"number belongs to your dietitian.\n\n"+
				"Whenever something happens, tell me.\n\n%s",
			p.DisplayName, help(p)))
		return
	}
	b.reply(ctx, chat,
		"Sehaty — a private health record.\n\n"+
			"I don't know this account yet. If someone gave you a passphrase, send it "+
			"on its own and we'll start.\n\n"+
			"If nobody gave you one, this server isn't yours and there's nothing here "+
			"for you.")
}

func (b *Bot) handleVoice(ctx context.Context, chat int64, p storage.Profile, m *telego.Message) {
	fileID, duration, mime := "", 0, ""
	if m.Voice != nil {
		fileID, duration, mime = m.Voice.FileID, m.Voice.Duration, m.Voice.MimeType
	} else {
		fileID, duration, mime = m.Audio.FileID, m.Audio.Duration, m.Audio.MimeType
	}
	raw, err := b.download(ctx, fileID)
	if err != nil {
		b.reply(ctx, chat, "Telegram wouldn't give me that recording: "+err.Error())
		return
	}
	att, err := tools.AttachMedia(b.Deps, tools.AttachMediaArgs{
		Profile: p.ID, Kind: "voice",
		Data:     base64.StdEncoding.EncodeToString(raw),
		MIMEType: mime,
	})
	if err != nil {
		b.reply(ctx, chat, "I couldn't save that recording: "+err.Error())
		return
	}
	if b.Deps.Transcriber == nil || !b.Deps.Transcriber.Enabled() {
		b.reply(ctx, chat, fmt.Sprintf(
			"Kept your recording (%ds), but I can't listen to it — transcription "+
				"isn't switched on here. It's stored, unread. Type it out and I'll "+
				"take it that way.", duration))
		return
	}
	b.typing(ctx, chat)
	t, err := tools.TranscribeVoice(b.Deps, tools.TranscribeArgs{Profile: p.ID, Hash: att.Hash})
	if err != nil {
		b.reply(ctx, chat, "I kept the recording, but couldn't make out the words: "+err.Error())
		return
	}
	if strings.TrimSpace(t.Text) == "" {
		b.reply(ctx, chat, "I kept the recording, but there's no speech in it that I can hear.")
		return
	}
	// The transcript is shown before it is acted on, every time, so a mishearing is
	// visible to the person it belongs to rather than buried inside a reply.
	b.reply(ctx, chat, fmt.Sprintf("I heard: “%s”", t.Text))
	b.converse(ctx, chat, p, t.Text, "The message you are answering was transcribed "+
		"from a voice note and may contain mishearings, especially food names and "+
		"numbers. Read anything back before you save it.")
}

func (b *Bot) handlePhoto(ctx context.Context, chat int64, p storage.Profile, m *telego.Message) {
	raw, err := b.download(ctx, bestPhoto(m.Photo).FileID)
	if err != nil {
		b.reply(ctx, chat, "Telegram wouldn't give me that photo: "+err.Error())
		return
	}
	att, err := tools.AttachMedia(b.Deps, tools.AttachMediaArgs{
		Profile: p.ID, Kind: "photo",
		Data: base64.StdEncoding.EncodeToString(raw), MIMEType: "image/jpeg",
	})
	if err != nil {
		b.reply(ctx, chat, "I couldn't save that photo: "+err.Error())
		return
	}
	// No guess at what the food is or weighs. Identifying a dish from a photo is
	// 87-93% accurate; estimating its weight is not, and a wrong gram figure becomes a
	// wrong calorie figure that looks measured.
	b.reply(ctx, chat, fmt.Sprintf(
		"Got the photo (%.0f KB), saved to today.\n\nI'm not going to guess what's on "+
			"the plate or what it weighs — that's how made-up numbers end up in a health "+
			"record. Tell me what it is and roughly how much, and I'll log it against "+
			"this picture.", float64(att.Bytes)/1024))
}

func (b *Bot) handleDashboard(ctx context.Context, chat int64, p storage.Profile) {
	var rows [][]telego.InlineKeyboardButton

	// The Mini App comes first because it is the better door: Telegram signs who is
	// opening it, so nothing has to be handed out that would work for whoever holds it.
	if strings.HasPrefix(b.DashURL, "https://") {
		rows = append(rows, tu.InlineKeyboardRow(telego.InlineKeyboardButton{
			Text:   "Buka di sini",
			WebApp: &telego.WebAppInfo{URL: b.DashURL + "/app"},
		}))
	}

	out, err := tools.DashboardLink(b.Deps, tools.DashboardLinkArgs{Profile: p.ID})
	if err == nil {
		rows = append(rows, tu.InlineKeyboardRow(telego.InlineKeyboardButton{
			Text: "Buka di browser", URL: out.URL,
		}))
	}
	if len(rows) == 0 {
		b.reply(ctx, chat, "Nggak bisa bikin link: "+err.Error())
		return
	}

	text := "Catatanmu."
	if len(rows) == 2 {
		text = "Catatanmu.\n\n“Buka di sini” tetap di dalam Telegram — Telegram yang " +
			"bilang ke aku bahwa itu kamu, jadi nggak ada link yang perlu dijaga.\n\n" +
			"“Buka di browser” bikin link yang hidup satu jam. Selama masih hidup, siapa " +
			"pun yang pegang bisa baca — jadi jangan diteruskan."
	}
	if _, err := b.tg.SendMessage(ctx, tu.Message(tu.ID(chat), text).
		WithReplyMarkup(tu.InlineKeyboard(rows...))); err != nil {
		log.Printf("telegram: dashboard send failed: %v", err)
	}
}

func (b *Bot) handleWeight(ctx context.Context, chat int64, p storage.Profile, text string) {
	fields := strings.Fields(strings.ReplaceAll(text, ",", "."))
	var kg float64
	var ok bool
	for _, f := range fields {
		f = strings.TrimSuffix(strings.TrimSuffix(f, "kg"), "KG")
		if v, err := strconv.ParseFloat(f, 64); err == nil && v > 20 && v < 400 {
			kg, ok = v, true
			break
		}
	}
	if !ok {
		b.reply(ctx, chat, "Aku nggak nemu angkanya. Coba kayak gini: berat 87.4")
		return
	}
	if err := b.Deps.DB.LogWeight(p.ID, storage.WeightEntry{
		Date: time.Now().Format("2006-01-02"), WeightKg: kg}); err != nil {
		b.reply(ctx, chat, "Nggak bisa dicatat: "+err.Error())
		return
	}
	b.reply(ctx, chat, fmt.Sprintf("%.1f kg, tercatat buat hari ini.", kg))
}

func (b *Bot) summary(p storage.Profile) string {
	pr, err := tools.Progress(b.Deps, p.ID, 30)
	if err != nil {
		return "I could not read your record just now: " + err.Error()
	}
	w := "belum pernah dicatat"
	if pr.LatestWeightKg != nil {
		w = fmt.Sprintf("%.1f kg", *pr.LatestWeightKg)
	}
	return fmt.Sprintf(
		"%s — 30 hari terakhir\n\n"+
			"berat       %s\n"+
			"latihan     %d sesi, %d set\n"+
			"kardio      %d menit\n"+
			"makan       tercatat %d hari\n\n"+
			"lagi ngejar: %s",
		p.DisplayName, w, pr.LiftingSessions, pr.SetsLogged,
		int(pr.CardioMinutes), pr.FoodDaysLogged, strings.ReplaceAll(p.Goal, "_", " "))
}

func help(p storage.Profile) string {
	return "Mostly, just talk to me:\n\n" +
		"“ayam goreng 150 gram tadi siang”\n" +
		"“bench press 4 sets of 8 at 60 kg”\n" +
		"“how did last week go?”\n\n" +
		"Voice notes and photos of your food work as well. And a few shortcuts:\n\n" +
		"• berat 87.4 — your weight this morning\n" +
		"• /me — the last thirty days\n" +
		"• /dash — a dashboard link, good for an hour\n" +
		"• /ulang — start the questions over\n" +
		"• /lupakan — forget this thread (your record stays)\n\n" +
		"One thing worth knowing: I don't guess. If I can't tell which food you mean, " +
		"or how much of it there was, I'll ask instead of picking something plausible. " +
		"A health record is only worth having if every number in it is real."
}

// download fetches a file's bytes.
//
// Bounded: an unbounded read from a remote service is an out-of-memory waiting to happen.
// The Bot API caps downloads at 20 MB anyway — below Sehaty's own 25 MB media limit, which
// is worth remembering the day a long recording is refused for no visible reason.
func (b *Bot) download(ctx context.Context, fileID string) ([]byte, error) {
	f, err := b.tg.GetFile(ctx, &telego.GetFileParams{FileID: fileID})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", b.tg.FileDownloadURL(f.FilePath), nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", fileID, resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 30<<20))
}

// typing shows the hint while the model thinks. Cosmetic, so failure is ignored: a reply
// that arrives without it is still the reply.
func (b *Bot) typing(ctx context.Context, chat int64) {
	_ = b.tg.SendChatAction(ctx, &telego.SendChatActionParams{
		ChatID: tu.ID(chat), Action: telego.ChatActionTyping,
	})
}

// bestPhoto picks the largest rendition Telegram offers.
//
// It sends several sizes, smallest first, and the largest is the one worth keeping: a meal
// photo is stored once and looked at later, and a thumbnail cannot be un-shrunk.
func bestPhoto(sizes []telego.PhotoSize) telego.PhotoSize {
	best := sizes[0]
	for _, p := range sizes {
		if p.FileSize > best.FileSize {
			best = p
		}
	}
	return best
}

// The restart confirmation carries its own callback data, for the same reason the choice
// buttons do: a "yes" tapped after a redeploy should still mean yes.
const (
	cbRestartYes = cbPrefix + "r:1"
	cbRestartNo  = cbPrefix + "r:0"
)

// handleRestartCallback answers the confirmation.
func (b *Bot) handleRestartCallback(ctx context.Context, chat int64, p storage.Profile, yes bool) {
	if !yes {
		b.reply(ctx, chat, "Oke, nggak jadi.")
		return
	}
	if _, err := tools.ResetAssessment(b.Deps, p.ID); err != nil {
		b.reply(ctx, chat, "Nggak bisa diulang: "+err.Error())
		return
	}
	b.chats.forget(chat)
	b.converse(ctx, chat, mustProfile(b.Deps, p.ID), "ulangi assessment",
		"Their assessment answers were just cleared at their request. Nothing they "+
			"logged was deleted. Say so in one line, then ask the first question.")
}
