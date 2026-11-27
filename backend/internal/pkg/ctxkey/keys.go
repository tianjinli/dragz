// Package ctxkey defines the request-scoped context keys shared by the
// middleware chain and the handlers.
package ctxkey

import (
	"context"
	"database/sql"
)

// key is an unexported type so a context key from this package can never
// collide with one written by another package.
type key int

// Request-scoped context keys. They are set by the middleware chain and read by
// the handlers; nothing else should write them.
const (
	// UserID holds the authenticated user id.
	UserID key = iota + 1
	// Username holds the authenticated username, kept for audit logging.
	Username
	// TokenScopes holds the scopes of an API-token caller.
	TokenScopes
	// Translator holds the request-scoped localizer.
	Translator
	// TenantID holds the tenant the request is acting inside. Zero means the
	// platform domain (no tenant selected).
	TenantID
	// HostTenantID is the tenant implied by the request Host. It is absent
	// when the host is a platform host or does not resolve.
	HostTenantID
	// IsPlatformOperator is true when the caller is a platform operator inside
	// the current tenant.
	IsPlatformOperator
	// Tx holds the *sql.Tx the store should use for this request.
	Tx
	// RLSBypass is true when the caller may see every tenant. Startup and the
	// bootstrap administrator set it; request handlers do not.
	RLSBypass
	// MustChangePassword is set from the session user on every authenticated
	// request. It is absent for API tokens, which never have an initial password.
	MustChangePassword
)

// WithUserID stores the authenticated user id. The middleware chain sets it
// once per request; audit columns read it through UserIDFrom.
func WithUserID(ctx context.Context, id uint64) context.Context {
	return context.WithValue(ctx, UserID, id)
}

// UserIDFrom reads the user id stored by WithUserID. A missing value reports
// false, which audit columns treat as "no user context" and record as 0.
func UserIDFrom(ctx context.Context) (uint64, bool) {
	if ctx == nil {
		return 0, false
	}
	id, ok := ctx.Value(UserID).(uint64)
	return id, ok && id > 0
}

// WithTenantID stores the tenant the rest of the request should use.
func WithTenantID(ctx context.Context, id uint64) context.Context {
	return context.WithValue(ctx, TenantID, id)
}

// WithHostTenantID stores the tenant resolved from the request Host.
func WithHostTenantID(ctx context.Context, id uint64) context.Context {
	return context.WithValue(ctx, HostTenantID, id)
}

// HostTenantIDFrom reads the tenant stored by WithHostTenantID.
func HostTenantIDFrom(ctx context.Context) (uint64, bool) {
	if ctx == nil {
		return 0, false
	}
	id, ok := ctx.Value(HostTenantID).(uint64)
	return id, ok
}

// TenantIDFrom reads the tenant stored by WithTenantID.
func TenantIDFrom(ctx context.Context) (uint64, bool) {
	if ctx == nil {
		return 0, false
	}
	id, ok := ctx.Value(TenantID).(uint64)
	return id, ok
}

// WithPlatformOperator records whether the caller is a platform operator.
func WithPlatformOperator(ctx context.Context, operator bool) context.Context {
	return context.WithValue(ctx, IsPlatformOperator, operator)
}

// PlatformOperatorFrom reports the flag stored by WithPlatformOperator.
// A missing value is false.
func PlatformOperatorFrom(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	operator, _ := ctx.Value(IsPlatformOperator).(bool)
	return operator
}

// WithTx attaches the transaction later store calls should run on.
func WithTx(ctx context.Context, tx *sql.Tx) context.Context {
	return context.WithValue(ctx, Tx, tx)
}

// TxFrom returns the transaction stored by WithTx.
func TxFrom(ctx context.Context) (*sql.Tx, bool) {
	if ctx == nil {
		return nil, false
	}
	tx, ok := ctx.Value(Tx).(*sql.Tx)
	return tx, ok && tx != nil
}

// WithoutTx drops any transaction stored by WithTx.
//
// Platform work uses this before opening its own transaction on the platform
// pool. Without it, *DB.BeginTx would see the request transaction and open a
// savepoint on it instead — silently running cross-tenant SQL on the
// application role's connection, which is exactly what the separate platform
// identity exists to prevent.
func WithoutTx(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	if _, ok := TxFrom(ctx); !ok {
		return ctx
	}
	return context.WithValue(ctx, Tx, (*sql.Tx)(nil))
}

// WithRLSBypass marks the context as allowed to ignore tenant row filters.
func WithRLSBypass(ctx context.Context, bypass bool) context.Context {
	return context.WithValue(ctx, RLSBypass, bypass)
}

// RLSBypassFrom reports whether WithRLSBypass stored true.
func RLSBypassFrom(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	bypass, _ := ctx.Value(RLSBypass).(bool)
	return bypass
}

// WithMustChangePassword records whether the session user still needs to
// replace the initial password. Only the JWT authentication branch sets it.
func WithMustChangePassword(ctx context.Context, required bool) context.Context {
	return context.WithValue(ctx, MustChangePassword, required)
}

// MustChangePasswordFrom reports the flag stored by WithMustChangePassword.
// ok is false for API tokens and unauthenticated requests.
func MustChangePasswordFrom(ctx context.Context) (required bool, ok bool) {
	if ctx == nil {
		return false, false
	}
	required, ok = ctx.Value(MustChangePassword).(bool)
	return required, ok
}
