package jwtauth

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/tianjinli/dragz/internal/i18n"
	"github.com/tianjinli/dragz/pkg/appkit"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

type jwtAuthService struct {
	logger *zap.Logger
	cache  redis.UniversalClient
	conf   *appkit.TokenConfig
}

func NewJwtAuthService(
	logger *zap.Logger,
	cache redis.UniversalClient,
	conf *appkit.TokenConfig,
) appkit.JwtAuthService {
	const (
		minAccessExpiresIn  = 1 * time.Minute
		maxAccessExpiresIn  = 24 * time.Hour
		minRefreshExpiresIn = 7 * 24 * time.Hour
		maxRefreshExpiresIn = 90 * 24 * time.Hour
	)

	conf.AccessExpiresIn = clamp(conf.AccessExpiresIn, minAccessExpiresIn, maxAccessExpiresIn)
	conf.RefreshExpiresIn = clamp(conf.RefreshExpiresIn, minRefreshExpiresIn, maxRefreshExpiresIn)

	if conf.IssuerUri != "" {
		jwt.WithIssuer(conf.IssuerUri)
	}
	return &jwtAuthService{logger: logger, cache: cache, conf: conf}
}

func clamp(d, min, max time.Duration) time.Duration {
	if d < min {
		return min
	}
	if d > max {
		return max
	}
	return d
}

func (s *jwtAuthService) wrapI18nError(err error) error {
	switch {
	case errors.Is(err, nil):
		return nil
	case errors.Is(err, jwt.ErrTokenMalformed):
		return appkit.NewBadRequest(i18n.ErrTokenMalformed)
	case errors.Is(err, jwt.ErrTokenNotValidYet):
		return appkit.NewBadRequest(i18n.ErrTokenNotValidYet)
	case errors.Is(err, jwt.ErrTokenExpired):
		return appkit.NewUnauthorized(i18n.ErrTokenExpired)
	case errors.Is(err, jwt.ErrTokenSignatureInvalid):
		return appkit.NewUnauthorized(i18n.ErrTokenSignatureInvalid)
	case errors.Is(err, jwt.ErrTokenUnverifiable):
		return appkit.NewUnauthorized(i18n.ErrTokenUnverifiable)
	case errors.Is(err, jwt.ErrTokenInvalidIssuer):
		return appkit.NewUnauthorized(i18n.ErrTokenInvalidIssuer)
	default:
		return appkit.NewInternalServerError(i18n.ErrTokenInvalid)
	}
}

func (s *jwtAuthService) createToken(claims jwt.MapClaims, duration time.Duration, key string) (time.Time, string, error) {
	expiresAt := time.Now().Add(duration)
	claims[appkit.JwtExpiresAtKey] = jwt.NewNumericDate(expiresAt)
	if s.conf.IssuerUri != "" {
		claims[appkit.JwtIssuerUriKey] = s.conf.IssuerUri
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err := jwtToken.SignedString([]byte(key))
	return expiresAt, token, s.wrapI18nError(err)
}

func (s *jwtAuthService) CreateAccessToken(_ context.Context, claims jwt.MapClaims) (time.Time, string, error) {
	return s.createToken(claims, s.conf.AccessExpiresIn, s.conf.AccessSecretKey)
}

func (s *jwtAuthService) CreateRefreshToken(ctx context.Context, claims jwt.MapClaims, redisID string) (time.Time, string, error) {
	claims[appkit.JwtRefreshTokenKey] = redisID
	expiresAt, token, err := s.createToken(claims, s.conf.RefreshExpiresIn, s.conf.RefreshSecretKey)
	if err != nil {
		return expiresAt, token, err
	}
	redisKey := fmt.Sprintf(appkit.RedisRefreshTokenKey, redisID)
	err = s.cache.Set(ctx, redisKey, nil, s.conf.RefreshExpiresIn).Err()
	return expiresAt, token, err
}

func (s *jwtAuthService) DeleteRefreshToken(ctx context.Context, redisID string) error {
	redisKey := fmt.Sprintf(appkit.RedisRefreshTokenKey, redisID)
	return s.cache.Del(ctx, redisKey).Err()
}

func (s *jwtAuthService) RevokeRefreshToken(ctx context.Context, redisMask string) error {
	pattern := fmt.Sprintf(appkit.RedisRefreshTokenKey, redisMask)
	iter := s.cache.Scan(ctx, 0, pattern, 0).Iterator()

	var keys []string
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if len(keys) > 0 {
		return s.cache.Del(ctx, keys...).Err()
	}

	return iter.Err()
}

func (s *jwtAuthService) parseClaims(str, key string) (jwt.MapClaims, error) {
	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(str, claims, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			//fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			return nil, jwt.ErrSignatureInvalid
		}
		return []byte(key), nil
	})
	if err != nil {
		return nil, s.wrapI18nError(err)
	}
	if !token.Valid {
		return nil, appkit.NewInternalServerError(i18n.ErrTokenInvalid)
	}
	return claims, nil
}

// ParseAccessClaims parses the custom claims from the request token.
func (s *jwtAuthService) ParseAccessClaims(_ context.Context, str string) (jwt.MapClaims, error) {
	return s.parseClaims(str, s.conf.AccessSecretKey)
}

// ParseRefreshClaims parses the custom claims from the request token.
func (s *jwtAuthService) ParseRefreshClaims(ctx context.Context, str string) (jwt.MapClaims, error) {
	claims, err := s.parseClaims(str, s.conf.RefreshSecretKey)
	if err != nil {
		return nil, err
	}
	redisKey := fmt.Sprintf(appkit.RedisRefreshTokenKey, claims[appkit.JwtRefreshTokenKey])
	if err = s.cache.Get(ctx, redisKey).Err(); err != nil {
		return nil, appkit.NewUnauthorized(i18n.ErrTokenExpired)
	}
	return claims, nil
}
