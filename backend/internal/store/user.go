package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// UserListFilter is the query string of the user list. Scope carries the caller's data scope; nil means unrestricted.
type UserListFilter struct {
	Keyword      string
	DepartmentID uint64
	Status       *int8
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
		KeywordPattern: optionalPattern(filter.Keyword),
		DepartmentID:   optionalUint64Null(filter.DepartmentID),
		Status:         optionalInt16(filter.Status),
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
		KeywordPattern: params.KeywordPattern, DepartmentID: params.DepartmentID, Status: params.Status,
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
			IsSuperAdmin: smallIntBool(row.IsSuperAdmin), MustChangePassword: smallIntBool(row.MustChangePassword),
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
	row, err := r.queries.GetUserDetail(ctx, int64(userID))
	if err != nil {
		return UserDetail{}, err
	}
	detail := UserDetail{
		UserListItem: UserListItem{
			ID: uint64(row.ID), DepartmentID: nullInt64Ptr(row.DepartmentID),
			DepartmentName: row.DepartmentName, DepartmentCode: row.DepartmentCode,
			Username: row.Username, DisplayName: row.DisplayName,
			Email: row.Email, Mobile: row.Mobile, Gender: int8(row.Gender), Status: int8(row.Status),
			IsSuperAdmin: smallIntBool(row.IsSuperAdmin), MustChangePassword: smallIntBool(row.MustChangePassword),
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

// CreateUser inserts the account and its role/position assignments in one transaction.
func (r *AuthRepository) CreateUser(ctx context.Context, input SaveUserInput) (uint64, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	userID, err := q.CreateUser(ctx, sqlcstore.CreateUserParams{
		DepartmentID: ptrUint64Null(input.DepartmentID), Username: input.Username,
		PasswordHash: input.PasswordHash, DisplayName: input.DisplayName, Email: input.Email,
		Mobile: input.Mobile, Gender: int16(input.Gender), Status: int16(input.Status),
		MustChangePassword: boolSmallInt16(input.MustChangePassword), Remark: input.Remark,
	})
	if err != nil {
		return 0, wrapDuplicate(err)
	}
	if err := replaceUserRelations(ctx, q, uint64(userID), input.RoleIDs, input.PositionIDs); err != nil {
		return 0, err
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return uint64(userID), nil
}

// UpdateUser rewrites the profile and the assignments in one transaction, and revokes refresh tokens when the account is disabled.
func (r *AuthRepository) UpdateUser(ctx context.Context, userID uint64, input SaveUserInput) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	affected, err := q.UpdateUser(ctx, sqlcstore.UpdateUserParams{
		DepartmentID: ptrUint64Null(input.DepartmentID), DisplayName: input.DisplayName,
		Email: input.Email, Mobile: input.Mobile, Gender: int16(input.Gender),
		Status: int16(input.Status), MustChangePassword: boolSmallInt16(input.MustChangePassword),
		Remark: input.Remark, ID: int64(userID),
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
	if input.Status != 1 {
		if err := q.RevokeAllUserRefreshTokens(ctx, sqlcstore.RevokeAllUserRefreshTokensParams{
			RevokeReason: "user disabled", UserID: userID,
		}); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpdateUserStatus enables or disables an account and revokes its refresh tokens when disabling.
func (r *AuthRepository) UpdateUserStatus(ctx context.Context, userID uint64, status int8) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	affected, err := q.UpdateUserStatus(ctx, sqlcstore.UpdateUserStatusParams{
		Status: int16(status), ID: int64(userID),
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

// DeleteUser soft deletes an account and revokes its refresh tokens. The built-in administrator is protected by the query's WHERE clause.
func (r *AuthRepository) DeleteUser(ctx context.Context, userID uint64) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	q := r.qtx(tx)
	affected, err := q.SoftDeleteUser(ctx, int64(userID))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrUserProtected
	}
	if err := q.RevokeAllUserRefreshTokens(ctx, sqlcstore.RevokeAllUserRefreshTokensParams{
		RevokeReason: "user deleted", UserID: userID,
	}); err != nil {
		return err
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
	if err := q.DeleteUserRoles(ctx, userID); err != nil {
		return err
	}
	if err := q.DeleteUserPositions(ctx, userID); err != nil {
		return err
	}
	for _, roleID := range distinctIDs(roleIDs) {
		if err := q.InsertUserRole(ctx, sqlcstore.InsertUserRoleParams{UserID: userID, RoleID: roleID}); err != nil {
			return fmt.Errorf("assign role %d: %w", roleID, err)
		}
	}
	for _, positionID := range distinctIDs(positionIDs) {
		if err := q.InsertUserPosition(ctx, sqlcstore.InsertUserPositionParams{UserID: userID, PositionID: positionID}); err != nil {
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
