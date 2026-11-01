package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The admin ships exactly two message packs, Chinese and English. Chinese is
// the bundle default, so an unregistered language (ja-JP, de-DE, ...) gets
// Chinese messages rather than raw keys.
func TestTranslatorResolvesShippedLanguages(t *testing.T) {
	translator, err := NewTranslator()
	if err != nil {
		t.Fatalf("NewTranslator: %v", err)
	}
	const key = "errors.translation_not_found"

	requestFor := func(header string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "/api/system/translations", nil)
		r.Header.Set("Accept-Language", header)
		return r
	}
	english := translator.T(requestFor("en-US"), key)
	chinese := translator.T(requestFor("zh-CN"), key)
	if english == key || chinese == key {
		t.Fatalf("locale files are missing %q: en=%q zh=%q", key, english, chinese)
	}
	if english == chinese {
		t.Fatalf("en and zh returned the same text %q", english)
	}
	if got := translator.T(requestFor("ja-JP"), key); got != chinese {
		t.Errorf("ja-JP message = %q, want the Chinese source %q", got, chinese)
	}
}
