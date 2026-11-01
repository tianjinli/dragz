package logic

import (
	"context"

	"dragz-admin/backend/internal/httpapi/svc"
	"dragz-admin/backend/internal/store"
)

// UserLogic is the orchestration layer for accounts. It exists so handlers never reach into the repository directly.
type UserLogic struct {
	svcCtx *svc.ServiceContext
}

// NewUserLogic binds the logic layer to the service context.
func NewUserLogic(svcCtx *svc.ServiceContext) *UserLogic {
	return &UserLogic{svcCtx: svcCtx}
}

// ListUsers returns one page of accounts within the caller's data scope.
func (l *UserLogic) ListUsers(ctx context.Context, filter store.UserListFilter) ([]store.UserListItem, int64, error) {
	return l.svcCtx.Auth.ListUsers(ctx, filter)
}

// UserDetail returns one account together with its role and position assignments.
func (l *UserLogic) UserDetail(ctx context.Context, userID uint64) (store.UserDetail, error) {
	return l.svcCtx.Auth.UserDetail(ctx, userID)
}

// CreateUser adds an account.
func (l *UserLogic) CreateUser(ctx context.Context, input store.SaveUserInput) (uint64, error) {
	return l.svcCtx.Auth.CreateUser(ctx, input)
}

// UpdateUser rewrites an account.
func (l *UserLogic) UpdateUser(ctx context.Context, userID uint64, input store.SaveUserInput) error {
	return l.svcCtx.Auth.UpdateUser(ctx, userID, input)
}

// UpdateUserStatus enables or disables an account.
func (l *UserLogic) UpdateUserStatus(ctx context.Context, userID uint64, status int8) error {
	return l.svcCtx.Auth.UpdateUserStatus(ctx, userID, status)
}

// DeleteUser removes an account.
func (l *UserLogic) DeleteUser(ctx context.Context, userID uint64) error {
	return l.svcCtx.Auth.DeleteUser(ctx, userID)
}

// ResetUserPassword sets a new password and logs the user out everywhere.
func (l *UserLogic) ResetUserPassword(ctx context.Context, userID uint64, passwordHash string) error {
	return l.svcCtx.Auth.ResetUserPassword(ctx, userID, passwordHash)
}
