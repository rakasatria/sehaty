package tools

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/rakasatria/sehaty/internal/storage"
)

// Seeded keys. Agents reuse names they can SEE; duplicates appear when the namespace is
// invisible, so list_documents advertises these even before anything is written.
var seededKeys = []string{"diet", "clinical-notes", "injury-history", "medications"}

var keyOK = regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
var nonKey = regexp.MustCompile(`[^a-z0-9]+`)

// canonicalKey normalises a document key so the STORE owns spelling discipline, not the
// caller. "Diet_Plan", "diet plan" and "Diet  Plan" all become "diet-plan".
//
// (Unicode normalisation is deliberately not done — it needs golang.org/x/text, and keys
// are constrained to ASCII anyway. A non-ASCII key is rejected rather than folded.)
func canonicalKey(s string) (string, error) {
	k := nonKey.ReplaceAllString(strings.ToLower(strings.TrimSpace(s)), "-")
	k = strings.Trim(k, "-")
	if k == "" {
		return "", fmt.Errorf("document key is required")
	}
	if len(k) > 64 {
		return "", fmt.Errorf("document key %q is too long (max 64 characters)", s)
	}
	if !keyOK.MatchString(k) {
		return "", fmt.Errorf("document key %q is not usable; use lowercase words separated by hyphens", s)
	}
	return k, nil
}

// squash strips separators so near-misses collide: diet-plan and dietplan both squash to
// "dietplan". Canonicalisation alone would leave those as two documents.
func squash(k string) string { return strings.ReplaceAll(k, "-", "") }

type PutDocumentArgs struct {
	Profile         string `json:"profile"`
	Key             string `json:"key" jsonschema:"short slug, e.g. diet or clinical-notes. Call list_documents first to see what already exists"`
	Body            string `json:"body" jsonschema:"the document in markdown. No YAML front matter — title and kind are separate fields"`
	Title           string `json:"title,omitempty"`
	Kind            string `json:"kind,omitempty" jsonschema:"prescription, note, history"`
	ExpectedVersion int    `json:"expected_version,omitempty" jsonschema:"the version you last read. Required when updating: it stops a stale copy overwriting newer content. Omit only when creating"`
	CreateNew       bool   `json:"create_new,omitempty" jsonschema:"true to create a document that does not exist yet"`
}

type PutDocumentOut struct {
	Profile string `json:"profile"`
	Key     string `json:"key"`
	Version int    `json:"version"`
	Title   string `json:"title,omitempty"`
	Note    string `json:"note"`
}

func PutDocument(d Deps, a PutDocumentArgs) (PutDocumentOut, error) {
	if _, err := requireProfile(d, a.Profile); err != nil {
		return PutDocumentOut{}, err
	}
	key, err := canonicalKey(a.Key)
	if err != nil {
		return PutDocumentOut{}, err
	}
	if strings.TrimSpace(a.Body) == "" {
		return PutDocumentOut{}, fmt.Errorf("document body is empty; nothing to store")
	}

	existing, err := d.DB.ListDocuments(a.Profile)
	if err != nil {
		return PutDocumentOut{}, err
	}
	var head int
	var known []string
	var headTitle, headKind string
	for _, e := range existing {
		known = append(known, e.Key)
		if e.Key == key {
			head, headTitle, headKind = e.Version, e.Title, e.Kind
		}
	}

	if head == 0 {
		// Creating. Guard against a near-duplicate of something that already exists.
		for _, e := range known {
			if squash(e) == squash(key) {
				return PutDocumentOut{}, fmt.Errorf(
					"refusing to create %q: %q already exists and is the same name in "+
						"different clothing. Write to %q instead", key, e, e)
			}
		}
		if !a.CreateNew {
			return PutDocumentOut{}, fmt.Errorf(
				"%q does not exist. Pass create_new to create it, or write to one of: %s",
				key, strings.Join(append(known, seededKeys...), ", "))
		}
	} else if a.ExpectedVersion == 0 {
		return PutDocumentOut{}, fmt.Errorf(
			"%q exists at version %d. Read it, then pass expected_version=%d so a stale "+
				"copy cannot overwrite newer content", key, head, head)
	}

	expected := a.ExpectedVersion
	if head == 0 {
		expected = 0
	}
	// An update that omits title or kind must CARRY THEM FORWARD, not blank them. Writing
	// a new body previously replaced "Nutritionist plan" with the bare key, quietly losing
	// metadata the caller never intended to touch.
	title := a.Title
	if title == "" {
		title = headTitle
	}
	if title == "" {
		title = key
	}
	kind := a.Kind
	if kind == "" {
		kind = headKind
	}
	if kind == "" {
		kind = "note"
	}
	v, err := d.DB.PutDocumentIfVersion(d.Cipher, a.Profile, storage.Document{
		Key: key, Title: title, Kind: kind, Body: a.Body, Encrypted: true}, expected)
	if err != nil {
		if errors.Is(err, storage.ErrVersionConflict) {
			return PutDocumentOut{}, fmt.Errorf("%w — re-read the document and try again", err)
		}
		return PutDocumentOut{}, err
	}
	note := fmt.Sprintf("Saved %q as version %d.", key, v)
	if v > 1 {
		note += fmt.Sprintf(" Version %d is still readable.", v-1)
	}
	return PutDocumentOut{Profile: a.Profile, Key: key, Version: v, Title: title, Note: note}, nil
}

type GetDocumentArgs struct {
	Profile string `json:"profile"`
	Key     string `json:"key"`
	Version int    `json:"version,omitempty" jsonschema:"omit for the current version"`
}

type GetDocumentOut struct {
	Profile   string `json:"profile"`
	Key       string `json:"key"`
	Version   int    `json:"version"`
	Title     string `json:"title,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Body      string `json:"body"`
	UpdatedAt string `json:"updated_at,omitempty"`
	Note      string `json:"note,omitempty"`
}

func GetDocument(d Deps, a GetDocumentArgs) (GetDocumentOut, error) {
	key, err := canonicalKey(a.Key)
	if err != nil {
		return GetDocumentOut{}, err
	}
	doc, err := d.DB.GetDocument(d.Cipher, a.Profile, key, a.Version)
	if err != nil {
		return GetDocumentOut{}, err
	}
	out := GetDocumentOut{Profile: a.Profile, Key: doc.Key, Version: doc.Version,
		Title: doc.Title, Kind: doc.Kind, Body: doc.Body, UpdatedAt: doc.UpdatedAt,
		Note: fmt.Sprintf("Version %d. Pass expected_version=%d when writing changes back.",
			doc.Version, doc.Version)}
	return out, nil
}

type DocumentSummary struct {
	Key       string `json:"key"`
	Title     string `json:"title,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Version   int    `json:"version"`
	UpdatedAt string `json:"updated_at,omitempty"`
}

type ListDocumentsOut struct {
	Count     int               `json:"count"`
	Documents []DocumentSummary `json:"documents"`
	Suggested []string          `json:"suggested_keys,omitempty"`
	Note      string            `json:"note,omitempty"`
}

// ListDocuments never returns bodies: listing what exists must not decrypt what nobody
// asked for.
func ListDocuments(d Deps, profileID string) (ListDocumentsOut, error) {
	docs, err := d.DB.ListDocuments(profileID)
	if err != nil {
		return ListDocumentsOut{}, err
	}
	out := ListDocumentsOut{Count: len(docs), Documents: []DocumentSummary{}}
	have := map[string]bool{}
	for _, doc := range docs {
		have[doc.Key] = true
		out.Documents = append(out.Documents, DocumentSummary{Key: doc.Key, Title: doc.Title,
			Kind: doc.Kind, Version: doc.Version, UpdatedAt: doc.UpdatedAt})
	}
	for _, k := range seededKeys {
		if !have[k] {
			out.Suggested = append(out.Suggested, k)
		}
	}
	if len(out.Suggested) > 0 {
		out.Note = "Use one of the suggested keys rather than inventing a new name for the same thing."
	}
	return out, nil
}

type DocumentHistoryOut struct {
	Key      string            `json:"key"`
	Count    int               `json:"count"`
	Versions []DocumentSummary `json:"versions"`
}

func DocumentHistory(d Deps, profileID, key string) (DocumentHistoryOut, error) {
	k, err := canonicalKey(key)
	if err != nil {
		return DocumentHistoryOut{}, err
	}
	hist, err := d.DB.DocumentVersions(profileID, k)
	if err != nil {
		return DocumentHistoryOut{}, err
	}
	out := DocumentHistoryOut{Key: k, Count: len(hist), Versions: []DocumentSummary{}}
	for _, h := range hist {
		out.Versions = append(out.Versions, DocumentSummary{Key: h.Key, Title: h.Title,
			Kind: h.Kind, Version: h.Version, UpdatedAt: h.UpdatedAt})
	}
	return out, nil
}
