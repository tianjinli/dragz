package store

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/sqlc-dev/pqtype"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

func (r *AuthRepository) qtx(tx *Tx) *sqlcstore.Queries {
	return r.queries.WithTx(tx.Tx)
}

func smallIntBool(v int16) bool { return v != 0 }

func boolSmallInt16(v bool) int16 {
	if v {
		return 1
	}
	return 0
}

func nullInt64Ptr(v sql.NullInt64) *uint64 {
	if !v.Valid {
		return nil
	}
	x := uint64(v.Int64)
	return &x
}

func nullTimePtr(v sql.NullTime) *time.Time {
	if !v.Valid {
		return nil
	}
	t := v.Time
	return &t
}

func ptrUint64Null(v *uint64) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(*v), Valid: true}
}

func optionalPattern(keyword string) sql.NullString {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: "%" + keyword + "%", Valid: true}
}

func optionalInt16(v *int8) sql.NullInt16 {
	if v == nil {
		return sql.NullInt16{}
	}
	return sql.NullInt16{Int16: int16(*v), Valid: true}
}

func optionalString(s string) sql.NullString {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func optionalPrefix(s string) sql.NullString {
	s = strings.TrimSpace(s)
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s + "%", Valid: true}
}

func optionalUint64Null(v uint64) sql.NullInt64 {
	if v == 0 {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Int64: int64(v), Valid: true}
}

func optionalBoolAsSmallInt(v *bool) sql.NullInt16 {
	if v == nil {
		return sql.NullInt16{}
	}
	return sql.NullInt16{Int16: boolSmallInt16(*v), Valid: true}
}

// uint64SliceOrEmpty returns a non-nil empty slice when ids is nil/empty (sqlc bigint[] narg).
func uint64SliceOrEmpty(ids []uint64) []uint64 {
	if len(ids) == 0 {
		return []uint64{}
	}
	return ids
}

func jsonNullRaw(value json.RawMessage) pqtype.NullRawMessage {
	if len(value) == 0 || string(value) == "null" {
		return pqtype.NullRawMessage{}
	}
	return pqtype.NullRawMessage{RawMessage: value, Valid: true}
}

func rawFromNull(v pqtype.NullRawMessage) json.RawMessage {
	if !v.Valid {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(v.RawMessage)
}

func nullTimeFromPtr(v *time.Time) sql.NullTime {
	if v == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *v, Valid: true}
}

func nullBoolSmallInt(v bool) sql.NullInt16 {
	return sql.NullInt16{Int16: boolSmallInt16(v), Valid: true}
}

// nullRawArray decodes a nullable JSON column that holds an array, returning an
// empty array when the column is NULL. It serves both the notice target list and
// the API-token IP allowlist.
func nullRawArray(v pqtype.NullRawMessage) json.RawMessage {
	if !v.Valid {
		return json.RawMessage(`[]`)
	}
	return json.RawMessage(v.RawMessage)
}

func mapAuthUser(row sqlcstore.GetUserByIDRow) User {
	return User{
		ID:                 uint64(row.ID),
		DepartmentID:       row.DepartmentID,
		Username:           row.Username,
		PasswordHash:       row.PasswordHash,
		DisplayName:        row.DisplayName,
		Email:              row.Email,
		Mobile:             row.Mobile,
		Status:             int8(row.Status),
		IsSuperAdmin:       smallIntBool(row.IsSuperAdmin),
		MustChangePassword: smallIntBool(row.MustChangePassword),
		FailedLoginCount:   int(row.FailedLoginCount),
		LockedUntil:        row.LockedUntil,
		TokenVersion:       int64(row.TokenVersion),
	}
}

func mapAuthUserByUsername(row sqlcstore.GetUserByUsernameRow) User {
	return User{
		ID:                 uint64(row.ID),
		DepartmentID:       row.DepartmentID,
		Username:           row.Username,
		PasswordHash:       row.PasswordHash,
		DisplayName:        row.DisplayName,
		Email:              row.Email,
		Mobile:             row.Mobile,
		Status:             int8(row.Status),
		IsSuperAdmin:       smallIntBool(row.IsSuperAdmin),
		MustChangePassword: smallIntBool(row.MustChangePassword),
		FailedLoginCount:   int(row.FailedLoginCount),
		LockedUntil:        row.LockedUntil,
		TokenVersion:       int64(row.TokenVersion),
	}
}
