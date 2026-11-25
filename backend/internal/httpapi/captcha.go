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
	captchaRedisPrefix     = "login:captcha:"
	captchaFailRedisPrefix = "login:fail-count:"
	captchaNeedTTLSeconds  = 1800

	slideWidth     = 300
	slideHeight    = 150
	slidePieceSize = 60
	slideHoleY     = 46
	slideTolerance = 10
)

// shouldRequireCaptcha: the switch is absolute — when enabled, every login attempt requires a captcha.
func shouldRequireCaptcha(enabled bool) bool {
	return enabled
}

type captchaIssueResponse struct {
	CaptchaID   string `json:"captchaId"`
	CaptchaType string `json:"captchaType"`
	ImageBase64 string `json:"imageBase64"`
	PieceBase64 string `json:"pieceBase64"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	PieceSize   int    `json:"pieceSize"`
	PieceTop    int    `json:"pieceTop"`
	ExpiresIn   int    `json:"expiresIn"`
}

func captchaIssueHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, background, piece, answer, pieceTop, err := issueSlideCaptcha()
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
			CaptchaType: "slide",
			ImageBase64: background,
			PieceBase64: piece,
			Width:       slideWidth,
			Height:      slideHeight,
			PieceSize:   slidePieceSize,
			PieceTop:    pieceTop,
			ExpiresIn:   captchaTTLSeconds,
		})
	}
}

func loginChallengeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		settings, err := ctx.Auth.LoginSecuritySettings(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.login_service_unavailable")
			return
		}
		lockMinutes := settings.LockMinutes
		if lockMinutes <= 0 {
			lockMinutes = 30
		}
		// lockMinutes mirrors security.login.lockMinutes (账号锁定分钟数) for login UI progress.
		ok(w, r, map[string]any{
			"captchaRequired": shouldRequireCaptcha(settings.CaptchaEnabled),
			"lockMinutes":     lockMinutes,
		})
	}
}

// issueSlideCaptcha builds a jigsaw-style slider captcha. The answer is the
// horizontal offset (in pixels) where the piece fits the hole.
func issueSlideCaptcha() (id, background, piece, answer string, pieceTop int, err error) {
	id = strings.ReplaceAll(uuid.NewString(), "-", "")
	holeX, err := randomInt(24, slideWidth-slidePieceSize-24)
	if err != nil {
		return "", "", "", "", 0, err
	}
	holeY, err := randomInt(24, slideHeight-slidePieceSize-24)
	if err != nil {
		return "", "", "", "", 0, err
	}
	pattern, err := buildCaptchaPattern()
	if err != nil {
		return "", "", "", "", 0, err
	}
	return id, buildSlideBackground(pattern, holeX, holeY), buildSlidePiece(pattern, holeX, holeY), strconv.Itoa(holeX), holeY, nil
}

// slidePieceShapes is the union of a rounded square and four knobs. The same
// geometry is used for the background hole and for clipping the dragged piece.
const slidePieceShapes = `<rect x="10" y="10" width="40" height="40" rx="8" %s/>
<circle cx="30" cy="10" r="10" %s/>
<circle cx="30" cy="50" r="10" %s/>
<circle cx="10" cy="30" r="10" %s/>
<circle cx="50" cy="30" r="10" %s/>`

func slideShapes(attrs string) string {
	return fmt.Sprintf(slidePieceShapes, attrs, attrs, attrs, attrs, attrs)
}

func buildSlideBackground(pattern string, holeX, holeY int) string {
	hole := slideShapes(`fill="#0f172a" fill-opacity="0.48" stroke="#f8fafc" stroke-width="2"`)
	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d"><rect width="100%%" height="100%%" fill="#e2e8f0"/><g>%s</g><g transform="translate(%d %d)">%s</g></svg>`,
		slideWidth, slideHeight, slideWidth, slideHeight, pattern, holeX, holeY, hole,
	)
	return svgDataURL(svg)
}

func buildSlidePiece(pattern string, holeX, holeY int) string {
	clip := slideShapes("")
	outline := slideShapes(`fill="none" stroke="#ffffff" stroke-width="2" stroke-opacity="0.95"`)
	svg := fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d"><defs><clipPath id="piece">%s</clipPath></defs><g clip-path="url(#piece)"><g transform="translate(-%d -%d)">%s</g></g>%s</svg>`,
		slidePieceSize, slidePieceSize, slidePieceSize, slidePieceSize, clip, holeX, holeY, pattern, outline,
	)
	return svgDataURL(svg)
}

func buildCaptchaPattern() (string, error) {
	palette := []string{"#0ea5e9", "#8b5cf6", "#f97316", "#22c55e", "#ec4899", "#facc15", "#14b8a6", "#6366f1"}
	var b strings.Builder
	for i := 0; i < 22; i++ {
		x, err := randomInt(-20, slideWidth-20)
		if err != nil {
			return "", err
		}
		y, err := randomInt(-20, slideHeight-20)
		if err != nil {
			return "", err
		}
		w, err := randomInt(24, 70)
		if err != nil {
			return "", err
		}
		h, err := randomInt(24, 58)
		if err != nil {
			return "", err
		}
		colorIndex, err := randomInt(0, len(palette)-1)
		if err != nil {
			return "", err
		}
		color := palette[colorIndex]
		if i%3 == 0 {
			b.WriteString(fmt.Sprintf(`<circle cx="%d" cy="%d" r="%d" fill="%s" opacity="0.9"/>`, x+w/2, y+h/2, w/2, color))
			continue
		}
		angle, err := randomInt(-35, 35)
		if err != nil {
			return "", err
		}
		b.WriteString(fmt.Sprintf(`<rect x="%d" y="%d" width="%d" height="%d" rx="8" fill="%s" opacity="0.9" transform="rotate(%d %d %d)"/>`,
			x, y, w, h, color, angle, x+w/2, y+h/2))
	}
	return b.String(), nil
}

func randomInt(min, max int) (int, error) {
	if max <= min {
		return min, nil
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(max-min+1)))
	if err != nil {
		return 0, err
	}
	return min + int(v.Int64()), nil
}

func svgDataURL(svg string) string {
	return "data:image/svg+xml;base64," + base64.StdEncoding.EncodeToString([]byte(svg))
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
	if err != nil {
		return errCaptchaInvalid
	}
	// go-zero returns ("", nil) for a missing key: timed out or already consumed.
	if stored == "" {
		return errCaptchaExpired
	}
	if !captchaAnswerMatches(stored, code) {
		return errCaptchaInvalid
	}
	return nil
}

func captchaAnswerMatches(stored, provided string) bool {
	want, err := strconv.Atoi(strings.TrimSpace(stored))
	if err != nil {
		return false
	}
	got, err := strconv.Atoi(strings.TrimSpace(provided))
	if err != nil {
		return false
	}
	diff := want - got
	if diff < 0 {
		diff = -diff
	}
	return diff <= slideTolerance
}

func clearUnknownFailCount(ctx *app.Context, reqCtx context.Context, username string) {
	name := strings.ToLower(strings.TrimSpace(username))
	if name == "" {
		return
	}
	_, _ = ctx.Cache.DelCtx(reqCtx, captchaFailRedisPrefix+name)
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

// Login-flow sentinels, compared with errors.Is by the login handler.
var (
	errCaptchaRequired = errors.New("captcha required")
	errCaptchaInvalid  = errors.New("captcha invalid")
	errCaptchaExpired  = errors.New("captcha expired")
)
