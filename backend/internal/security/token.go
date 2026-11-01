// Package security holds the cryptographic primitives: token minting and
// validation, secret encryption, and private-key parsing.
package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// TokenManager issues and validates access tokens. Access tokens are signed JWTs; refresh tokens are opaque random strings whose hash is the only thing stored.
type TokenManager struct {
	secret        []byte
	issuer        string
	accessExpire  time.Duration
	refreshExpire time.Duration
}

// AccessClaims is the payload of an access token. TokenVersion is compared against the database so a forced logout invalidates tokens that have not expired yet.
type AccessClaims struct {
	UserID       uint64 `json:"uid"`
	Username     string `json:"username"`
	TokenVersion int64  `json:"tv"`
	jwt.RegisteredClaims
}

// Pair is a freshly minted access/refresh pair together with the values the caller has to persist.
type Pair struct {
	AccessToken      string
	RefreshToken     string
	RefreshHash      string
	RefreshFamilyID  string
	AccessExpiresAt  time.Time
	RefreshExpiresAt time.Time
}

// NewTokenManager builds a manager from the configured secret, issuer and lifetimes.
func NewTokenManager(secret, issuer string, accessExpireSeconds, refreshExpireSeconds int64) *TokenManager {
	return &TokenManager{
		secret:        []byte(secret),
		issuer:        issuer,
		accessExpire:  time.Duration(accessExpireSeconds) * time.Second,
		refreshExpire: time.Duration(refreshExpireSeconds) * time.Second,
	}
}

// Issue mints an access token and a refresh token. The refresh token is returned in clear exactly once; only its hash is meant to be stored.
func (m *TokenManager) Issue(userID uint64, username string, tokenVersion int64) (Pair, error) {
	now := time.Now()
	accessExpiresAt := now.Add(m.accessExpire)
	claims := AccessClaims{
		UserID:       userID,
		Username:     username,
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(accessExpiresAt),
			ID:        uuid.NewString(),
		},
	}
	accessToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return Pair{}, err
	}

	refreshToken, err := randomToken(48)
	if err != nil {
		return Pair{}, err
	}
	return Pair{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		RefreshHash:      HashOpaqueToken(refreshToken),
		RefreshFamilyID:  uuid.NewString(),
		AccessExpiresAt:  accessExpiresAt,
		RefreshExpiresAt: now.Add(m.refreshExpire),
	}, nil
}

// ValidateAccess parses and verifies an access token, rejecting a wrong issuer or a signing method other than HS256.
func (m *TokenManager) ValidateAccess(value string) (AccessClaims, error) {
	claims := AccessClaims{}
	token, err := jwt.ParseWithClaims(value, &claims, func(token *jwt.Token) (any, error) {
		return m.secret, nil
	}, jwt.WithIssuer(m.issuer), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !token.Valid {
		return AccessClaims{}, fmt.Errorf("invalid access token: %w", err)
	}
	return claims, nil
}

// NewAPIToken mints a personal access token. The clear value is returned once and only its hash is stored.
func NewAPIToken() (string, string, error) {
	random, err := randomToken(36)
	if err != nil {
		return "", "", err
	}
	value := "ua_" + random
	return value, HashOpaqueToken(value), nil
}

// HashOpaqueToken is the one-way hash used for every opaque token, so a database leak does not leak usable tokens.
func HashOpaqueToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func randomToken(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
