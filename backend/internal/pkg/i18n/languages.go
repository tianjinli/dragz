package i18n

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// SourceLanguage is the language preset data is authored in. It is stored
// inline in the owning tables' text columns, so it never needs a
// sys_translation row and never triggers a lookup.
const SourceLanguage = "zh"

// targetLanguages lists the languages preset data can be translated into, as
// sys_translation.lang codes. Chinese is the source and never appears here.
//
// English is the only target today: the admin ships a Chinese and an English
// interface, and preset data (menus, roles, departments, ...) is maintained in
// both. The list is the single place to extend when the product needs another
// language — Lang, the translation page and its validation all read from here.
var targetLanguages = []string{"en"}

// TargetLanguage is the language preset data is translated into first, i.e. the
// one the translation page edits next to the Chinese source. Empty when no
// target is registered.
func TargetLanguage() string {
	if len(targetLanguages) == 0 {
		return ""
	}
	return targetLanguages[0]
}

// baseTag lowercases a BCP-47 tag and keeps the primary subtag: "en-US" -> "en".
func baseTag(tag string) string {
	tag = strings.ToLower(strings.TrimSpace(tag))
	if index := strings.IndexAny(tag, "-_"); index >= 0 {
		tag = tag[:index]
	}
	return tag
}

// IsTarget reports whether code is a language preset data can be translated
// into, i.e. a translated language rather than the Chinese source.
func IsTarget(code string) bool {
	code = baseTag(code)
	if code == "" {
		return false
	}
	for _, lang := range targetLanguages {
		if lang == code {
			return true
		}
	}
	return false
}

// Resolve maps a request tag to a readable language code. ok is false when the
// tag is neither Chinese nor a supported target, so the caller can keep looking
// for a better entry in the header.
func Resolve(tag string) (code string, ok bool) {
	base := baseTag(tag)
	switch {
	case base == "":
		return SourceLanguage, false
	case base == SourceLanguage:
		return SourceLanguage, true
	case IsTarget(base):
		return base, true
	default:
		return SourceLanguage, false
	}
}

// Lang returns the language code the request should read preset data in: the
// highest-weighted Accept-Language entry this build supports, else Chinese.
func Lang(r *http.Request) string {
	if r == nil {
		return SourceLanguage
	}
	header := strings.TrimSpace(r.Header.Get("Accept-Language"))
	if header == "" {
		return SourceLanguage
	}
	type weighted struct {
		tag   string
		q     float64
		order int
	}
	entries := make([]weighted, 0, 4)
	for order, part := range strings.Split(header, ",") {
		fields := strings.Split(part, ";")
		tag := strings.TrimSpace(fields[0])
		if tag == "" {
			continue
		}
		quality := 1.0
		for _, param := range fields[1:] {
			key, value, found := strings.Cut(strings.TrimSpace(param), "=")
			if !found || !strings.EqualFold(strings.TrimSpace(key), "q") {
				continue
			}
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64); err == nil {
				quality = parsed
			}
		}
		if quality <= 0 {
			continue
		}
		entries = append(entries, weighted{tag: tag, q: quality, order: order})
	}
	// Stable sort: entries with the same weight keep their header order.
	sort.SliceStable(entries, func(a, b int) bool { return entries[a].q > entries[b].q })
	for _, entry := range entries {
		if code, ok := Resolve(entry.tag); ok {
			return code
		}
	}
	return SourceLanguage
}
