package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// Session is one live refresh-token session as the 在线会话 list shows it.
type Session struct {
	ID          uint64     `json:"id"`
	UserID      uint64     `json:"userId"`
	Username    string     `json:"username"`
	DisplayName string     `json:"displayName"`
	DeviceID    string     `json:"deviceId"`
	DeviceName  string     `json:"deviceName"`
	LoginMethod string     `json:"loginMethod"`
	IP          string     `json:"ip"`
	UserAgent   string     `json:"userAgent"`
	ExpiresAt   time.Time  `json:"expiresAt"`
	LastUsedAt  *time.Time `json:"lastUsedAt"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// APIToken is one personal access token. Only the prefix is stored in clear; the token itself is shown once at creation.
type APIToken struct {
	ID          uint64          `json:"id"`
	UserID      uint64          `json:"userId"`
	Username    string          `json:"username"`
	Name        string          `json:"name"`
	TokenPrefix string          `json:"tokenPrefix"`
	Scopes      json.RawMessage `json:"scopes"`
	IPAllowlist json.RawMessage `json:"ipAllowlist"`
	Status      int8            `json:"status"`
	ExpiresAt   *time.Time      `json:"expiresAt"`
	LastUsedAt  *time.Time      `json:"lastUsedAt"`
	LastUsedIP  string          `json:"lastUsedIp"`
	CreatedAt   time.Time       `json:"createdAt"`
	RevokedAt   *time.Time      `json:"revokedAt"`
}

// APITokenPrincipal is the identity an API token resolves to during authentication.
type APITokenPrincipal struct {
	TokenID   uint64
	UserID    uint64
	Username  string
	Scopes    []string
	Allowlist []string
}

// LoginLog is one login attempt, successful or not.
type LoginLog struct {
	ID            uint64    `json:"id"`
	UserID        *uint64   `json:"userId"`
	Username      string    `json:"username"`
	LoginMethod   string    `json:"loginMethod"`
	Success       bool      `json:"success"`
	FailureReason string    `json:"failureReason"`
	IP            string    `json:"ip"`
	Location      string    `json:"location"`
	UserAgent     string    `json:"userAgent"`
	IsSuspicious  bool      `json:"isSuspicious"`
	CreatedAt     time.Time `json:"createdAt"`
}

// OperationLog is one audited write request. Permission holds the button code the guard enforced, which is why it survives on this table even though sys_menu.permission was deleted.
type OperationLog struct {
	ID           uint64          `json:"id"`
	UserID       *uint64         `json:"userId"`
	Username     string          `json:"username"`
	Module       string          `json:"module"`
	Action       string          `json:"action"`
	Permission   string          `json:"permission"`
	Method       string          `json:"method"`
	Path         string          `json:"path"`
	TargetType   string          `json:"targetType"`
	TargetID     string          `json:"targetId"`
	RequestID    string          `json:"requestId"`
	IP           string          `json:"ip"`
	Success      bool            `json:"success"`
	StatusCode   int             `json:"statusCode"`
	DurationMS   int64           `json:"durationMs"`
	BeforeJSON   json.RawMessage `json:"before"`
	AfterJSON    json.RawMessage `json:"after"`
	ErrorCode    string          `json:"errorCode"`
	ErrorMessage string          `json:"errorMessage"`
	CreatedAt    time.Time       `json:"createdAt"`
}

// ErrorLog is one server-side failure captured by the panic/500 handler.
type ErrorLog struct {
	ID         uint64     `json:"id"`
	RequestID  string     `json:"requestId"`
	UserID     *uint64    `json:"userId"`
	Module     string     `json:"module"`
	Method     string     `json:"method"`
	Path       string     `json:"path"`
	ErrorCode  string     `json:"errorCode"`
	Message    string     `json:"message"`
	IP         string     `json:"ip"`
	Resolved   bool       `json:"resolved"`
	ResolvedBy *uint64    `json:"resolvedBy"`
	ResolvedAt *time.Time `json:"resolvedAt"`
	CreatedAt  time.Time  `json:"createdAt"`
}

// OperationLogInput is what the audit middleware hands to the repository.
type OperationLogInput struct {
	UserID       uint64
	Username     string
	Module       string
	Action       string
	Permission   string
	Method       string
	Path         string
	TargetType   string
	TargetID     string
	RequestID    string
	IP           string
	UserAgent    string
	Success      bool
	StatusCode   int
	DurationMS   int64
	ErrorCode    string
	ErrorMessage string
}

// ErrorLogInput is what the recovery middleware hands to the repository.
type ErrorLogInput struct {
	RequestID string
	UserID    uint64
	Module    string
	Method    string
	Path      string
	ErrorCode string
	Message   string
	IP        string
}

// ListSessions returns one page of live sessions plus the total count.
func (r *AuthRepository) ListSessions(ctx context.Context, keyword string, page, pageSize int) ([]Session, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	pattern := optionalPattern(keyword)
	total, err := r.queries.CountSessionsFiltered(ctx, pattern)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.queries.ListSessionsFiltered(ctx, sqlcstore.ListSessionsFilteredParams{
		Limit: int32(pageSize), Offset: int32((page - 1) * pageSize), KeywordPattern: pattern,
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]Session, 0, len(rows))
	for _, row := range rows {
		items = append(items, Session{
			ID: uint64(row.ID), UserID: row.UserID, Username: row.Username, DisplayName: row.DisplayName,
			DeviceID: row.DeviceID, DeviceName: row.DeviceName, LoginMethod: row.LoginMethod,
			IP: row.Ip, UserAgent: row.UserAgent, ExpiresAt: row.ExpiresAt,
			LastUsedAt: nullTimePtr(row.LastUsedAt), CreatedAt: row.CreatedAt,
		})
	}
	return items, total, nil
}

// RevokeSession revokes one session, which also invalidates its refresh token.
func (r *AuthRepository) RevokeSession(ctx context.Context, sessionID uint64, reason string) error {
	affected, err := r.queries.RevokeSession(ctx, sqlcstore.RevokeSessionParams{
		RevokeReason: reason, ID: int64(sessionID),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// ListAPITokens returns one page of API tokens plus the total count.
func (r *AuthRepository) ListAPITokens(ctx context.Context, keyword string, userID *uint64, status *int8, page, pageSize int) ([]APIToken, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	params := sqlcstore.CountAPITokensFilteredParams{
		KeywordPattern: optionalPattern(keyword),
		UserID:         ptrUint64Null(userID),
		Status:         optionalInt16(status),
	}
	total, err := r.queries.CountAPITokensFiltered(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.queries.ListAPITokensFiltered(ctx, sqlcstore.ListAPITokensFilteredParams{
		Limit: int32(pageSize), Offset: int32((page - 1) * pageSize),
		KeywordPattern: params.KeywordPattern, UserID: params.UserID, Status: params.Status,
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]APIToken, 0, len(rows))
	for _, row := range rows {
		items = append(items, APIToken{
			ID: uint64(row.ID), UserID: row.UserID, Username: row.Username, Name: row.Name,
			TokenPrefix: row.TokenPrefix, Scopes: row.Scopes, IPAllowlist: nullRawArray(row.IpAllowlist),
			Status: int8(row.Status), ExpiresAt: nullTimePtr(row.ExpiresAt), LastUsedAt: nullTimePtr(row.LastUsedAt),
			LastUsedIP: row.LastUsedIp, CreatedAt: row.CreatedAt, RevokedAt: nullTimePtr(row.RevokedAt),
		})
	}
	return items, total, nil
}

// CreateAPIToken stores the hash of a freshly minted token together with its scopes and IP allowlist.
func (r *AuthRepository) CreateAPIToken(ctx context.Context, userID uint64, name, prefix, hash string, scopes, allowlist []string, expiresAt *time.Time) (uint64, error) {
	scopesJSON, _ := json.Marshal(scopes)
	allowlistJSON, _ := json.Marshal(allowlist)
	id, err := r.queries.CreateAPIToken(ctx, sqlcstore.CreateAPITokenParams{
		UserID: userID, Name: name, TokenPrefix: prefix, TokenHash: hash,
		Scopes: scopesJSON, IpAllowlist: jsonNullRaw(allowlistJSON), ExpiresAt: nullTimeFromPtr(expiresAt),
	})
	return uint64(id), err
}

// RevokeAPIToken revokes a token. A non-nil ownerID restricts the update to that user, which is how a self-service caller is stopped from revoking somebody else's token.
func (r *AuthRepository) RevokeAPIToken(ctx context.Context, tokenID uint64, ownerID *uint64) error {
	var affected int64
	var err error
	if ownerID != nil {
		affected, err = r.queries.RevokeAPITokenOwned(ctx, sqlcstore.RevokeAPITokenOwnedParams{
			ID: int64(tokenID), UserID: *ownerID,
		})
	} else {
		affected, err = r.queries.RevokeAPIToken(ctx, int64(tokenID))
	}
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// AuthenticateAPIToken resolves a presented token to a principal and enforces its expiry and IP allowlist. A successful call also stamps last_used_at.
func (r *AuthRepository) AuthenticateAPIToken(ctx context.Context, tokenHash, ip string) (APITokenPrincipal, error) {
	row, err := r.queries.AuthenticateAPIToken(ctx, tokenHash)
	if err != nil {
		return APITokenPrincipal{}, err
	}
	if row.ExpiresAt.Valid && !row.ExpiresAt.Time.After(time.Now()) {
		return APITokenPrincipal{}, ErrTokenExpired
	}
	var principal APITokenPrincipal
	principal.TokenID = uint64(row.ID)
	principal.UserID = row.UserID
	principal.Username = row.Username
	if err := json.Unmarshal(row.Scopes, &principal.Scopes); err != nil {
		return APITokenPrincipal{}, err
	}
	allowlist := []byte(`[]`)
	if row.IpAllowlist.Valid {
		allowlist = row.IpAllowlist.RawMessage
	}
	if err := json.Unmarshal(allowlist, &principal.Allowlist); err != nil {
		return APITokenPrincipal{}, err
	}
	if len(principal.Allowlist) > 0 && !containsString(principal.Allowlist, ip) {
		return APITokenPrincipal{}, ErrIPNotAllowed
	}
	_ = r.queries.TouchAPITokenUsage(ctx, sqlcstore.TouchAPITokenUsageParams{LastUsedIp: ip, ID: int64(principal.TokenID)})
	return principal, nil
}

// ListLoginLogs returns one page of login attempts plus the total count.
func (r *AuthRepository) ListLoginLogs(ctx context.Context, keyword string, success *bool, page, pageSize int) ([]LoginLog, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	params := sqlcstore.CountLoginLogsFilteredParams{
		KeywordPattern: optionalPattern(keyword),
		Success:        optionalBoolAsSmallInt(success),
	}
	total, err := r.queries.CountLoginLogsFiltered(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.queries.ListLoginLogsFiltered(ctx, sqlcstore.ListLoginLogsFilteredParams{
		Limit: int32(pageSize), Offset: int32((page - 1) * pageSize),
		KeywordPattern: params.KeywordPattern, Success: params.Success,
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]LoginLog, 0, len(rows))
	for _, row := range rows {
		items = append(items, LoginLog{
			ID: uint64(row.ID), UserID: nullInt64Ptr(row.UserID), Username: row.Username,
			LoginMethod: row.LoginMethod, Success: smallIntBool(row.Success), FailureReason: row.FailureReason,
			IP: row.Ip, Location: row.Location, UserAgent: row.UserAgent,
			IsSuspicious: smallIntBool(row.IsSuspicious), CreatedAt: row.CreatedAt,
		})
	}
	return items, total, nil
}

// ListOperationLogs returns one page of audited requests plus the total count.
func (r *AuthRepository) ListOperationLogs(ctx context.Context, keyword, module string, success *bool, page, pageSize int) ([]OperationLog, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	params := sqlcstore.CountOperationLogsFilteredParams{
		KeywordPattern: optionalPattern(keyword),
		Module:         optionalString(module),
		Success:        optionalBoolAsSmallInt(success),
	}
	total, err := r.queries.CountOperationLogsFiltered(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.queries.ListOperationLogsFiltered(ctx, sqlcstore.ListOperationLogsFilteredParams{
		Limit: int32(pageSize), Offset: int32((page - 1) * pageSize),
		KeywordPattern: params.KeywordPattern, Module: params.Module, Success: params.Success,
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]OperationLog, 0, len(rows))
	for _, row := range rows {
		items = append(items, OperationLog{
			ID: uint64(row.ID), UserID: nullInt64Ptr(row.UserID), Username: row.Username,
			Module: row.Module, Action: row.Action, Permission: row.Permission, Method: row.Method,
			Path: row.Path, TargetType: row.TargetType, TargetID: row.TargetID, RequestID: row.RequestID,
			IP: row.Ip, Success: smallIntBool(row.Success), StatusCode: int(row.StatusCode),
			DurationMS: int64(row.DurationMs), BeforeJSON: rawFromNull(row.BeforeJson), AfterJSON: rawFromNull(row.AfterJson),
			ErrorCode: row.ErrorCode, ErrorMessage: row.ErrorMessage, CreatedAt: row.CreatedAt,
		})
	}
	return items, total, nil
}

// ListErrorLogs returns one page of captured failures plus the total count.
func (r *AuthRepository) ListErrorLogs(ctx context.Context, keyword string, resolved *bool, page, pageSize int) ([]ErrorLog, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	params := sqlcstore.CountErrorLogsFilteredParams{
		KeywordPattern: optionalPattern(keyword),
		Resolved:       optionalBoolAsSmallInt(resolved),
	}
	total, err := r.queries.CountErrorLogsFiltered(ctx, params)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.queries.ListErrorLogsFiltered(ctx, sqlcstore.ListErrorLogsFilteredParams{
		Limit: int32(pageSize), Offset: int32((page - 1) * pageSize),
		KeywordPattern: params.KeywordPattern, Resolved: params.Resolved,
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]ErrorLog, 0, len(rows))
	for _, row := range rows {
		items = append(items, ErrorLog{
			ID: uint64(row.ID), RequestID: row.RequestID, UserID: nullInt64Ptr(row.UserID),
			Module: row.Module, Method: row.Method, Path: row.Path, ErrorCode: row.ErrorCode,
			Message: row.Message, IP: row.Ip, Resolved: smallIntBool(row.Resolved),
			ResolvedBy: nullInt64Ptr(row.ResolvedBy), ResolvedAt: nullTimePtr(row.ResolvedAt), CreatedAt: row.CreatedAt,
		})
	}
	return items, total, nil
}

// InsertOperationLog appends one audited request.
func (r *AuthRepository) InsertOperationLog(ctx context.Context, input OperationLogInput) error {
	return r.queries.InsertOperationLog(ctx, sqlcstore.InsertOperationLogParams{
		UserID: input.UserID, Username: input.Username, Module: input.Module, Action: input.Action,
		Permission: input.Permission, Method: input.Method, Path: input.Path, TargetType: input.TargetType,
		TargetID: input.TargetID, RequestID: input.RequestID, Ip: input.IP, UserAgent: input.UserAgent,
		Success: boolSmallInt16(input.Success), StatusCode: int32(input.StatusCode),
		DurationMs: uint64(input.DurationMS), ErrorCode: input.ErrorCode, ErrorMessage: input.ErrorMessage,
	})
}

// InsertErrorLog appends one captured failure.
func (r *AuthRepository) InsertErrorLog(ctx context.Context, input ErrorLogInput) error {
	return r.queries.InsertErrorLog(ctx, sqlcstore.InsertErrorLogParams{
		RequestID: input.RequestID, UserID: input.UserID, Module: input.Module, Method: input.Method,
		Path: input.Path, ErrorCode: input.ErrorCode, Message: input.Message, Ip: input.IP,
	})
}

// ResolveErrorLog marks a captured failure as handled. Resolving twice is refused, because the WHERE clause only matches unresolved rows.
func (r *AuthRepository) ResolveErrorLog(ctx context.Context, logID, resolverID uint64) error {
	affected, err := r.queries.ResolveErrorLog(ctx, sqlcstore.ResolveErrorLogParams{
		ResolvedBy: sql.NullInt64{Int64: int64(resolverID), Valid: true}, ID: int64(logID),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
