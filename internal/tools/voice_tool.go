package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/rakasatria/sehaty/internal/media"
)

type TranscribeArgs struct {
	Profile string `json:"profile"`
	Hash    string `json:"hash" jsonschema:"content hash of the voice note, from attach_media or list_media"`
	Force   bool   `json:"force,omitempty" jsonschema:"re-transcribe even if a transcript already exists"`
}

type TranscribeOut struct {
	Profile  string `json:"profile"`
	Hash     string `json:"hash"`
	Text     string `json:"text"`
	Source   string `json:"source"`
	Language string `json:"language,omitempty"`
	Cached   bool   `json:"cached"`
	Note     string `json:"note,omitempty"`
}

// TranscribeVoice reads a stored voice note.
//
// Cached by default: transcription is the one operation that sends data off the machine,
// so doing it twice for the same recording spends money and privacy for a result already
// on disk.
func TranscribeVoice(d Deps, a TranscribeArgs) (TranscribeOut, error) {
	if _, err := requireProfile(d, a.Profile); err != nil {
		return TranscribeOut{}, err
	}
	if !a.Force {
		if t, err := d.DB.GetTranscript(a.Profile, a.Hash); err == nil {
			return TranscribeOut{Profile: a.Profile, Hash: a.Hash, Text: t.Text,
				Source: t.Source, Language: t.Language, Cached: true}, nil
		}
	}
	if d.Transcriber == nil || !d.Transcriber.Enabled() {
		return TranscribeOut{}, fmt.Errorf(
			"transcription is not configured on this server; the recording is stored and " +
				"can be played, but nothing can read it")
	}
	raw, kind, err := GetMediaBytes(d, GetMediaArgs{Profile: a.Profile, Kind: "voice", Hash: a.Hash})
	if err != nil {
		return TranscribeOut{}, err
	}
	if kind != media.KindVoice {
		return TranscribeOut{}, fmt.Errorf("%s is not a voice note", a.Hash)
	}
	res, err := d.Transcriber.Transcribe(context.Background(), raw, sniffMIME(raw, kind))
	if err != nil {
		return TranscribeOut{}, err
	}
	out := TranscribeOut{Profile: a.Profile, Hash: a.Hash, Text: res.Text,
		Source: res.Model, Language: res.Language}
	if strings.TrimSpace(res.Text) == "" {
		out.Note = "No speech was found in this recording."
		return out, nil
	}
	if err := d.DB.PutTranscript(a.Profile, a.Hash, res.Text, res.Model, res.Language); err != nil {
		return TranscribeOut{}, err
	}
	out.Note = "Transcribed by a model, so treat it as heard rather than as said. " +
		"Confirm anything you are about to log from it."
	return out, nil
}
