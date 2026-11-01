// Package logic is the orchestration layer between the handlers and the
// repositories, so handlers never reach into internal/store directly.
package logic

import (
	"context"
	"time"

	"dragz-admin/backend/internal/httpapi/svc"
	"dragz-admin/backend/internal/store"
)

// AuthLogic is the orchestration layer for the login flow.
type AuthLogic struct {
	svcCtx *svc.ServiceContext
}

// NewAuthLogic binds the logic layer to the service context.
func NewAuthLogic(svcCtx *svc.ServiceContext) *AuthLogic {
	return &AuthLogic{svcCtx: svcCtx}
}

// LoginSecuritySettings returns the resolved lockout, captcha and password policy.
func (l *AuthLogic) LoginSecuritySettings(ctx context.Context) (store.LoginSecuritySettings, error) {
	return l.svcCtx.Auth.LoginSecuritySettings(ctx)
}

// FindUserByUsername loads the credentials of one account for a password login.
func (l *AuthLogic) FindUserByUsername(ctx context.Context, username string) (store.User, error) {
	return l.svcCtx.Auth.FindUserByUsername(ctx, username)
}

// FindUserByID loads the credentials of one account by ID.
func (l *AuthLogic) FindUserByID(ctx context.Context, userID uint64) (store.User, error) {
	return l.svcCtx.Auth.FindUserByID(ctx, userID)
}

// RecordLoginFailure bumps the failure counter, applies the lock when the threshold is reached, and writes the login log.
func (l *AuthLogic) RecordLoginFailure(ctx context.Context, username, ip, userAgent, reason string, lockUntil *time.Time) error {
	return l.svcCtx.Auth.RecordLoginFailure(ctx, username, ip, userAgent, reason, lockUntil)
}
