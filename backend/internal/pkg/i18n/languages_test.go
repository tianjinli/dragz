package i18n

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func requestWithLanguage(header string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/system/menus", nil)
	if header != "" {
		r.Header.Set("Accept-Language", header)
	}
	return r
}

func TestLangResolvesSupportedLanguages(t *testing.T) {
	cases := []struct {
		header string
		want   string
	}{
		{"", SourceLanguage},
		{"zh-CN", SourceLanguage},
		{"zh", SourceLanguage},
		{"en-US", "en"},
		{"en", "en"},
		{"en-GB", "en"},
		// The header is ordered by weight, not by position.
		{"zh-CN,en-US;q=0.9", SourceLanguage},
		{"zh-CN;q=0.8,en-US;q=0.9", "en"},
		// Languages outside the two shipped languages fall back to Chinese.
		{"ja-JP", SourceLanguage},
		{"ja-JP,en-US;q=0.9", "en"},
		{"de-DE,de;q=0.9,en;q=0.8", "en"},
		// q=0 means "not acceptable".
		{"en;q=0", SourceLanguage},
	}
	for _, testCase := range cases {
		if got := Lang(requestWithLanguage(testCase.header)); got != testCase.want {
			t.Errorf("Lang(%q) = %q, want %q", testCase.header, got, testCase.want)
		}
	}
}

func TestIsTargetExcludesSourceLanguage(t *testing.T) {
	if IsTarget(SourceLanguage) {
		t.Fatalf("IsTarget(%q) = true, the source language is not a translation target", SourceLanguage)
	}
	if !IsTarget("en") || !IsTarget("en-US") {
		t.Fatal("IsTarget rejected English")
	}
	for _, code := range []string{"", "xx", "ja", "de"} {
		if IsTarget(code) {
			t.Errorf("IsTarget(%q) = true for an unregistered language", code)
		}
	}
}
