package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"dragz-admin/backend/internal/security"
	"dragz-admin/backend/internal/store"
)

// HandlerEnv is what a maintenance handler is allowed to touch.
type HandlerEnv struct {
	DB      *store.DB
	Repo    *store.AuthRepository
	Secrets *security.SecretBox
}

var AllowedHandlers = []string{
	"maintenance.cleanup_refresh_tokens",
	"maintenance.cleanup_audit_logs",
	"maintenance.cleanup_failed_uploads",
	"maintenance.cleanup_storage_test_files",
	"maintenance.cleanup_orphan_files",
}

// HandlerAllowed reports whether a handler name is in the allowlist. A job row is data, so the handler name it carries must never reach arbitrary code.
func HandlerAllowed(handler string) bool {
	for _, candidate := range AllowedHandlers {
		if candidate == handler {
			return true
		}
	}
	return false
}

// RunHandler dispatches one allowlisted maintenance handler. Every limit in the payload is clamped here rather than in SQL, so a malformed payload cannot delete the whole table in one statement.
func RunHandler(ctx context.Context, env HandlerEnv, job store.ScheduledJob) (string, error) {
	payload := struct {
		RetentionDays  int `json:"retentionDays"`
		RetentionHours int `json:"retentionHours"`
		GraceDays      int `json:"graceDays"`
		RecycleDays    int `json:"recycleDays"`
		MaxFiles       int `json:"maxFiles"`
	}{}
	_ = json.Unmarshal(job.Payload, &payload)
	if payload.RetentionDays <= 0 {
		payload.RetentionDays = 30
	}
	if payload.RetentionDays > 3650 {
		return "", errors.New("retentionDays cannot exceed 3650")
	}
	cutoff := fmt.Sprintf("NOW() - (%d * INTERVAL '1 day')", payload.RetentionDays)
	var result sql.Result
	var err error
	switch job.Handler {
	case "maintenance.cleanup_refresh_tokens":
		result, err = env.DB.ExecContext(ctx, `DELETE FROM sys_refresh_token
      WHERE id IN (
        SELECT id FROM sys_refresh_token
        WHERE expires_at < `+cutoff+`
           OR (revoked_at IS NOT NULL AND revoked_at < `+cutoff+`)
        LIMIT 10000
      )`)
	case "maintenance.cleanup_audit_logs":
		tx, txErr := env.DB.BeginTx(ctx, nil)
		if txErr != nil {
			return "", txErr
		}
		defer func() { _ = tx.Rollback() }()
		total := int64(0)
		for _, query := range []string{
			`DELETE FROM sys_login_log WHERE id IN (
        SELECT id FROM sys_login_log WHERE created_at < ` + cutoff + ` LIMIT 10000)`,
			`DELETE FROM sys_operation_log WHERE id IN (
        SELECT id FROM sys_operation_log WHERE created_at < ` + cutoff + ` LIMIT 10000)`,
			`DELETE FROM sys_error_log WHERE id IN (
        SELECT id FROM sys_error_log WHERE resolved = 1 AND created_at < ` + cutoff + ` LIMIT 10000)`,
		} {
			deleted, deleteErr := tx.ExecContext(ctx, query)
			if deleteErr != nil {
				return "", deleteErr
			}
			count, _ := deleted.RowsAffected()
			total += count
		}
		if err := tx.Commit(); err != nil {
			return "", err
		}
		return fmt.Sprintf("deleted %d audit records", total), nil
	case "maintenance.cleanup_failed_uploads":
		result, err = env.DB.ExecContext(ctx, `UPDATE sys_file SET deleted_at = NOW(), status = 'purged'
      WHERE id IN (
        SELECT id FROM sys_file
        WHERE status = 'failed' AND deleted_at IS NULL
          AND created_at < `+cutoff+`
        LIMIT 10000
      )`)
	case "maintenance.cleanup_storage_test_files":
		retentionHours := payload.RetentionHours
		if retentionHours <= 0 {
			retentionHours = 24
		}
		if retentionHours > 168 {
			return "", errors.New("retentionHours cannot exceed 168")
		}
		return cleanupStorageTestFiles(ctx, env.DB, env.Repo, env.Secrets, retentionHours)
	case "maintenance.cleanup_orphan_files":
		if payload.MaxFiles > 5000 {
			return "", errors.New("maxFiles cannot exceed 5000")
		}
		return cleanupOrphanFiles(ctx, env.DB, env.Repo, env.Secrets, payload.GraceDays, payload.MaxFiles, payload.RecycleDays)

	default:
		return "", errors.New("job handler is not allowlisted")
	}
	if err != nil {
		return "", err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("affected %d records", count), nil
}
