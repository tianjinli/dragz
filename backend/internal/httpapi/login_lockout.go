package httpapi

import (
	"time"

	"dragz-admin/backend/internal/store"
)

// loginFailureCoolDown applies after every failed login under all mechanisms.
const loginFailureCoolDown = 3 * time.Second

// resolveLoginFailurePenalty returns lock/cool-down window for a failure.
// Account lock: failedCount >= maxAttempts → lock for lockMinutes (independent of captcha).
// Otherwise every failure gets a flat 3s cool-down.
func resolveLoginFailurePenalty(failedCount int, settings store.LoginSecuritySettings) (until time.Time, duration time.Duration, locked bool) {
	if settings.MaxAttempts > 0 && failedCount >= settings.MaxAttempts {
		mins := settings.LockMinutes
		if mins <= 0 {
			mins = 30
		}
		d := time.Duration(mins) * time.Minute
		return time.Now().Add(d), d, true
	}
	return time.Now().Add(loginFailureCoolDown), loginFailureCoolDown, false
}

func shouldEnforceLoginWait(remaining time.Duration) bool {
	return remaining > 0
}

func isAccountLockWait(failedCount, maxAttempts int, remaining time.Duration) bool {
	if remaining <= 0 {
		return false
	}
	if maxAttempts > 0 && failedCount >= maxAttempts {
		return true
	}
	return remaining > loginFailureCoolDown+time.Second
}

func loginAttemptsRemaining(failedCount, maxAttempts int) int {
	remaining := maxAttempts - failedCount
	if remaining < 0 {
		return 0
	}
	return remaining
}

// formatRetryAfterKey returns an i18n key and template data for a duration label.
func formatRetryAfterKey(d time.Duration) (string, map[string]any) {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	totalSec := int(d.Seconds())
	if totalSec < 60 {
		return "errors.duration.seconds", map[string]any{"Seconds": totalSec}
	}
	hours := totalSec / 3600
	mins := (totalSec % 3600) / 60
	secs := totalSec % 60
	if hours > 0 {
		if mins == 0 {
			return "errors.duration.hours", map[string]any{"Hours": hours}
		}
		return "errors.duration.hours_minutes", map[string]any{"Hours": hours, "Minutes": mins}
	}
	if secs == 0 {
		return "errors.duration.minutes", map[string]any{"Minutes": mins}
	}
	return "errors.duration.minutes_seconds", map[string]any{"Minutes": mins, "Seconds": secs}
}

func loginFailureMessageKey(failedCount int, locked bool, maxAttempts int) (string, map[string]any) {
	if locked {
		return "errors.locked", nil
	}
	if maxAttempts > 0 {
		remaining := loginAttemptsRemaining(failedCount, maxAttempts)
		if failedCount >= 1 && remaining > 0 {
			return "errors.invalid_login_remaining", map[string]any{"Remaining": remaining}
		}
	}
	return "errors.invalid_login", nil
}

type loginLockData struct {
	Kind              string `json:"kind"` // "cooldown" | "lock"
	RetryAfterSeconds int    `json:"retryAfterSeconds"`
	LockedUntil       string `json:"lockedUntil,omitempty"`
	FailedCount       int    `json:"failedCount,omitempty"`
	AttemptsRemaining int    `json:"attemptsRemaining,omitempty"`
	CaptchaRequired   bool   `json:"captchaRequired"`
}

func newLoginLockData(until time.Time, duration time.Duration, locked bool, failedCount int, maxAttempts int, captchaRequired bool) loginLockData {
	kind := "cooldown"
	if locked {
		kind = "lock"
	}
	data := loginLockData{
		Kind:              kind,
		RetryAfterSeconds: int(duration.Round(time.Second).Seconds()),
		LockedUntil:       until.UTC().Format(time.RFC3339),
		FailedCount:       failedCount,
		CaptchaRequired:   captchaRequired,
	}
	if !locked && maxAttempts > 0 {
		data.AttemptsRemaining = loginAttemptsRemaining(failedCount, maxAttempts)
	}
	return data
}
