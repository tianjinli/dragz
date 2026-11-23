// Package store is the data-access layer. Business SQL lives in queries/*.sql
// and is generated into internal/store/sqlc; the files here map those rows
// onto the domain types the rest of the application uses.
package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"dragz-admin/backend/internal/pkg/ctxkey"
	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// AuthRepository is the PostgreSQL implementation of the authentication, authorization and account repositories. Every query it exposes comes from queries/*.sql via sqlc.
type AuthRepository struct {
	db         *DB
	platformDB *DB
	queries    *sqlcstore.Queries
}

// SetPlatformDB installs the cross-tenant identity for policy loading.
func (r *AuthRepository) SetPlatformDB(db *DB) {
	if db != nil {
		r.platformDB = db
	}
}

func (r *AuthRepository) platformHandle() *DB {
	if r != nil && r.platformDB != nil {
		return r.platformDB
	}
	if r == nil {
		return nil
	}
	return r.db
}

// User is the credential record used during login. It is never serialised to a client: the admin API exposes UserDetail instead.
type User struct {
	ID                 uint64
	DepartmentID       sql.NullInt64
	Username           string
	PasswordHash       string
	DisplayName        string
	Email              string
	Mobile             string
	Status             int8
	IsSuperAdmin       bool
	MustChangePassword bool
	FailedLoginCount   int
	LockedUntil        sql.NullTime
	TokenVersion       int64
	AvatarFileID       sql.NullInt64
}

// RefreshSession is one refresh-token row, resolved by the hash of the token the client presented.
// Tenant lives on the access token (tid), not on the refresh row.
type RefreshSession struct {
	ID     uint64
	UserID uint64
	// TenantID is the tenant the session was opened in. Zero means the token
	// predates migration 017 and the caller must re-derive the tenant.
	TenantID  sql.NullInt64
	Username  string
	TokenHash string
	FamilyID  string
	ExpiresAt time.Time
	RevokedAt sql.NullTime
}

// Menu is a sidebar node. Permission buttons never reach it: both menu queries filter type <> 'button'.
type Menu struct {
	ID          uint64
	ParentID    uint64
	Type        string
	Name        string
	Code        string
	RouteName   string
	RoutePath   string
	Component   string
	Redirect    string
	Icon        string
	Sort        int
	Visible     bool
	KeepAlive   bool
	AffixTab    bool
	ExternalURL string
}

// NewAuthRepository binds the generated queries to a database handle.
func NewAuthRepository(db *DB) *AuthRepository {
	return &AuthRepository{db: db, queries: sqlcstore.New(newSessionDBTX(db))}
}

// CountUsers reports how many active users exist. Bootstrap uses it to decide whether the seeded administrator still has to be created.
func (r *AuthRepository) CountUsers() (int64, error) {
	ctx := ctxkey.WithRLSBypass(context.Background(), true)
	return r.queries.CountActiveUsers(ctx)
}

// CreateBootstrapAdmin inserts the first administrator and grants it role 1 in one transaction, so a half-created admin cannot exist.
func (r *AuthRepository) CreateBootstrapAdmin(username, passwordHash string) error {
	// The bootstrap administrator belongs to the default tenant, which
	// migrations/013 creates with id 1. The tenant is set explicitly rather
	// than left to a trigger default: app_fill_tenant_id refuses to guess.
	ctx := ctxkey.WithTenantID(ctxkey.WithRLSBypass(context.Background(), true), 1)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	userID, err := q.CreateBootstrapAdmin(ctx, sqlcstore.CreateBootstrapAdminParams{
		Username: username, PasswordHash: passwordHash,
	})
	if err != nil {
		return err
	}
	if err := q.InsertUserRole(ctx, sqlcstore.InsertUserRoleParams{UserID: uint64(userID), RoleID: 1}); err != nil {
		return err
	}
	// The account is global; membership is what makes it visible in tenant 1.
	// The bootstrap administrator administers that tenant and is the platform
	// operator, which is what the default deployment is.
	if err := q.UpsertTenantMember(ctx, sqlcstore.UpsertTenantMemberParams{
		TenantID: 1, UserID: uint64(userID), IsTenantAdmin: 1, IsPlatformOperator: true, Status: 1,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// FindUserByUsername loads the credentials of one account. An unknown name surfaces as sql.ErrNoRows, which the login flow treats as "invalid credentials".
func (r *AuthRepository) FindUserByUsername(ctx context.Context, username string) (User, error) {
	row, err := r.queries.GetUserByUsername(ctx, username)
	if err != nil {
		return User{}, err
	}
	return mapAuthUserByUsername(row), nil
}

// FindUserByID loads the credentials of one account by primary key.
func (r *AuthRepository) FindUserByID(ctx context.Context, userID uint64) (User, error) {
	row, err := r.queries.GetUserByID(ctx, int64(userID))
	if err != nil {
		return User{}, err
	}
	return mapAuthUser(row), nil
}

// UpdateSelfProfile writes the fields a user may change about themselves, including the avatar file id.
func (r *AuthRepository) UpdateSelfProfile(ctx context.Context, userID uint64, displayName, email, mobile string, avatarFileID sql.NullInt64) error {
	affected, err := r.queries.UpdateSelfProfile(ctx, sqlcstore.UpdateSelfProfileParams{
		DisplayName: displayName, Email: email, Mobile: mobile,
		AvatarFileID: avatarFileID, ID: int64(userID),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// ChangeSelfPassword replaces the password hash and revokes every refresh token of the user, so other devices are logged out.
func (r *AuthRepository) ChangeSelfPassword(ctx context.Context, userID uint64, passwordHash string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	affected, err := q.ChangeSelfPassword(ctx, sqlcstore.ChangeSelfPasswordParams{
		PasswordHash: passwordHash, ID: int64(userID),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	if err := q.RevokeUserRefreshTokens(ctx, sqlcstore.RevokeUserRefreshTokensParams{
		RevokeReason: "password changed", UserID: userID,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// RecordLoginFailure increments the failure counter (and optionally sets a lock) and writes the failure to the login log in one transaction.
func (r *AuthRepository) RecordLoginFailure(ctx context.Context, username, ip, userAgent, reason string, lockUntil *time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	if lockUntil == nil {
		err = q.IncrementFailedLogin(ctx, username)
	} else {
		err = q.IncrementFailedLoginWithLock(ctx, sqlcstore.IncrementFailedLoginWithLockParams{
			LockedUntil: sql.NullTime{Time: *lockUntil, Valid: true},
			Username:    username,
		})
	}
	if err != nil {
		return err
	}
	if err := q.InsertLoginLogFailure(ctx, sqlcstore.InsertLoginLogFailureParams{
		Username: username, FailureReason: reason, Ip: ip, UserAgent: userAgent,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// ClearLoginFailures resets the failed-login counter and lock window after a successful unlock.
func (r *AuthRepository) ClearLoginFailures(ctx context.Context, userID uint64) error {
	return r.queries.ClearFailedLogin(ctx, int64(userID))
}

// RecordLoginSuccess stamps the last login and appends the success entry to the login log in one transaction.
func (r *AuthRepository) RecordLoginSuccess(ctx context.Context, userID uint64, username, ip, userAgent string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	if err := q.RecordLoginSuccessUser(ctx, sqlcstore.RecordLoginSuccessUserParams{
		LastLoginIp: ip, ID: int64(userID),
	}); err != nil {
		return err
	}
	if err := q.InsertLoginLogSuccess(ctx, sqlcstore.InsertLoginLogSuccessParams{
		UserID:   sql.NullInt64{Int64: int64(userID), Valid: true},
		Username: username, Ip: ip, UserAgent: userAgent,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// SaveRefreshToken stores a refresh token issued by an interactive login.
// tenantID is recorded so a later refresh stays in the tenant the user logged
// into instead of falling back to their first membership.
func (r *AuthRepository) SaveRefreshToken(ctx context.Context, userID, tenantID uint64, tokenHash, familyID, deviceID, deviceName, ip, userAgent string, expiresAt time.Time) error {
	return r.queries.InsertRefreshToken(ctx, sqlcstore.InsertRefreshTokenParams{
		UserID: userID, TenantID: optionalUint64Null(tenantID), TokenHash: tokenHash, FamilyID: familyID,
		DeviceID: deviceID, DeviceName: deviceName, Ip: ip, UserAgent: userAgent, ExpiresAt: expiresAt,
	})
}

// SaveExternalRefreshToken stores a refresh token issued through an external identity provider, recording which method was used.
func (r *AuthRepository) SaveExternalRefreshToken(ctx context.Context, userID, tenantID uint64, tokenHash, familyID, loginMethod, ip, userAgent string, expiresAt time.Time) error {
	return r.queries.InsertExternalRefreshToken(ctx, sqlcstore.InsertExternalRefreshTokenParams{
		UserID: userID, TenantID: optionalUint64Null(tenantID), TokenHash: tokenHash, FamilyID: familyID,
		LoginMethod: loginMethod, Ip: ip, UserAgent: userAgent, ExpiresAt: expiresAt,
	})
}

// FindRefreshSession resolves a refresh token by hash.
func (r *AuthRepository) FindRefreshSession(ctx context.Context, tokenHash string) (RefreshSession, error) {
	row, err := r.queries.FindRefreshSession(ctx, tokenHash)
	if err != nil {
		return RefreshSession{}, err
	}
	return RefreshSession{
		ID: uint64(row.ID), UserID: row.UserID, TenantID: row.TenantID, Username: row.Username,
		TokenHash: row.TokenHash, FamilyID: row.FamilyID, ExpiresAt: row.ExpiresAt, RevokedAt: row.RevokedAt,
	}, nil
}

// RotateRefreshToken marks the old token used and inserts its replacement in one transaction. If the old row was already rotated, ErrTokenReused is returned and the caller must revoke the whole family.
func (r *AuthRepository) RotateRefreshToken(ctx context.Context, oldID uint64, userID, tenantID uint64, tokenHash, familyID, deviceID, deviceName, ip, userAgent string, expiresAt time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	affected, err := q.MarkRefreshTokenRotated(ctx, int64(oldID))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrTokenReused
	}
	if err := q.InsertRefreshToken(ctx, sqlcstore.InsertRefreshTokenParams{
		UserID: userID, TenantID: optionalUint64Null(tenantID), TokenHash: tokenHash, FamilyID: familyID,
		DeviceID: deviceID, DeviceName: deviceName, Ip: ip, UserAgent: userAgent, ExpiresAt: expiresAt,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// RevokeRefreshToken revokes one refresh token by hash. Same as RevokeSession:
// the user's token version moves so that login's access token is rejected
// immediately. Missing or already-revoked tokens are treated as success.
func (r *AuthRepository) RevokeRefreshToken(ctx context.Context, tokenHash, reason string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	userID, err := q.RevokeRefreshTokenByHash(ctx, sqlcstore.RevokeRefreshTokenByHashParams{
		RevokeReason: reason, TokenHash: tokenHash,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return err
	}
	if err := q.BumpUserTokenVersion(ctx, int64(userID)); err != nil {
		return err
	}
	return tx.Commit()
}

// RevokeRefreshFamily revokes every token in a rotation family, which is how a detected replay is contained.
func (r *AuthRepository) RevokeRefreshFamily(ctx context.Context, familyID, reason string) error {
	return r.queries.RevokeRefreshFamily(ctx, sqlcstore.RevokeRefreshFamilyParams{
		RevokeReason: reason, FamilyID: familyID,
	})
}

// RolePermissionPolicy is one Casbin policy row: a role, its tenant domain and a permission code it holds.
type RolePermissionPolicy struct {
	RoleID         uint64
	TenantID       uint64
	PermissionCode string
}

// UserRoleIDs returns the IDs of the user's enabled, non-deleted roles in one tenant.
func (r *AuthRepository) UserRoleIDs(ctx context.Context, userID, tenantID uint64) ([]uint64, error) {
	return r.queries.ListUserRoleIDsActive(ctx, sqlcstore.ListUserRoleIDsActiveParams{
		UserID: userID, TenantID: tenantID,
	})
}

// UserRole is one role the account holds. The code addresses the preset
// translation (role.<code>); the name is the source-language text stored on the
// row, used as the fallback when no translation exists.
type UserRole struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// requestTenantID is the tenant stored on ctx. A missing value is 0, so a
// request with no tenant matches no tenant-scoped role grant.
func requestTenantID(ctx context.Context) uint64 {
	id, ok := ctxkey.TenantIDFrom(ctx)
	if !ok {
		return 0
	}
	return id
}

// UserRoles returns the user's enabled roles, in display order, for the tenant on ctx.
func (r *AuthRepository) UserRoles(ctx context.Context, userID uint64) ([]UserRole, error) {
	rows, err := r.queries.ListUserRoles(ctx, sqlcstore.ListUserRolesParams{
		UserID: userID, TenantID: requestTenantID(ctx),
	})
	if err != nil {
		return nil, err
	}
	roles := make([]UserRole, 0, len(rows))
	for _, row := range rows {
		roles = append(roles, UserRole{Code: row.Code, Name: row.Name})
	}
	return roles, nil
}

// DepartmentRef resolves a nullable department reference to its code and name,
// returning an empty ref when the user has no department. The code keys the
// translation record ("dept.<code>"); the name is the source-language text.
func (r *AuthRepository) DepartmentRef(ctx context.Context, departmentID sql.NullInt64) (NamedRef, error) {
	if !departmentID.Valid {
		return NamedRef{}, nil
	}
	row, err := r.queries.GetDepartmentNameByID(ctx, departmentID.Int64)
	if errors.Is(err, sql.ErrNoRows) {
		return NamedRef{}, nil
	}
	if err != nil {
		return NamedRef{}, err
	}
	return NamedRef{Code: row.Code, Name: row.Name}, nil
}

// PermissionCodes returns the button codes granted to the user inside the tenant on ctx.
// A super administrator short-circuits to the wildcard.
func (r *AuthRepository) PermissionCodes(ctx context.Context, userID uint64, superAdmin bool) ([]string, error) {
	if superAdmin {
		return []string{"*"}, nil
	}
	return r.queries.ListUserPermissionCodes(ctx, sqlcstore.ListUserPermissionCodesParams{
		UserID: userID, TenantID: requestTenantID(ctx),
	})
}

// Menus returns the sidebar tree source rows for one user, or every visible menu for a super administrator. Pruning of disabled plugins happens in SQL.
func (r *AuthRepository) Menus(ctx context.Context, userID uint64, superAdmin bool) ([]Menu, error) {
	var menus []Menu
	if superAdmin {
		rows, err := r.queries.ListAllVisibleMenus(ctx)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			menus = append(menus, Menu{
				ID: uint64(row.ID), ParentID: row.ParentID, Type: row.Type, Name: row.Name, Code: row.Code,
				RouteName: row.RouteName, RoutePath: row.RoutePath, Component: row.Component, Redirect: row.Redirect,
				Icon: row.Icon, Sort: int(row.Sort),
				Visible: smallIntBool(row.Visible), KeepAlive: smallIntBool(row.KeepAlive),
				AffixTab: smallIntBool(row.AffixTab), ExternalURL: row.ExternalUrl,
			})
		}
		return menus, nil
	}
	rows, err := r.queries.ListUserVisibleMenus(ctx, sqlcstore.ListUserVisibleMenusParams{
		UserID: userID, TenantID: requestTenantID(ctx),
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		menus = append(menus, Menu{
			ID: uint64(row.ID), ParentID: row.ParentID, Type: row.Type, Name: row.Name, Code: row.Code,
			RouteName: row.RouteName, RoutePath: row.RoutePath, Component: row.Component, Redirect: row.Redirect,
			Icon: row.Icon, Sort: int(row.Sort),
			Visible: smallIntBool(row.Visible), KeepAlive: smallIntBool(row.KeepAlive),
			AffixTab: smallIntBool(row.AffixTab), ExternalURL: row.ExternalUrl,
		})
	}
	if menus == nil {
		menus = []Menu{}
	}
	return menus, nil
}
