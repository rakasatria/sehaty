// Package transcribe turns a voice note into text via OpenRouter.
//
// THIS IS THE ONE PLACE SEHATY SENDS DATA OFF THE MACHINE. Everything else — the database,
// the media, the documents — stays encrypted on disk and never leaves. A voice note cannot
// be transcribed locally on a 2-core box with any accuracy in Bahasa Indonesia, so this
// exists, and it is OFF unless a key is configured.
//
// data_collection is set to "deny" so OpenRouter routes only to providers that do not
// retain or train on the request. That is a routing instruction, not a cryptographic
// guarantee: the audio does leave the machine, and anyone deploying this should know that
// rather than discover it.
package transcribe

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const endpoint = "https://openrouter.ai/api/v1/chat/completions"

// DefaultModel is chosen for cost and for Bahasa Indonesia.
//
// Audio is billed at the prompt rate here (~32 tokens per second of audio), so a
// thirty-second note costs a fraction of a cent. Google's Indonesian coverage is strong;
// the speech-specialist alternative (Voxtral) is European-focused and, per audio token,
// three orders of magnitude more expensive.
const DefaultModel = "google/gemini-2.5-flash-lite"

// The instruction is deliberately narrow. A model asked to "understand" a voice note will
// summarise, tidy grammar and translate — and a health record needs what was SAID, not a
// helpful paraphrase of it.
const instruction = `Transcribe this audio exactly as spoken. Rules:
- Write only the words spoken. No summary, no commentary, no translation.
- Keep the original language. Indonesian stays Indonesian; English stays English.
- Keep code-switching as spoken — Indonesians mix English words in constantly.
- Numbers and units as said ("dua ratus gram", not "200g") unless the digits were spoken.
- If a passage is genuinely inaudible, write [tidak jelas] and continue.
- If there is no speech at all, reply with exactly: [no speech]
Reply with the transcript and nothing else.`

type Client struct {
	Key   string
	Model string
	HTTP  *http.Client
}

func New(key, model string) *Client {
	if model == "" {
		model = DefaultModel
	}
	return &Client{Key: key, Model: model,
		HTTP: &http.Client{Timeout: 120 * time.Second}}
}

// Enabled reports whether transcription is configured. With no key the feature is simply
// absent — voice notes are still stored, just not read.
func (c *Client) Enabled() bool { return c != nil && c.Key != "" }

type Result struct {
	Text     string
	Model    string
	Language string
}

// formatFor maps a MIME type to the format string OpenRouter expects.
func formatFor(mime string) string {
	switch {
	case strings.Contains(mime, "ogg"), strings.Contains(mime, "opus"):
		return "ogg"
	case strings.Contains(mime, "mpeg"), strings.Contains(mime, "mp3"):
		return "mp3"
	case strings.Contains(mime, "wav"), strings.Contains(mime, "wave"):
		return "wav"
	case strings.Contains(mime, "m4a"), strings.Contains(mime, "mp4"), strings.Contains(mime, "aac"):
		return "m4a"
	case strings.Contains(mime, "flac"):
		return "flac"
	case strings.Contains(mime, "webm"):
		return "webm"
	default:
		return "ogg" // what Telegram and most phone recorders send
	}
}

func (c *Client) Transcribe(ctx context.Context, audio []byte, mime string) (Result, error) {
	if !c.Enabled() {
		return Result{}, fmt.Errorf("transcription is not configured on this server")
	}
	if len(audio) == 0 {
		return Result{}, fmt.Errorf("no audio to transcribe")
	}
	body := map[string]any{
		"model": c.Model,
		// Route only to providers that do not retain or train on the request.
		"provider": map[string]any{"data_collection": "deny"},
		"messages": []any{map[string]any{
			"role": "user",
			"content": []any{
				map[string]any{"type": "text", "text": instruction},
				map[string]any{"type": "input_audio", "input_audio": map[string]any{
					"data":   base64.StdEncoding.EncodeToString(audio),
					"format": formatFor(mime),
				}},
			},
		}},
		"temperature": 0,
	}
	raw, err := json.Marshal(body)
	if err != nil {
		return Result{}, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(raw))
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Title", "Sehaty")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("transcription request: %w", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode != http.StatusOK {
		return Result{}, fmt.Errorf("transcription failed (%d): %s",
			resp.StatusCode, firstLine(string(out)))
	}
	var parsed struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return Result{}, fmt.Errorf("unreadable transcription response: %w", err)
	}
	if len(parsed.Choices) == 0 {
		return Result{}, fmt.Errorf("the model returned no transcript")
	}
	text := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if text == "" || text == "[no speech]" {
		return Result{Model: c.Model}, nil // silence is a valid answer, not an error
	}
	return Result{Text: text, Model: c.Model, Language: guessLang(text)}, nil
}

// guessLang is a coarse hint for display, not a claim. It looks for words that are common
// in Indonesian and rare in English; anything ambiguous is left blank rather than guessed.
func guessLang(s string) string {
	l := " " + strings.ToLower(s) + " "
	for _, w := range []string{" yang ", " tadi ", " saya ", " makan ", " dan ", " dengan ",
		" sudah ", " belum ", " nasi ", " ini ", " itu ", " tidak "} {
		if strings.Contains(l, w) {
			return "id"
		}
	}
	return ""
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i > 0 {
		s = s[:i]
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}
