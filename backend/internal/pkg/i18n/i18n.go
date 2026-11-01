// Package i18n loads the embedded locale files and localises message keys per
// request.
package i18n

import (
	"embed"
	"encoding/json"
	"net/http"
	"strings"

	goi18n "github.com/nicksnyder/go-i18n/v2/i18n"
	"golang.org/x/text/language"
)

//go:embed locale/*.json
var localeFS embed.FS

// Translator wraps the go-i18n bundle built from the embedded locale files.
type Translator struct {
	bundle *goi18n.Bundle
	// ids is the set of message IDs the bundle knows, so callers can tell a
	// stored message ID apart from arbitrary admin-entered text.
	ids map[string]bool
}

// NewTranslator loads every locale/*.json next to this package into one bundle.
// Interface strings ship as Chinese + English only, and Chinese is the bundle
// default — the same default as the admin UI, so both sides always agree on
// what a request without a usable language tag reads.
func NewTranslator() (*Translator, error) {
	bundle := goi18n.NewBundle(language.Chinese)
	bundle.RegisterUnmarshalFunc("json", json.Unmarshal)
	ids := map[string]bool{}
	entries, err := localeFS.ReadDir("locale")
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		content, readErr := localeFS.ReadFile("locale/" + entry.Name())
		if readErr != nil {
			return nil, readErr
		}
		if _, parseErr := bundle.ParseMessageFileBytes(content, entry.Name()); parseErr != nil {
			return nil, parseErr
		}
		// The locale files are flat string maps; record every key so Has can
		// distinguish stored message IDs from plain text.
		var flat map[string]string
		if unmarshalErr := json.Unmarshal(content, &flat); unmarshalErr == nil {
			for id := range flat {
				ids[id] = true
			}
		}
	}
	return &Translator{bundle: bundle, ids: ids}, nil
}

// Has reports whether messageID exists in the bundle.
func (t *Translator) Has(messageID string) bool {
	if t == nil {
		return false
	}
	return t.ids[messageID]
}

// Localizer builds a localizer for one request from its Accept-Language header, falling back to Chinese.
func (t *Translator) Localizer(r *http.Request) *goi18n.Localizer {
	tags := []string{}
	if header := strings.TrimSpace(r.Header.Get("Accept-Language")); header != "" {
		for _, part := range strings.Split(header, ",") {
			tag := strings.TrimSpace(strings.Split(part, ";")[0])
			if tag != "" {
				tags = append(tags, tag)
			}
		}
	}
	if len(tags) == 0 {
		tags = []string{SourceLanguage}
	}
	return goi18n.NewLocalizer(t.bundle, tags...)
}

// T localizes messageID. Optional templateData supports go-i18n templates (e.g. {{.Remaining}}).
func (t *Translator) T(r *http.Request, messageID string, templateData ...map[string]any) string {
	if t == nil {
		return messageID
	}
	cfg := &goi18n.LocalizeConfig{MessageID: messageID}
	if len(templateData) > 0 && templateData[0] != nil {
		cfg.TemplateData = templateData[0]
	}
	value, err := t.Localizer(r).Localize(cfg)
	if err != nil || value == "" {
		return messageID
	}
	return value
}

// storedPrefix marks a text column value that carries a message ID plus
// template data instead of rendered text, so a message written before the
// reader's language was known still renders in that language.
const storedPrefix = "i18n:"

type storedMessage struct {
	ID   string         `json:"id"`
	Data map[string]any `json:"data,omitempty"`
}

// StoredMessage encodes messageID and its template data for storage in a text
// column (login logs, plugin errors). Render it with LocalizeStored.
func StoredMessage(messageID string, templateData map[string]any) string {
	payload, err := json.Marshal(storedMessage{ID: messageID, Data: templateData})
	if err != nil {
		return messageID
	}
	return storedPrefix + string(payload)
}

// LocalizeStored renders a value produced by StoredMessage or a bare message
// ID. Anything else (legacy rows written as plain text) is translated as a
// message ID and, finding none, returned unchanged.
//
// Template data values that are themselves message IDs (for example a
// permission label whose noun is stored as "perm.noun.user") are translated
// first, so templates like "新增{{.noun}}" compose in the reader's language.
func (t *Translator) LocalizeStored(r *http.Request, raw string) string {
	if t == nil || raw == "" {
		return raw
	}
	if payload, ok := strings.CutPrefix(raw, storedPrefix); ok {
		var stored storedMessage
		if err := json.Unmarshal([]byte(payload), &stored); err == nil && stored.ID != "" {
			for key, value := range stored.Data {
				if text, isText := value.(string); isText && t.Has(text) {
					stored.Data[key] = t.T(r, text)
				}
			}
			return t.T(r, stored.ID, stored.Data)
		}
	}
	return t.T(r, raw)
}

// Lang lives in languages.go: it resolves any language registered there, so
// adding a language does not touch this file.
