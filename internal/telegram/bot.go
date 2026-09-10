package telegram

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/rakasatria/sehaty/internal/storage"
	"github.com/rakasatria/sehaty/internal/tools"
)

// Channel is the identity namespace. telegram/8412 and mcp/8412 are different people, and
// the identity table keeps them apart.
const Channel = "telegram"

type Bot struct {
	API        *API
	Deps       tools.Deps
	Passphrase string
}

// Run polls until the context is cancelled.
//
// Errors are logged and the loop continues. A bot that dies on one bad update is a bot
// that is silently offline the next time someone sends a voice note from a warung.
func (b *Bot) Run(ctx context.Context) {
	me, err := b.API.Me(ctx)
	if err != nil {
		log.Printf("telegram: cannot reach the bot API: %v", err)
		return
	}
	log.Printf("telegram: listening as @%s", me.Username)

	var offset int64
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		updates, err := b.API.GetUpdates(ctx, offset, 50)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("telegram: poll failed, retrying: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		for _, u := range updates {
			offset = u.UpdateID + 1
			if u.Message == nil {
				continue
			}
			// One bad message must not stop the loop or leak a stack trace to a user.
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Printf("telegram: panic handling update %d: %v", u.UpdateID, r)
					}
				}()
				b.handle(ctx, u.Message)
			}()
		}
	}
}

func (b *Bot) reply(ctx context.Context, chatID int64, text string) {
	if err := b.API.Send(ctx, chatID, text); err != nil {
		log.Printf("telegram: send failed: %v", err)
	}
}

// whoIs resolves a Telegram account to a profile, or reports that it is not registered.
func (b *Bot) whoIs(from *User) (storage.Profile, bool) {
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

func (b *Bot) handle(ctx context.Context, m *Message) {
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
	case m.BestPhoto() != nil:
		b.handlePhoto(ctx, chat, profile, m)
		return
	}

	lower := strings.ToLower(text)
	switch {
	case lower == "/start", lower == "/help", lower == "bantuan", lower == "help":
		b.reply(ctx, chat, help(profile))
	case lower == "/me", lower == "/profil", lower == "/profile":
		b.reply(ctx, chat, b.summary(profile))
	case lower == "/dash", lower == "/dashboard":
		b.handleDashboard(ctx, chat, profile)
	case strings.HasPrefix(lower, "/berat"), strings.HasPrefix(lower, "/weight"),
		strings.HasPrefix(lower, "berat "), strings.HasPrefix(lower, "weight "):
		b.handleWeight(ctx, chat, profile, text)
	default:
		b.reply(ctx, chat, unrecognised(text))
	}
}

func (b *Bot) handleUnregistered(ctx context.Context, m *Message, chat int64, text string) {
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
		b.reply(ctx, chat, fmt.Sprintf(
			"Registered as %s.\n\nYour record is private to this account. "+
				"Send a voice note or a meal photo and it goes straight in.\n\n%s",
			p.DisplayName, help(p)))
		return
	}
	b.reply(ctx, chat,
		"Sehaty — a private health record.\n\n"+
			"This account is not registered. Send the passphrase you were given, "+
			"on its own, to begin.\n\n"+
			"If you were not given one, this is not your server and there is nothing here for you.")
}

func (b *Bot) handleVoice(ctx context.Context, chat int64, p storage.Profile, m *Message) {
	f := m.Voice
	if f == nil {
		f = m.Audio
	}
	raw, err := b.API.Download(ctx, f.FileID)
	if err != nil {
		b.reply(ctx, chat, "Could not fetch that recording: "+err.Error())
		return
	}
	att, err := tools.AttachMedia(b.Deps, tools.AttachMediaArgs{
		Profile: p.ID, Kind: "voice",
		Data:     base64.StdEncoding.EncodeToString(raw),
		MIMEType: f.MimeType,
	})
	if err != nil {
		b.reply(ctx, chat, "Could not store that recording: "+err.Error())
		return
	}
	if b.Deps.Transcriber == nil || !b.Deps.Transcriber.Enabled() {
		b.reply(ctx, chat, fmt.Sprintf(
			"Recording saved (%ds). Transcription is not enabled on this server, "+
				"so it is stored but not read.", f.Duration))
		return
	}
	b.reply(ctx, chat, "Listening…")
	t, err := tools.TranscribeVoice(b.Deps, tools.TranscribeArgs{Profile: p.ID, Hash: att.Hash})
	if err != nil {
		b.reply(ctx, chat, "Saved, but could not transcribe it: "+err.Error())
		return
	}
	if strings.TrimSpace(t.Text) == "" {
		b.reply(ctx, chat, "Saved, but I could not hear any speech in it.")
		return
	}
	// Deliberately NOT acted on. This was heard by a model, not written by the person,
	// and logging a meal from a mishearing puts a wrong number in a health record.
	b.reply(ctx, chat, fmt.Sprintf(
		"I heard:\n\n“%s”\n\nSaved as a note. I have not logged anything from it — "+
			"tell me what to record if that is right.", t.Text))
}

func (b *Bot) handlePhoto(ctx context.Context, chat int64, p storage.Profile, m *Message) {
	ph := m.BestPhoto()
	raw, err := b.API.Download(ctx, ph.FileID)
	if err != nil {
		b.reply(ctx, chat, "Could not fetch that photo: "+err.Error())
		return
	}
	att, err := tools.AttachMedia(b.Deps, tools.AttachMediaArgs{
		Profile: p.ID, Kind: "photo",
		Data: base64.StdEncoding.EncodeToString(raw), MIMEType: "image/jpeg",
	})
	if err != nil {
		b.reply(ctx, chat, "Could not store that photo: "+err.Error())
		return
	}
	// No guess at what the food is or weighs. Identifying a dish from a photo is
	// 87-93% accurate; estimating its weight is not, and a wrong gram figure becomes a
	// wrong calorie figure that looks measured.
	b.reply(ctx, chat, fmt.Sprintf(
		"Photo saved (%.0f KB).\n\nI have not guessed what it is or how much it weighs — "+
			"tell me the food and the portion and I will log it against this photo.",
		float64(att.Bytes)/1024))
}

func (b *Bot) handleDashboard(ctx context.Context, chat int64, p storage.Profile) {
	out, err := tools.DashboardLink(b.Deps, tools.DashboardLinkArgs{Profile: p.ID})
	if err != nil {
		b.reply(ctx, chat, "Could not make a link: "+err.Error())
		return
	}
	b.reply(ctx, chat, fmt.Sprintf(
		"%s\n\nGood for one hour. Anyone holding this link can read your record until "+
			"it lapses, so do not forward it.", out.URL))
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
		b.reply(ctx, chat, "I could not find a weight in that. Try: berat 87.4")
		return
	}
	if err := b.Deps.DB.LogWeight(p.ID, storage.WeightEntry{
		Date: time.Now().Format("2006-01-02"), WeightKg: kg}); err != nil {
		b.reply(ctx, chat, "Could not record it: "+err.Error())
		return
	}
	b.reply(ctx, chat, fmt.Sprintf("Recorded %.1f kg for today.", kg))
}

func (b *Bot) summary(p storage.Profile) string {
	pr, err := tools.Progress(b.Deps, p.ID, 30)
	if err != nil {
		return "Could not read your record: " + err.Error()
	}
	w := "not recorded"
	if pr.LatestWeightKg != nil {
		w = fmt.Sprintf("%.1f kg", *pr.LatestWeightKg)
	}
	return fmt.Sprintf(
		"%s — last 30 days\n\nweight: %s\ntraining: %d sessions, %d sets\n"+
			"cardio: %d min\nfood logged on %d days\n\ngoal: %s",
		p.DisplayName, w, pr.LiftingSessions, pr.SetsLogged,
		int(pr.CardioMinutes), pr.FoodDaysLogged, strings.ReplaceAll(p.Goal, "_", " "))
}

func help(p storage.Profile) string {
	return "What I can do:\n\n" +
		"• send a voice note — I save it and tell you what I heard\n" +
		"• send a meal photo — I save it against your record\n" +
		"• berat 87.4 — record your weight\n" +
		"• /me — your last 30 days\n" +
		"• /dash — a dashboard link, good for one hour\n\n" +
		"I do not guess. If I am not sure what something is, I will say so rather than " +
		"record a number nobody measured."
}

func unrecognised(text string) string {
	if len(text) > 60 {
		text = text[:60] + "…"
	}
	return fmt.Sprintf(
		"I did not understand “%s”.\n\nI am a simple interface, not a conversation — "+
			"I understand voice notes, photos, and a few commands. Send /help to see them.",
		text)
}
