package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/store"
)

type oauthState struct {
	ProviderID uint64 `json:"providerId"`
	State      string `json:"state"`
	Verifier   string `json:"verifier"`
	ReturnPath string `json:"returnPath"`
	ExpiresAt  int64  `json:"expiresAt"`
}

type oauthExchangeRequest struct {
	Code string `json:"code"`
}

func publicAuthProvidersHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := ctx.Auth.PublicAuthProviders(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.oauth_methods_load_failed")
			return
		}
		ok(w, r, items)
	}
}

func startOAuthHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerID, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.oauth_provider_id_invalid")
			return
		}
		provider, err := ctx.Auth.ActiveAuthProvider(r.Context(), providerID)
		if err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.oauth_provider_unavailable")
			return
		}
		returnPath := safeReturnPath(r.URL.Query().Get("returnPath"))
		state := uuid.NewString()
		verifier := strings.ReplaceAll(uuid.NewString()+uuid.NewString(), "-", "")
		challengeHash := sha256.Sum256([]byte(verifier))
		challenge := base64.RawURLEncoding.EncodeToString(challengeHash[:])
		statePayload := oauthState{
			ProviderID: providerID, State: state, Verifier: verifier,
			ReturnPath: returnPath, ExpiresAt: time.Now().Add(10 * time.Minute).Unix(),
		}
		cookieValue, err := encodeOAuthState(ctx, statePayload)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.oauth_state_create_failed")
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name: oauthStateCookieName(providerID), Value: cookieValue,
			Path: fmt.Sprintf("/api/auth/oauth/%d/callback", providerID), MaxAge: 600,
			HttpOnly: true, Secure: ctx.Config.CookieSecure, SameSite: http.SameSiteLaxMode,
		})
		authorizationURL, err := url.Parse(provider.AuthorizationURL)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.oauth_authorize_url_invalid")
			return
		}
		query := authorizationURL.Query()
		query.Set("response_type", "code")
		query.Set("client_id", provider.ClientID)
		query.Set("redirect_uri", provider.CallbackURL)
		query.Set("scope", provider.Scopes)
		query.Set("state", state)
		query.Set("code_challenge", challenge)
		query.Set("code_challenge_method", "S256")
		authorizationURL.RawQuery = query.Encode()
		ok(w, r, map[string]string{"authorizationUrl": authorizationURL.String()})
	}
}

func oauthCallbackHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		providerID, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.oauth_provider_id_invalid")
			return
		}
		state, err := readOAuthState(ctx, r, providerID)
		clearOAuthStateCookie(w, ctx, providerID)
		if err != nil || state.State != r.URL.Query().Get("state") || time.Now().Unix() > state.ExpiresAt {
			fail(w, r, http.StatusUnauthorized, 40110, "errors.oauth_state_invalid")
			return
		}
		if strings.TrimSpace(r.URL.Query().Get("error")) != "" {
			fail(w, r, http.StatusUnauthorized, 40111, "errors.oauth_authorize_failed")
			return
		}
		code := strings.TrimSpace(r.URL.Query().Get("code"))
		if code == "" {
			fail(w, r, http.StatusBadRequest, 40002, "errors.oauth_code_required")
			return
		}
		provider, err := ctx.Auth.ActiveAuthProvider(r.Context(), providerID)
		if err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.oauth_provider_unavailable")
			return
		}
		clientSecret, err := ctx.Secrets.Decrypt(provider.ClientSecret)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.oauth_secret_decrypt_failed")
			return
		}
		accessToken, err := exchangeOAuthCode(r.Context(), provider, clientSecret, code, state.Verifier)
		if err != nil {
			fail(w, r, http.StatusBadGateway, 50210, "errors.oauth_token_exchange_failed")
			return
		}
		profile, err := fetchExternalProfile(r.Context(), provider.UserInfoURL, accessToken)
		if err != nil {
			fail(w, r, http.StatusBadGateway, 50211, "errors.oauth_userinfo_failed")
			return
		}
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(uuid.NewString()+uuid.NewString()), bcrypt.DefaultCost)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.oauth_identity_init_failed")
			return
		}
		user, err := ctx.Auth.ResolveExternalUser(r.Context(), provider, profile, string(passwordHash))
		if err != nil || user.Status != 1 {
			fail(w, r, http.StatusForbidden, 40310, "errors.oauth_not_bound")
			return
		}
		pair, err := ctx.Tokens.Issue(user.ID, user.Username, user.TokenVersion)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.token_issue_failed")
			return
		}
		loginMethod := "oauth:" + provider.ProviderType
		if err := ctx.Auth.SaveExternalRefreshToken(r.Context(), user.ID, pair.RefreshHash,
			pair.RefreshFamilyID, loginMethod, clientIP(r), r.UserAgent(), pair.RefreshExpiresAt); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.session_create_failed")
			return
		}
		_ = ctx.Auth.RecordOAuthLogin(r.Context(), user.ID, user.Username, loginMethod, clientIP(r), r.UserAgent())
		exchangeCode := uuid.NewString()
		payload, _ := json.Marshal(pairResponse(ctx, pair))
		if err := ctx.Cache.SetexCtx(r.Context(), "oauth:exchange:"+exchangeCode, string(payload), 60); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.oauth_result_store_failed")
			return
		}
		writeRefreshCookie(w, ctx, pair.RefreshToken)
		redirectURL := addQueryValue(state.ReturnPath, "code", exchangeCode)
		http.Redirect(w, r, redirectURL, http.StatusFound)
	}
}

func exchangeOAuthHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req oauthExchangeRequest
		if !decode(w, r, &req) {
			return
		}
		if strings.TrimSpace(req.Code) == "" {
			fail(w, r, http.StatusBadRequest, 40001, "errors.oauth_exchange_code_required")
			return
		}
		payload, err := ctx.Cache.GetDelCtx(r.Context(), "oauth:exchange:"+strings.TrimSpace(req.Code))
		if err != nil || payload == "" {
			fail(w, r, http.StatusUnauthorized, 40112, "errors.oauth_exchange_invalid")
			return
		}
		var response tokenResponse
		if json.Unmarshal([]byte(payload), &response) != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.oauth_result_read_failed")
			return
		}
		ok(w, r, response)
	}
}

func exchangeOAuthCode(ctx context.Context, provider store.AuthProvider, secret, code, verifier string) (string, error) {
	form := url.Values{
		"grant_type": {"authorization_code"}, "code": {code},
		"redirect_uri": {provider.CallbackURL}, "client_id": {provider.ClientID},
		"client_secret": {secret}, "code_verifier": {verifier},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, provider.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("Accept", "application/json")
	response, err := oauthHTTPClient().Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", errors.New("oauth token endpoint rejected request")
	}
	var payload struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(body, &payload) == nil && payload.AccessToken != "" {
		return payload.AccessToken, nil
	}
	values, err := url.ParseQuery(string(body))
	if err != nil || values.Get("access_token") == "" {
		return "", errors.New("oauth access token is missing")
	}
	return values.Get("access_token"), nil
}

func fetchExternalProfile(ctx context.Context, userInfoURL, accessToken string) (store.ExternalProfile, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, userInfoURL, nil)
	if err != nil {
		return store.ExternalProfile{}, err
	}
	request.Header.Set("Authorization", "Bearer "+accessToken)
	request.Header.Set("Accept", "application/json")
	response, err := oauthHTTPClient().Do(request)
	if err != nil {
		return store.ExternalProfile{}, err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if err != nil || response.StatusCode < 200 || response.StatusCode >= 300 {
		return store.ExternalProfile{}, errors.New("oauth userinfo endpoint rejected request")
	}
	var values map[string]any
	if json.Unmarshal(body, &values) != nil {
		return store.ExternalProfile{}, errors.New("oauth userinfo is not JSON")
	}
	externalID := firstString(values, "sub", "id", "user_id")
	if externalID == "" {
		return store.ExternalProfile{}, errors.New("oauth userinfo has no stable identifier")
	}
	email := firstString(values, "email")
	if verified, exists := values["email_verified"]; exists {
		if valid, isBool := verified.(bool); isBool && !valid {
			email = ""
		}
	}
	raw := string(body)
	if len(raw) > 32*1024 {
		raw = raw[:32*1024]
	}
	return store.ExternalProfile{
		ExternalUserID: externalID, Username: firstString(values, "preferred_username", "login", "username"),
		DisplayName: firstString(values, "name", "display_name"), Email: email,
		Mobile: firstString(values, "phone_number", "mobile"), Avatar: firstString(values, "picture", "avatar_url"),
		RawJSON: raw,
	}, nil
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, exists := values[key]; exists && value != nil {
			switch typed := value.(type) {
			case string:
				if strings.TrimSpace(typed) != "" {
					return strings.TrimSpace(typed)
				}
			case float64:
				return strconv.FormatInt(int64(typed), 10)
			}
		}
	}
	return ""
}

func oauthHTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
	}}
}

func encodeOAuthState(ctx *app.Context, state oauthState) (string, error) {
	payload, err := json.Marshal(state)
	if err != nil {
		return "", err
	}
	encrypted, err := ctx.Secrets.Encrypt(string(payload))
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString([]byte(encrypted)), nil
}

func readOAuthState(ctx *app.Context, r *http.Request, providerID uint64) (oauthState, error) {
	cookie, err := r.Cookie(oauthStateCookieName(providerID))
	if err != nil {
		return oauthState{}, err
	}
	encrypted, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return oauthState{}, err
	}
	payload, err := ctx.Secrets.Decrypt(string(encrypted))
	if err != nil {
		return oauthState{}, err
	}
	var state oauthState
	if err := json.Unmarshal([]byte(payload), &state); err != nil || state.ProviderID != providerID {
		return oauthState{}, errors.New("oauth state is invalid")
	}
	return state, nil
}

func clearOAuthStateCookie(w http.ResponseWriter, ctx *app.Context, providerID uint64) {
	http.SetCookie(w, &http.Cookie{
		Name: oauthStateCookieName(providerID), Value: "", Path: fmt.Sprintf("/api/auth/oauth/%d/callback", providerID),
		MaxAge: -1, Expires: time.Unix(1, 0), HttpOnly: true,
		Secure: ctx.Config.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func oauthStateCookieName(providerID uint64) string {
	return fmt.Sprintf("ua_oauth_state_%d", providerID)
}

func safeReturnPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || !strings.HasPrefix(value, "/") || strings.HasPrefix(value, "//") {
		return "/auth/oauth-callback"
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return "/auth/oauth-callback"
	}
	return value
}

func addQueryValue(rawURL, key, value string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "/auth/oauth-callback?code=" + url.QueryEscape(value)
	}
	query := parsed.Query()
	query.Set(key, value)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
