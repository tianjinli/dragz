package i18n

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStoredMessageRendersInReaderLanguage(t *testing.T) {
	translator, err := NewTranslator()
	if err != nil {
		t.Fatalf("NewTranslator: %v", err)
	}
	raw := StoredMessage("errors.plugin.dependency_disabled_detail", map[string]any{"Plugins": "mall"})
	if !strings.HasPrefix(raw, storedPrefix) {
		t.Fatalf("stored value = %q, want the %q prefix", raw, storedPrefix)
	}

	cases := map[string]string{
		"zh": "依赖的插件未启用：mall",
		"en": "Required plugin disabled: mall",
	}
	for language, want := range cases {
		r := httptest.NewRequest("GET", "/", nil)
		r.Header.Set("Accept-Language", language)
		if got := translator.LocalizeStored(r, raw); got != want {
			t.Fatalf("%s: LocalizeStored = %q, want %q", language, got, want)
		}
	}
}

func TestLocalizeStoredAcceptsBareKeyAndLegacyText(t *testing.T) {
	translator, err := NewTranslator()
	if err != nil {
		t.Fatalf("NewTranslator: %v", err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.Header.Set("Accept-Language", "en")

	if got := translator.LocalizeStored(r, "login_log.reason.account_disabled"); got != "Account is disabled" {
		t.Fatalf("bare key = %q, want translated text", got)
	}
	// Rows written before failure reasons became message IDs keep their text.
	if got := translator.LocalizeStored(r, "用户名或密码错误"); got != "用户名或密码错误" {
		t.Fatalf("legacy text = %q, want it returned unchanged", got)
	}
	if got := translator.LocalizeStored(r, ""); got != "" {
		t.Fatalf("empty = %q, want empty", got)
	}
}
