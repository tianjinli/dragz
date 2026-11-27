package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"dragz-admin/backend/internal/pkg/ctxkey"
	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// UserListFilter is the query string of the user list. Scope carries the caller's data scope; nil means unrestricted.
type UserListFilter struct {
	Keyword      string
	DepartmentID uint64
	Status       *int8
	CreatedFrom  *time.Time
	CreatedTo    *time.Time
	Page         int
	PageSize     int
	Scope        *DataScope
}

// NamedRef is a preset row a user row points at — its department, one of its
// roles or positions. Code is what the translation record is keyed by
// ("dept.<code>", "role.<code>", "post.<code>"); Name is the source text the
// owning table holds and the fallback when a language has no translation.
type NamedRef struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// UserListItem is one row of the user list.
type UserListItem struct {
	ID                 uint64     `json:"id"`
	DepartmentID       *uint64    `json:"departmentId"`
	DepartmentName     string     `json:"departmentName"`
	Username           string     `json:"username"`
	DisplayName        string     `json:"displayName"`
	Email              string     `json:"email"`
	Mobile             string     `json:"mobile"`
	Gender             int8       `json:"gender"`
	Status             int8       `json:"status"`
	IsSuperAdmin       bool       `json:"isSuperAdmin"`
	MustChangePassword bool       `json:"mustChangePassword"`
	RoleNames          string     `json:"roleNames"`
	PositionNames      string     `json:"positionNames"`
	LastLoginAt        *time.Time `json:"lastLoginAt"`
	LastLoginIP        string     `json:"lastLoginIp"`
	CreatedAt          time.Time  `json:"createdAt"`

	// Preset references, not part of the payload: the handler uses them to
	// render DepartmentName/RoleNames/PositionNames in the request language.
	DepartmentCode string     `json:"-"`
	Roles          []NamedRef `json:"-"`
	Positions      []NamedRef `json:"-"`
}

// UserDetail is a list row plus the fields only the edit form needs.
type UserDetail struct {
	UserListItem
	Remark      string   `json:"remark"`
	RoleIDs     []uint64 `json:"roleIds"`
	PositionIDs []uint64 `json:"positionIds"`
}

// parseNamedRefs reads the JSON array the list query builds for a user's roles
// or positions. A malformed value yields no refs, which leaves the aggregated
// source names in place rather than dropping them.
func parseNamedRefs(raw string) []NamedRef {
	if raw == "" {
		return nil
	}
	var refs []NamedRef
	if json.Unmarshal([]byte(raw), &refs) != nil {
		return nil
	}
	return refs
}

// SaveUserInput is the payload shared by create and update. Update ignores Username and PasswordHash: neither can be changed after creation.
type SaveUserInput struct {
	DepartmentID       *uint64
	Username           string
	PasswordHash       string
	DisplayName        string
	Email              string
	Mobile             string
	Gender             int8
	Status             int8
	MustChangePassword bool
	Remark             string
	RoleIDs            []uint64
	PositionIDs        []uint64
}

// ListUsers returns one page of users plus the total count. The data scope is applied in SQL, not in Go, so pagination stays correct.
func (r *AuthRepository) ListUsers(ctx context.Context, filter UserListFilter) ([]UserListItem, int64, error) {
	filter.Page, filter.PageSize = normalizePage(filter.Page, filter.PageSize)
	scopeAll := filter.Scope == nil || filter.Scope.All
	var selfUserID sql.NullInt64
	departmentIDs := []uint64{}
	if !scopeAll {
		selfUserID = optionalUint64Null(filter.Scope.SelfUserID)
		departmentIDs = uint64SliceOrEmpty(filter.Scope.DepartmentIDs)
	}
	params := sqlcstore.CountUsersFilteredParams{
		TenantID:       tenantFilter(ctx),
		KeywordPattern: optionalPattern(filter.Keyword),
		DepartmentID:   optionalUint64Null(filter.DepartmentID),
		Status:         optionalInt16(filter.Status),
		CreatedFrom:    nullTimeFromPtr(filter.CreatedFrom),
		CreatedTo:      nullTimeFromPtr(filter.CreatedTo),
		ScopeAll:       scopeAll,
		SelfUserID:     selfUserID,
		DepartmentIds:  departmentIDs,
	}
	total, err := r.queries.CountUsersFiltered(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.queries.ListUsersFiltered(ctx, sqlcstore.ListUsersFilteredParams{
		Limit: int32(filter.PageSize), Offset: int32((filter.Page - 1) * filter.PageSize),
		TenantID:       params.TenantID,
		KeywordPattern: params.KeywordPattern, DepartmentID: params.DepartmentID, Status: params.Status,
		CreatedFrom: params.CreatedFrom, CreatedTo: params.CreatedTo,
		ScopeAll: params.ScopeAll, SelfUserID: params.SelfUserID, DepartmentIds: params.DepartmentIds,
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]UserListItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, UserListItem{
			ID: uint64(row.ID), DepartmentID: nullInt64Ptr(row.DepartmentID),
			DepartmentName: row.DepartmentName, Username: row.Username, DisplayName: row.DisplayName,
			Email: row.Email, Mobile: row.Mobile, Gender: int8(row.Gender), Status: int8(row.Status),
			IsSuperAdmin: row.IsSuperAdmin, MustChangePassword: smallIntBool(row.MustChangePassword),
			RoleNames: row.RoleNames, PositionNames: row.PositionNames,
			DepartmentCode: row.DepartmentCode,
			Roles:          parseNamedRefs(row.Roles),
			Positions:      parseNamedRefs(row.Positions),
			LastLoginAt:    nullTimePtr(row.LastLoginAt), LastLoginIP: row.LastLoginIp, CreatedAt: row.CreatedAt,
		})
	}
	return items, total, nil
}

// UserDetail loads one user together with its role and position assignments.
func (r *AuthRepository) UserDetail(ctx context.Context, userID uint64) (UserDetail, error) {
	row, err := r.queries.GetUserDetail(ctx, sqlcstore.GetUserDetailParams{
		ID: int64(userID), TenantID: tenantFilter(ctx),
	})
	if err != nil {
		return UserDetail{}, err
	}
	detail := UserDetail{
		UserListItem: UserListItem{
			ID: uint64(row.ID), DepartmentID: nullInt64Ptr(row.DepartmentID),
			DepartmentName: row.DepartmentName, DepartmentCode: row.DepartmentCode,
			Username: row.Username, DisplayName: row.DisplayName,
			Email: row.Email, Mobile: row.Mobile, Gender: int8(row.Gender), Status: int8(row.Status),
			IsSuperAdmin: row.IsSuperAdmin, MustChangePassword: smallIntBool(row.MustChangePassword),
			LastLoginAt: nullTimePtr(row.LastLoginAt), LastLoginIP: row.LastLoginIp, CreatedAt: row.CreatedAt,
		},
		Remark: row.Remark,
	}
	detail.RoleIDs, err = r.queries.ListUserRoleIDs(ctx, userID)
	if err != nil {
		return UserDetail{}, err
	}
	detail.PositionIDs, err = r.queries.ListUserPositionIDs(ctx, userID)
	return detail, err
}

// CreateUser inserts the account, its role/position assignments, and the
// tenant membership in one transaction.
//
// Seat metering happens inside that transaction: a tenant that meters seats
// cannot create another member once the quota is full, and a later failure in
// the same transaction gives the seat back. Tenants without a seats quota row
// are unlimited.
//
// sys_user is a global account row; sys_tenant_member is what makes the account
// visible inside one tenant. Writing the member row here is what stops a new
// user from being created globally and then belonging to nobody.
func (r *AuthRepository) CreateUser(ctx context.Context, input SaveUserInput) (uint64, error) {
	tenantID, _ := ctxkey.TenantIDFrom(ctx)
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	// Bind the transaction to ctx so every helper below — including the quota
	// consumption, which reads and writes sys_tenant_quota through the tenant
	// repository — runs on this transaction instead of the pool.
	txCtx := ctxkey.WithTx(ctx, tx.Tx)

	if tenantID != 0 {
		if err := NewTenantRepository(r.db).ConsumeQuotaIfConfigured(txCtx, tenantID, "seats", 1); err != nil {
			return 0, err
		}
	}

	q := r.qtx(tx)
	userID, err := q.CreateUser(txCtx, sqlcstore.CreateUserParams{
		DepartmentID: ptrUint64Null(input.DepartmentID), Username: input.Username,
		PasswordHash: input.PasswordHash, DisplayName: input.DisplayName, Email: input.Email,
		Mobile: input.Mobile, Gender: int16(input.Gender), Status: int16(input.Status),
		MustChangePassword: boolSmallInt16(input.MustChangePassword), Remark: input.Remark,
		CreatedBy: actorID(ctx),
	})
	if err != nil {
		return 0, wrapDuplicate(err)
	}
	if err := replaceUserRelations(txCtx, q, uint64(userID), input.RoleIDs, input.PositionIDs); err != nil {
		return 0, err
	}
	if tenantID != 0 {
		if err := addTenantMember(txCtx, tx, tenantID, uint64(userID), input.DepartmentID,
			ctxkey.PlatformOperatorFrom(ctx)); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return uint64(userID), nil
}

// addTenantMember links a global account to one tenant. It is idempotent so a
// retried create cannot fail on the (tenant_id, user_id) primary key.
func addTenantMember(ctx context.Context, tx *Tx, tenantID, userID uint64, departmentID *uint64, platformOperator bool) error {
	_, err := tx.ExecContext(ctx, `
INSERT INTO sys_tenant_member (tenant_id, user_id, department_id, is_tenant_admin, is_platform_operator, status,
  created_by, updated_by)
VALUES ($1, $2, $3, FALSE, $4, 1, $5, $5)
ON CONFLICT (tenant_id, user_id) DO UPDATE
SET department_id = EXCLUDED.department_id,
    is_platform_operator = EXCLUDED.is_platform_operator,
    status = 1,
    updated_at = NOW(),
    updated_by = EXCLUDED.updated_by`,
		tenantID, userID, ptrUint64Null(departmentID), platformOperator, actorID(ctx))
	return err
}

// requireTenantMember rejects work on an account that is not an active member of
// the caller's tenant.
//
// sys_user is a global account table: the row carries no tenant_id and is not
// protected by row-level security. An id alone must therefore never authorise a
// write, otherwise one tenant's administrator could edit, disable, or reset the
// password of another tenant's account by guessing an id. A zero tenant means
// the platform view and is allowed.
func (r *AuthRepository) requireTenantMember(ctx context.Context, q *sqlcstore.Queries, userID uint64) error {
	tenantID, _ := ctxkey.TenantIDFrom(ctx)
	if tenantID == 0 {
		return nil
	}
	ok, err := q.IsTenantMember(ctx, sqlcstore.IsTenantMemberParams{
		TenantID: tenantID, UserID: userID,
	})
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotFound
	}
	return nil
}

// UpdateUser rewrites the profile and the assignments in one transaction, and revokes refresh tokens when the account is disabled.
func (r *AuthRepository) UpdateUser(ctx context.Context, userID uint64, input SaveUserInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	if err := r.requireTenantMember(ctx, q, userID); err != nil {
		return err
	}
	affected, err := q.UpdateUser(ctx, sqlcstore.UpdateUserParams{
		DepartmentID: ptrUint64Null(input.DepartmentID), DisplayName: input.DisplayName,
		Email: input.Email, Mobile: input.Mobile, Gender: int16(input.Gender),
		Status: int16(input.Status), MustChangePassword: boolSmallInt16(input.MustChangePassword),
		Remark: input.Remark, ID: int64(userID), UpdatedBy: actorID(ctx),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrUserProtected
	}
	if err := replaceUserRelations(ctx, q, userID, input.RoleIDs, input.PositionIDs); err != nil {
		return err
	}
	if err := r.syncMemberDepartment(ctx, q, userID, input.DepartmentID); err != nil {
		return err
	}
	if input.Status != 1 {
		if err := q.RevokeAllUserRefreshTokens(ctx, sqlcstore.RevokeAllUserRefreshTokensParams{
			RevokeReason: "user disabled", UserID: userID,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// syncMemberDepartment keeps the membership row's department in step with the
// account. The member row is what tenant-scoped department queries read.
func (r *AuthRepository) syncMemberDepartment(ctx context.Context, q *sqlcstore.Queries, userID uint64, departmentID *uint64) error {
	tenantID, _ := ctxkey.TenantIDFrom(ctx)
	if tenantID == 0 {
		return nil
	}
	err := q.UpdateTenantMemberDepartment(ctx, sqlcstore.UpdateTenantMemberDepartmentParams{
		TenantID: tenantID, UserID: userID, DepartmentID: ptrUint64Null(departmentID),
		Actor: actorID(ctx),
	})
	return err
}

// UpdateUserStatus enables or disables an account and revokes its refresh tokens when disabling.
func (r *AuthRepository) UpdateUserStatus(ctx context.Context, userID uint64, status int8) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	if err := r.requireTenantMember(ctx, q, userID); err != nil {
		return err
	}
	affected, err := q.UpdateUserStatus(ctx, sqlcstore.UpdateUserStatusParams{
		Status: int16(status), ID: int64(userID), UpdatedBy: actorID(ctx),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrUserProtected
	}
	if status != 1 {
		if err := q.RevokeAllUserRefreshTokens(ctx, sqlcstore.RevokeAllUserRefreshTokensParams{
			RevokeReason: "user disabled", UserID: userID,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteUser removes the account from the caller's tenant and revokes its
// sessions. The built-in administrator is protected by the query's WHERE clause.
//
// sys_user is shared by every tenant the account belongs to, so the membership
// row goes first and the account itself is only soft deleted once no tenant
// claims it. Deleting the global row from one tenant would silently take the
// account away from all the others.
func (r *AuthRepository) DeleteUser(ctx context.Context, userID uint64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	if err := r.requireTenantMember(ctx, q, userID); err != nil {
		return err
	}
	if err := q.RevokeAllUserRefreshTokens(ctx, sqlcstore.RevokeAllUserRefreshTokensParams{
		RevokeReason: "user deleted", UserID: userID,
	}); err != nil {
		return err
	}
	if tenantID, _ := ctxkey.TenantIDFrom(ctx); tenantID != 0 {
		if _, err := q.DeleteTenantMember(ctx, sqlcstore.DeleteTenantMemberParams{
			TenantID: tenantID, UserID: userID,
		}); err != nil {
			return err
		}
		if err := releaseQuota(ctx, q, tenantID, "seats", 1); err != nil {
			return err
		}
		remaining, err := q.CountMembershipsForUser(ctx, userID)
		if err != nil {
			return err
		}
		if remaining > 0 {
			// Another tenant still owns this account; leave the global row alone.
			return tx.Commit()
		}
	}
	affected, err := q.SoftDeleteUser(ctx, sqlcstore.SoftDeleteUserParams{
		ID: int64(userID), UpdatedBy: actorID(ctx),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrUserProtected
	}
	return tx.Commit()
}

// ResetUserPassword sets a new hash and revokes every refresh token of the user.
func (r *AuthRepository) ResetUserPassword(ctx context.Context, userID uint64, passwordHash string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	if err := r.requireTenantMember(ctx, q, userID); err != nil {
		return err
	}
	affected, err := q.ResetUserPassword(ctx, sqlcstore.ResetUserPasswordParams{
		PasswordHash: passwordHash, ID: int64(userID),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrUserProtected
	}
	if err := q.RevokeAllUserRefreshTokens(ctx, sqlcstore.RevokeAllUserRefreshTokensParams{
		RevokeReason: "password reset", UserID: userID,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

// ForceLogoutUser bumps token_version and revokes refresh tokens, so both the access and the refresh token of every device stop working.
func (r *AuthRepository) ForceLogoutUser(ctx context.Context, userID uint64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	if err := r.requireTenantMember(ctx, q, userID); err != nil {
		return err
	}
	if err := q.BumpUserTokenVersion(ctx, int64(userID)); err != nil {
		return err
	}
	if err := q.RevokeAllUserRefreshTokens(ctx, sqlcstore.RevokeAllUserRefreshTokensParams{
		RevokeReason: "forced logout", UserID: userID,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func replaceUserRelations(ctx context.Context, q *sqlcstore.Queries, userID uint64, roleIDs, positionIDs []uint64) error {
	actor := actorID(ctx)
	if err := q.DeleteUserRoles(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteUserPositions(ctx, userID); err != nil {
		return err
	}
	for _, roleID := range distinctIDs(roleIDs) {
		if err := q.InsertUserRole(ctx, sqlcstore.InsertUserRoleParams{
			UserID: userID, RoleID: roleID, CreatedBy: actor,
		}); err != nil {
			return fmt.Errorf("assign role %d: %w", roleID, err)
		}
	}
	for _, positionID := range distinctIDs(positionIDs) {
		if err := q.InsertUserPosition(ctx, sqlcstore.InsertUserPositionParams{
			UserID: userID, PositionID: positionID, CreatedBy: actor,
		}); err != nil {
			return fmt.Errorf("assign position %d: %w", positionID, err)
		}
	}
	return nil
}

func distinctIDs(values []uint64) []uint64 {
	seen := make(map[uint64]struct{}, len(values))
	result := make([]uint64, 0, len(values))
	for _, value := range values {
		if value == 0 {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

func normalizePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 200 {
		pageSize = 200
	}
	return page, pageSize
}
