package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/sqlc-dev/pqtype"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// PublicAuthProvider is the unauthenticated projection of a provider: the login page needs the name and icon, never the secret.
type PublicAuthProvider struct {
	ID           uint64 `json:"id"`
	ProviderType string `json:"providerType"`
	Name         string `json:"name"`
	Icon         string `json:"icon"`
	Sort         int    `json:"sort"`
}

// ExternalProfile is the normalised user info returned by an external provider.
type ExternalProfile struct {
	ExternalUserID string
	Username       string
	DisplayName    string
	Email          string
	Mobile         string
	Avatar         string
	RawJSON        string
}

// PublicAuthProviders returns the enabled providers for the login page.
func (r *AuthRepository) PublicAuthProviders(ctx context.Context) ([]PublicAuthProvider, error) {
	rows, err := r.queries.ListPublicAuthProviders(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]PublicAuthProvider, 0, len(rows))
	for _, row := range rows {
		items = append(items, PublicAuthProvider{
			ID: uint64(row.ID), ProviderType: row.ProviderType, Name: row.Name, Icon: row.Icon, Sort: int(row.Sort),
		})
	}
	return items, nil
}

// ActiveAuthProvider loads an enabled provider including its secret, which the token exchange needs.
func (r *AuthRepository) ActiveAuthProvider(ctx context.Context, providerID uint64) (AuthProvider, error) {
	row, err := r.queries.GetActiveAuthProvider(ctx, int64(providerID))
	if err != nil {
		return AuthProvider{}, err
	}
	return mapAuthProviderRow(row.ID, row.ProviderType, row.Name, row.Icon, row.ClientID, row.ClientSecretEncrypted,
		row.AuthorizationUrl, row.TokenUrl, row.UserInfoUrl, row.IssuerUrl, row.CallbackUrl, row.Scopes,
		row.AutoRegister, row.DefaultRoleID, row.BindingRule, row.Status, row.Sort, row.UpdatedAt), nil
}

func profileJSON(raw string) pqtype.NullRawMessage {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return pqtype.NullRawMessage{}
	}
	if !json.Valid([]byte(raw)) {
		b, _ := json.Marshal(raw)
		return pqtype.NullRawMessage{RawMessage: b, Valid: true}
	}
	return pqtype.NullRawMessage{RawMessage: json.RawMessage(raw), Valid: true}
}

// ResolveExternalUser maps an external identity to a local user, creating or refreshing the identity row in one transaction. The binding rule decides whether an existing account is matched by email or mobile; without a match the provider's auto-register flag decides between creating a user and failing with ErrOAuthBindingRequired.
func (r *AuthRepository) ResolveExternalUser(ctx context.Context, provider AuthProvider, profile ExternalProfile, passwordHash string) (User, error) {
	if strings.TrimSpace(profile.ExternalUserID) == "" {
		return User{}, ErrOAuthSubjectMissing
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	userID, err := q.GetUserIdentityUserID(ctx, sqlcstore.GetUserIdentityUserIDParams{
		ProviderID: provider.ID, ExternalUserID: profile.ExternalUserID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		uid, resolveErr := resolveBindingUser(ctx, q, provider, profile, passwordHash)
		if resolveErr != nil {
			return User{}, resolveErr
		}
		userID = uid
		if err := q.InsertUserIdentity(ctx, sqlcstore.InsertUserIdentityParams{
			UserID: uid, ProviderID: provider.ID, ExternalUserID: profile.ExternalUserID,
			ExternalUsername: profile.Username, ExternalEmail: profile.Email, ProfileJson: profileJSON(profile.RawJSON),
		}); err != nil {
			return User{}, err
		}
	} else if err != nil {
		return User{}, err
	} else {
		if err := q.UpdateUserIdentity(ctx, sqlcstore.UpdateUserIdentityParams{
			ExternalUsername: profile.Username, ExternalEmail: profile.Email,
			ProfileJson: profileJSON(profile.RawJSON), ProviderID: provider.ID, ExternalUserID: profile.ExternalUserID,
		}); err != nil {
			return User{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}
	return r.FindUserByID(ctx, userID)
}

func resolveBindingUser(ctx context.Context, q *sqlcstore.Queries, provider AuthProvider, profile ExternalProfile, passwordHash string) (uint64, error) {
	var userID int64
	var err error
	switch provider.BindingRule {
	case "email":
		if profile.Email != "" {
			userID, err = q.FindUserIDByEmail(ctx, profile.Email)
			if err == nil {
				return uint64(userID), nil
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return 0, err
			}
		}
	case "mobile":
		if profile.Mobile != "" {
			userID, err = q.FindUserIDByMobile(ctx, profile.Mobile)
			if err == nil {
				return uint64(userID), nil
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return 0, err
			}
		}
	}
	if !provider.AutoRegister {
		return 0, ErrOAuthBindingRequired
	}
	username := fmt.Sprintf("oauth_%d_%s", provider.ID, strings.ReplaceAll(profile.ExternalUserID, "-", ""))
	if len(username) > 56 {
		username = username[:56]
	}
	displayName := strings.TrimSpace(profile.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(profile.Username)
	}
	if displayName == "" {
		displayName = "OAuth User"
	}
	userID, err = q.CreateOAuthUser(ctx, sqlcstore.CreateOAuthUserParams{
		Username: username, PasswordHash: passwordHash, DisplayName: displayName,
		Email: profile.Email, Mobile: profile.Mobile,
	})
	if err != nil {
		return 0, err
	}
	if provider.DefaultRoleID != nil {
		if err := q.InsertUserRoleIfActive(ctx, sqlcstore.InsertUserRoleIfActiveParams{
			UserID: uint64(userID), ID: int64(*provider.DefaultRoleID),
		}); err != nil {
			return 0, err
		}
	}
	return uint64(userID), nil
}

// RecordOAuthLogin clears the failure counter and appends the login log entry for an external login.
func (r *AuthRepository) RecordOAuthLogin(ctx context.Context, userID uint64, username, method, ip, userAgent string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	if err := q.ClearLoginState(ctx, sqlcstore.ClearLoginStateParams{LastLoginIp: ip, ID: int64(userID)}); err != nil {
		return err
	}
	if err := q.InsertLoginLogOAuth(ctx, sqlcstore.InsertLoginLogOAuthParams{
		UserID:   sql.NullInt64{Int64: int64(userID), Valid: true},
		Username: username, LoginMethod: method, Ip: ip, UserAgent: userAgent,
	}); err != nil {
		return err
	}
	return tx.Commit()
}
