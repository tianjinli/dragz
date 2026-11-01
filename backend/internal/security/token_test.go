package security

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestIssueCreatesValidAccessAndOpaqueRefreshTokens(t *testing.T) {
	manager := NewTokenManager("01234567890123456789012345678901", "dragz-admin-test", 60, 120)
	pair, err := manager.Issue(42, "alice", 3)
	if err != nil {
		t.Fatalf("Issue() error = %v", err)
	}
	if pair.AccessToken == "" || pair.RefreshToken == "" || pair.RefreshHash == "" {
		t.Fatal("Issue() returned an empty token")
	}
	if pair.RefreshToken == pair.RefreshHash {
		t.Fatal("refresh token must not be stored in plaintext form")
	}
	if pair.AccessExpiresAt.Before(time.Now()) || pair.RefreshExpiresAt.Before(pair.AccessExpiresAt) {
		t.Fatal("token expiry timestamps are invalid")
	}

	claims := &AccessClaims{}
	parsed, err := jwt.ParseWithClaims(pair.AccessToken, claims, func(token *jwt.Token) (any, error) {
		return []byte("01234567890123456789012345678901"), nil
	}, jwt.WithIssuer("dragz-admin-test"), jwt.WithValidMethods([]string{"HS256"}))
	if err != nil || !parsed.Valid {
		t.Fatalf("issued access token is invalid: %v", err)
	}
	if claims.UserID != 42 || claims.Username != "alice" || claims.TokenVersion != 3 {
		t.Fatalf("unexpected claims: %#v", claims)
	}
}

func TestHashOpaqueTokenIsDeterministicAndNonEmpty(t *testing.T) {
	first := HashOpaqueToken("refresh-token")
	second := HashOpaqueToken("refresh-token")
	if first == "" || first != second {
		t.Fatalf("HashOpaqueToken() got %q and %q", first, second)
	}
	if first == HashOpaqueToken("different-token") {
		t.Fatal("different tokens must not have the same test hash")
	}
}

func TestNewAPITokenUsesExpectedPrefixAndHash(t *testing.T) {
	value, hash, err := NewAPIToken()
	if err != nil {
		t.Fatalf("NewAPIToken() error = %v", err)
	}
	if len(value) < 20 || value[:3] != "ua_" {
		t.Fatalf("unexpected API token %q", value)
	}
	if hash != HashOpaqueToken(value) || hash == value {
		t.Fatal("API token hash is invalid")
	}
}
