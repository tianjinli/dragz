package appkit

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/tianjinli/dragz/internal/i18n"
)

const JwtExpiresAtKey = "exp"
const JwtIssuerUriKey = "iss"
const JwtRefreshTokenKey = "ref"

const JwtClaimsKey = "X-USER"

const RedisRefreshTokenKey = "refresh_token:%s"
const CookieRefreshTokenKey = "refresh_token"

type JwtAuthService interface {
	CreateAccessToken(ctx context.Context, claims jwt.MapClaims) (time.Time, string, error)
	// CreateRefreshToken redisID: "{user-id}:{uuid-v1}"
	CreateRefreshToken(ctx context.Context, claims jwt.MapClaims, redisID string) (time.Time, string, error)
	DeleteRefreshToken(ctx context.Context, redisID string) error
	// RevokeRefreshToken redisMask: "{user-id}:*"
	RevokeRefreshToken(ctx context.Context, redisMask string) error
	ParseAccessClaims(ctx context.Context, token string) (jwt.MapClaims, error)
	ParseRefreshClaims(ctx context.Context, token string) (jwt.MapClaims, error)
}

func SetRefreshTokenCookie(ctx *gin.Context, refreshToken string, expiresAt time.Time, path, domain string) {
	maxAge := int(expiresAt.Sub(time.Now()).Seconds())
	ctx.SetCookie(CookieRefreshTokenKey, refreshToken, maxAge, path, domain, !Debug, true)
}

func GetRefreshTokenCookie(ctx *gin.Context) (string, error) {
	return ctx.Cookie(CookieRefreshTokenKey)
}

func StoreCustomClaims(ctx *gin.Context, user jwt.MapClaims) {
	ctx.Set(JwtClaimsKey, user)
}

func LoadCustomClaims(ctx *gin.Context) (jwt.MapClaims, error) {
	if value, exists := ctx.Get(JwtClaimsKey); exists {
		if claims, ok := value.(jwt.MapClaims); ok {
			return claims, nil
		}
	}
	return nil, NewForbidden(i18n.ErrTokenInvalidClaims)
}
