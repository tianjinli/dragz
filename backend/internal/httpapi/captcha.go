package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"dragz-admin/backend/internal/app"
)

const (
	captchaTTLSeconds      = 300
	captchaCodeLen         = 4
	captchaRedisPrefix     = "login:captcha:"
	captchaNeedRedisPrefix = "login:captcha-need:"
	captchaFailRedisPrefix = "login:fail-count:"
	captchaNeedTTLSeconds  = 1800
)

// shouldRequireCaptcha: when captcha feature is enabled, require after ≥1 failure (or sticky need flag).
func shouldRequireCaptcha(enabled bool, failedCount int, hasNeedFlag bool) bool {
	if !enabled {
		return false
	}
	return failedCount >= 1 || hasNeedFlag
}

type captchaIssueResponse struct {
	CaptchaID   string `json:"captchaId"`
	ImageBase64 string `json:"imageBase64"`
	ExpiresIn   int    `json:"expiresIn"`
}

func captchaIssueHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, image, answer, err := issueCaptchaImage()
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.captcha_generate_failed")
			return
		}
		if err := ctx.Cache.SetexCtx(r.Context(), captchaRedisPrefix+id, answer, captchaTTLSeconds); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.captcha_unavailable")
			return
		}
		ok(w, r, captchaIssueResponse{
			CaptchaID:   id,
			ImageBase64: image,
			ExpiresIn:   captchaTTLSeconds,
		})
	}
}

func loginChallengeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username := strings.TrimSpace(r.URL.Query().Get("username"))
		settings, err := ctx.Auth.LoginSecuritySettings(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.login_service_unavailable")
			return
		}
		failedCount := 0
		if username != "" {
			if user, findErr := ctx.Auth.FindUserByUsername(r.Context(), username); findErr == nil {
				failedCount = user.FailedLoginCount
			} else {
				failedCount = unknownFailCount(ctx, r.Context(), username)
			}
		}
		required := shouldRequireCaptcha(settings.CaptchaEnabled, failedCount, hasCaptchaNeedFlag(ctx, r.Context(), username))
		ok(w, r, map[string]any{"captchaRequired": required})
	}
}

func issueCaptchaImage() (id, imageDataURL, answer string, err error) {
	id = strings.ReplaceAll(uuid.NewString(), "-", "")
	answer, err = randomDigits(captchaCodeLen)
	if err != nil {
		return "", "", "", err
	}
	svg := buildCaptchaSVG(answer)
	imageDataURL = "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
	return id, imageDataURL, answer, nil
}

func randomDigits(n int) (string, error) {
	var b strings.Builder
	b.Grow(n)
	for i := 0; i < n; i++ {
		v, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			return "", err
		}
		b.WriteByte(byte('0' + v.Int64()))
	}
	return b.String(), nil
}

func buildCaptchaSVG(answer string) string {
	var glyphs strings.Builder
	for i, ch := range answer {
		x := 18 + i*28
		y := 28 + (i%2)*4
		rotate := (i%3 - 1) * 12
		glyphs.WriteString(fmt.Sprintf(
			`<text x="%d" y="%d" font-size="28" font-family="Arial,sans-serif" font-weight="700" fill="#1f2937" transform="rotate(%d %d %d)">%c</text>`,
			x, y, rotate, x, y, ch,
		))
	}
	noise := ""
	for i := 0; i < 5; i++ {
		noise += fmt.Sprintf(`<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#94a3b8" stroke-width="1" opacity="0.55"/>`,
			i*20, 8+i*7, 40+i*18, 40-i*5)
	}
	return fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="140" height="44" viewBox="0 0 140 44"><rect width="140" height="44" rx="6" fill="#f1f5f9"/>%s%s</svg>`,
		noise, glyphs.String(),
	)
}

func verifyAndConsumeCaptcha(ctx *app.Context, reqCtx context.Context, captchaID, captchaCode string) error {
	id := strings.TrimSpace(captchaID)
	code := strings.TrimSpace(captchaCode)
	if id == "" || code == "" {
		return errCaptchaRequired
	}
	key := captchaRedisPrefix + id
	stored, err := ctx.Cache.GetCtx(reqCtx, key)
	_, _ = ctx.Cache.DelCtx(reqCtx, key)
	if err != nil || stored == "" {
		return errCaptchaInvalid
	}
	if !strings.EqualFold(stored, code) {
		return errCaptchaInvalid
	}
	return nil
}

func markCaptchaRequired(ctx *app.Context, reqCtx context.Context, username string) {
	name := strings.ToLower(strings.TrimSpace(username))
	if name == "" {
		return
	}
	_ = ctx.Cache.SetexCtx(reqCtx, captchaNeedRedisPrefix+name, "1", captchaNeedTTLSeconds)
}

func clearCaptchaRequired(ctx *app.Context, reqCtx context.Context, username string) {
	name := strings.ToLower(strings.TrimSpace(username))
	if name == "" {
		return
	}
	_, _ = ctx.Cache.DelCtx(reqCtx, captchaNeedRedisPrefix+name)
	_, _ = ctx.Cache.DelCtx(reqCtx, captchaFailRedisPrefix+name)
}

func hasCaptchaNeedFlag(ctx *app.Context, reqCtx context.Context, username string) bool {
	name := strings.ToLower(strings.TrimSpace(username))
	if name == "" {
		return false
	}
	value, err := ctx.Cache.GetCtx(reqCtx, captchaNeedRedisPrefix+name)
	return err == nil && value != ""
}

func unknownFailCount(ctx *app.Context, reqCtx context.Context, username string) int {
	name := strings.ToLower(strings.TrimSpace(username))
	if name == "" {
		return 0
	}
	value, err := ctx.Cache.GetCtx(reqCtx, captchaFailRedisPrefix+name)
	if err != nil || value == "" {
		return 0
	}
	n, err := strconv.Atoi(value)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func bumpUnknownFailCount(ctx *app.Context, reqCtx context.Context, username string) int {
	name := strings.ToLower(strings.TrimSpace(username))
	if name == "" {
		return 0
	}
	key := captchaFailRedisPrefix + name
	n, err := ctx.Cache.IncrCtx(reqCtx, key)
	if err != nil {
		return 0
	}
	_ = ctx.Cache.ExpireCtx(reqCtx, key, captchaNeedTTLSeconds)
	return int(n)
}

func noteCaptchaAfterFailure(ctx *app.Context, reqCtx context.Context, username string, enabled bool, failedCount int) bool {
	required := shouldRequireCaptcha(enabled, failedCount, false)
	if required {
		markCaptchaRequired(ctx, reqCtx, username)
	}
	return required
}

// Login-flow sentinels, compared with errors.Is by the login handler.
var (
	errCaptchaRequired = errors.New("captcha required")
	errCaptchaInvalid  = errors.New("captcha invalid")
)
