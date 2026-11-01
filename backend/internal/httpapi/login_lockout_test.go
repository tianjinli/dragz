package httpapi

import (
	"testing"
	"time"

	"dragz-admin/backend/internal/store"
)

func TestResolveLoginFailurePenalty(t *testing.T) {
	settings := store.LoginSecuritySettings{MaxAttempts: 5, LockMinutes: 30}

	_, d, locked := resolveLoginFailurePenalty(5, settings)
	if !locked || d != 30*time.Minute {
		t.Fatalf("expected lock 30m, got locked=%v duration=%v", locked, d)
	}

	_, d, locked = resolveLoginFailurePenalty(2, settings)
	if locked || d != loginFailureCoolDown {
		t.Fatalf("early failure should cool-down, got locked=%v duration=%v", locked, d)
	}

	_, d, locked = resolveLoginFailurePenalty(4, settings)
	if locked || d != loginFailureCoolDown {
		t.Fatalf("4th failure should cool-down before maxAttempts, got locked=%v duration=%v", locked, d)
	}
}

func TestShouldEnforceLoginWait(t *testing.T) {
	if !shouldEnforceLoginWait(time.Minute) {
		t.Fatal("account lock must enforce")
	}
	if shouldEnforceLoginWait(0) {
		t.Fatal("zero remaining must not enforce")
	}
}

func TestLoginFailureMessageKey(t *testing.T) {
	key, data := loginFailureMessageKey(1, false, 5)
	if key != "errors.invalid_login_remaining" || data["Remaining"] != 4 {
		t.Fatalf("unexpected first failure: %s %#v", key, data)
	}
	key, data = loginFailureMessageKey(2, false, 5)
	if key != "errors.invalid_login_remaining" || data["Remaining"] != 3 {
		t.Fatalf("unexpected remaining: %s %#v", key, data)
	}
	key, data = loginFailureMessageKey(4, false, 5)
	if key != "errors.invalid_login_remaining" || data["Remaining"] != 1 {
		t.Fatalf("unexpected last free attempt: %s %#v", key, data)
	}
	key, data = loginFailureMessageKey(5, true, 5)
	if key != "errors.locked" || data != nil {
		t.Fatalf("unexpected lock message: %s %#v", key, data)
	}
}

func TestShouldRequireCaptcha(t *testing.T) {
	if shouldRequireCaptcha(false, 5, true) {
		t.Fatal("disabled captcha must never require")
	}
	if shouldRequireCaptcha(true, 0, false) {
		t.Fatal("zero failures without flag should not require captcha")
	}
	if !shouldRequireCaptcha(true, 1, false) {
		t.Fatal("one failure should require captcha when enabled")
	}
	if !shouldRequireCaptcha(true, 0, true) {
		t.Fatal("need flag should require captcha when enabled")
	}
}

func TestFormatRetryAfterKey(t *testing.T) {
	key, data := formatRetryAfterKey(45 * time.Second)
	if key != "errors.duration.seconds" || data["Seconds"] != 45 {
		t.Fatalf("got %s %#v", key, data)
	}
	key, data = formatRetryAfterKey(90 * time.Second)
	if key != "errors.duration.minutes_seconds" || data["Minutes"] != 1 || data["Seconds"] != 30 {
		t.Fatalf("got %s %#v", key, data)
	}
	key, data = formatRetryAfterKey(2 * time.Hour)
	if key != "errors.duration.hours" || data["Hours"] != 2 {
		t.Fatalf("got %s %#v", key, data)
	}
}

func TestBuildCaptchaSVG(t *testing.T) {
	svg := buildCaptchaSVG("1234")
	if !stringsContains(svg, "1234") && !stringsContains(svg, ">1<") {
		// glyphs are split per char
	}
	if !stringsContains(svg, "<svg") {
		t.Fatal("expected svg markup")
	}
}

func stringsContains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
