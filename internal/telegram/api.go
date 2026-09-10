// Package telegram is the second front door.
//
// It uses LONG POLLING rather than webhooks. A webhook needs a public HTTPS endpoint,
// which would mean exposing this server to the internet — the one thing the whole design
// avoids. Polling reaches out from behind the LAN and needs nothing published.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type API struct {
	Token string
	HTTP  *http.Client
	base  string
}

func New(token string) *API {
	return &API{Token: token, HTTP: &http.Client{Timeout: 90 * time.Second},
		base: "https://api.telegram.org"}
}

func (a *API) Enabled() bool { return a != nil && a.Token != "" }

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

type File struct {
	FileID   string `json:"file_id"`
	FileSize int    `json:"file_size"`
	MimeType string `json:"mime_type"`
	Duration int    `json:"duration"`
}

type Message struct {
	MessageID int64 `json:"message_id"`
	From      *User `json:"from"`
	Chat      struct {
		ID int64 `json:"id"`
	} `json:"chat"`
	Date     int64  `json:"date"`
	Text     string `json:"text"`
	Caption  string `json:"caption"`
	Voice    *File  `json:"voice"`
	Audio    *File  `json:"audio"`
	Photo    []File `json:"photo"`
	Document *File  `json:"document"`
}

// BestPhoto returns the largest rendition Telegram offers.
//
// Telegram sends several sizes, smallest first. The largest is the one worth keeping: a
// meal photo is stored once and looked at later, and a thumbnail cannot be un-shrunk.
func (m *Message) BestPhoto() *File {
	if len(m.Photo) == 0 {
		return nil
	}
	best := m.Photo[0]
	for _, p := range m.Photo {
		if p.FileSize > best.FileSize {
			best = p
		}
	}
	return &best
}

type Update struct {
	UpdateID int64    `json:"update_id"`
	Message  *Message `json:"message"`
}

func (a *API) call(ctx context.Context, method string, params url.Values, out any) error {
	u := fmt.Sprintf("%s/bot%s/%s", a.base, a.Token, method)
	req, err := http.NewRequestWithContext(ctx, "POST", u,
		strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := a.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	var env struct {
		OK          bool            `json:"ok"`
		Description string          `json:"description"`
		Result      json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return fmt.Errorf("%s: unreadable response: %w", method, err)
	}
	if !env.OK {
		return fmt.Errorf("%s: %s", method, env.Description)
	}
	if out != nil {
		return json.Unmarshal(env.Result, out)
	}
	return nil
}

// GetUpdates long-polls. Telegram holds the connection open until something happens, so
// this is one request per event rather than a busy loop.
func (a *API) GetUpdates(ctx context.Context, offset int64, timeout int) ([]Update, error) {
	p := url.Values{}
	p.Set("offset", fmt.Sprint(offset))
	p.Set("timeout", fmt.Sprint(timeout))
	p.Set("allowed_updates", `["message"]`)
	var out []Update
	err := a.call(ctx, "getUpdates", p, &out)
	return out, err
}

func (a *API) Send(ctx context.Context, chatID int64, text string) error {
	p := url.Values{}
	p.Set("chat_id", fmt.Sprint(chatID))
	// Telegram rejects messages over 4096 characters outright, which would turn a long
	// reply into no reply at all.
	if len(text) > 4000 {
		text = text[:3990] + "\n…"
	}
	p.Set("text", text)
	p.Set("disable_web_page_preview", "true")
	return a.call(ctx, "sendMessage", p, nil)
}

func (a *API) Me(ctx context.Context) (User, error) {
	var u User
	err := a.call(ctx, "getMe", url.Values{}, &u)
	return u, err
}

// Download fetches a file's bytes. Telegram gives a path first, then serves the content
// from a different URL.
func (a *API) Download(ctx context.Context, fileID string) ([]byte, error) {
	p := url.Values{}
	p.Set("file_id", fileID)
	var meta struct {
		FilePath string `json:"file_path"`
		FileSize int    `json:"file_size"`
	}
	if err := a.call(ctx, "getFile", p, &meta); err != nil {
		return nil, err
	}
	if meta.FilePath == "" {
		return nil, fmt.Errorf("telegram returned no path for %s", fileID)
	}
	u := fmt.Sprintf("%s/file/bot%s/%s", a.base, a.Token, meta.FilePath)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := a.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: HTTP %d", fileID, resp.StatusCode)
	}
	// Bounded: an unbounded read from a remote service is an out-of-memory waiting to
	// happen, and nothing legitimate here is larger than a photo.
	return io.ReadAll(io.LimitReader(resp.Body, 30<<20))
}

var _ = multipart.NewWriter // reserved for sending files back
var _ = bytes.NewReader
