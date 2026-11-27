package notify

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"dragz-admin/backend/internal/store"
)

// This file is intentionally plain database/sql: a plugin owns its tables and
// does not add sqlc queries to the host schema. DBTX keeps every statement on
// the request transaction so app.tenant_id / RLS session variables apply.

var errNotFound = errors.New("notify: not found")

// ChannelRow is one delivery channel. ConfigJSON is encrypted at rest.
type ChannelRow struct {
	ID          uint64 `json:"id"`
	ChannelType string `json:"channelType"`
	Name        string `json:"name"`
	Enabled     int16  `json:"enabled"`
	ConfigJSON  string `json:"config"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// TemplateRow is a rendered-message template.
type TemplateRow struct {
	ID          uint64 `json:"id"`
	ChannelType string `json:"channelType"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Subject     string `json:"subject"`
	Content     string `json:"content"`
	Status      int16  `json:"status"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// LogRow is one delivery attempt.
type LogRow struct {
	ID           uint64 `json:"id"`
	ChannelType  string `json:"channelType"`
	ChannelName  string `json:"channelName"`
	TemplateCode string `json:"templateCode"`
	Recipient    string `json:"recipient"`
	Subject      string `json:"subject"`
	Content      string `json:"content"`
	Status       int16  `json:"status"`
	ErrorMsg     string `json:"errorMsg"`
	SentAt       string `json:"sentAt"`
	CreatedAt    string `json:"createdAt"`
}

// dbq is the minimal database/sql surface used here. store.DBTX returns the
// request transaction (carrying the RLS session) or the pool fallback.
type dbq interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type repo struct{ db *store.DB }

func newRepo(db *store.DB) *repo { return &repo{db: db} }

func (r *repo) dbtx(ctx context.Context) dbq {
	return store.DBTX(ctx, r.db)
}

func wrapDuplicate(err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(strings.ToLower(err.Error()), "duplicate key") ||
		strings.Contains(strings.ToLower(err.Error()), "unique constraint") {
		return fmt.Errorf("%w: duplicate", err)
	}
	return err
}

// ---------------------------------------------------------------------------
// channels
// ---------------------------------------------------------------------------

func (r *repo) listChannels(ctx context.Context, channelType, keyword string, page, pageSize int) ([]ChannelRow, int, error) {
	var total int
	if err := r.dbtx(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notify_channel WHERE deleted_at IS NULL
		 AND ($1 = '' OR channel_type = $1)
		 AND ($2 = '' OR name ILIKE '%' || $2 || '%')`, channelType, keyword).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.dbtx(ctx).QueryContext(ctx,
		`SELECT id, channel_type, name, enabled, config_json,
		        to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM notify_channel
		 WHERE deleted_at IS NULL
		   AND ($1 = '' OR channel_type = $1)
		   AND ($2 = '' OR name ILIKE '%' || $2 || '%')
		 ORDER BY id DESC LIMIT $3 OFFSET $4`, channelType, keyword, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []ChannelRow{}
	for rows.Next() {
		var it ChannelRow
		if err := rows.Scan(&it.ID, &it.ChannelType, &it.Name, &it.Enabled, &it.ConfigJSON, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, it)
	}
	return items, total, rows.Err()
}

func (r *repo) getChannel(ctx context.Context, id uint64) (ChannelRow, error) {
	var it ChannelRow
	err := r.dbtx(ctx).QueryRowContext(ctx,
		`SELECT id, channel_type, name, enabled, config_json,
		        to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM notify_channel WHERE id = $1 AND deleted_at IS NULL`, id).
		Scan(&it.ID, &it.ChannelType, &it.Name, &it.Enabled, &it.ConfigJSON, &it.CreatedAt, &it.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ChannelRow{}, errNotFound
	}
	return it, err
}

func (r *repo) createChannel(ctx context.Context, channelType, name, configJSON string, enabled int16) (uint64, error) {
	var id uint64
	err := r.dbtx(ctx).QueryRowContext(ctx,
		`INSERT INTO notify_channel (channel_type, name, enabled, config_json, created_by, updated_by)
		 VALUES ($1, $2, $3, $4, $5, $5) RETURNING id`,
		channelType, name, enabled, configJSON, store.ActorID(ctx)).Scan(&id)
	return id, wrapDuplicate(err)
}

func (r *repo) updateChannel(ctx context.Context, id uint64, channelType, name, configJSON string, enabled int16) error {
	result, err := r.dbtx(ctx).ExecContext(ctx,
		`UPDATE notify_channel SET channel_type=$2, name=$3, enabled=$4, config_json=$5, updated_at=NOW(),
		   updated_by=$6
		 WHERE id=$1 AND deleted_at IS NULL`, id, channelType, name, enabled, configJSON, store.ActorID(ctx))
	if err != nil {
		return wrapDuplicate(err)
	}
	return affected(result)
}

func (r *repo) deleteChannel(ctx context.Context, id uint64) error {
	result, err := r.dbtx(ctx).ExecContext(ctx,
		`UPDATE notify_channel SET deleted_at=NOW(), updated_at=NOW(), updated_by=$2
		 WHERE id=$1 AND deleted_at IS NULL`, id, store.ActorID(ctx))
	if err != nil {
		return err
	}
	return affected(result)
}

// ---------------------------------------------------------------------------
// templates
// ---------------------------------------------------------------------------

func (r *repo) listTemplates(ctx context.Context, channelType, keyword string, page, pageSize int) ([]TemplateRow, int, error) {
	var total int
	if err := r.dbtx(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notify_template WHERE deleted_at IS NULL
		 AND ($1 = '' OR channel_type = $1)
		 AND ($2 = '' OR code ILIKE '%' || $2 || '%' OR name ILIKE '%' || $2 || '%')`, channelType, keyword).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.dbtx(ctx).QueryContext(ctx,
		`SELECT id, channel_type, code, name, subject, content, status,
		        to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM notify_template
		 WHERE deleted_at IS NULL
		   AND ($1 = '' OR channel_type = $1)
		   AND ($2 = '' OR code ILIKE '%' || $2 || '%' OR name ILIKE '%' || $2 || '%')
		 ORDER BY id DESC LIMIT $3 OFFSET $4`, channelType, keyword, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []TemplateRow{}
	for rows.Next() {
		var it TemplateRow
		if err := rows.Scan(&it.ID, &it.ChannelType, &it.Code, &it.Name, &it.Subject, &it.Content, &it.Status, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, it)
	}
	return items, total, rows.Err()
}

func (r *repo) getTemplate(ctx context.Context, id uint64) (TemplateRow, error) {
	var it TemplateRow
	err := r.dbtx(ctx).QueryRowContext(ctx,
		`SELECT id, channel_type, code, name, subject, content, status,
		        to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM notify_template WHERE id=$1 AND deleted_at IS NULL`, id).
		Scan(&it.ID, &it.ChannelType, &it.Code, &it.Name, &it.Subject, &it.Content, &it.Status, &it.CreatedAt, &it.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TemplateRow{}, errNotFound
	}
	return it, err
}

func (r *repo) findTemplate(ctx context.Context, channelType, code string) (TemplateRow, error) {
	var it TemplateRow
	err := r.dbtx(ctx).QueryRowContext(ctx,
		`SELECT id, channel_type, code, name, subject, content, status,
		        to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),
		        to_char(updated_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM notify_template WHERE channel_type=$1 AND code=$2 AND status=1 AND deleted_at IS NULL`, channelType, code).
		Scan(&it.ID, &it.ChannelType, &it.Code, &it.Name, &it.Subject, &it.Content, &it.Status, &it.CreatedAt, &it.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return TemplateRow{}, errNotFound
	}
	return it, err
}

func (r *repo) createTemplate(ctx context.Context, channelType, code, name, subject, content string, status int16) (uint64, error) {
	var id uint64
	err := r.dbtx(ctx).QueryRowContext(ctx,
		`INSERT INTO notify_template (channel_type, code, name, subject, content, status, created_by, updated_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$7) RETURNING id`,
		channelType, code, name, subject, content, status, store.ActorID(ctx)).Scan(&id)
	return id, wrapDuplicate(err)
}

func (r *repo) updateTemplate(ctx context.Context, id uint64, channelType, code, name, subject, content string, status int16) error {
	result, err := r.dbtx(ctx).ExecContext(ctx,
		`UPDATE notify_template SET channel_type=$2, code=$3, name=$4, subject=$5, content=$6, status=$7,
		   updated_at=NOW(), updated_by=$8
		 WHERE id=$1 AND deleted_at IS NULL`,
		id, channelType, code, name, subject, content, status, store.ActorID(ctx))
	if err != nil {
		return wrapDuplicate(err)
	}
	return affected(result)
}

func (r *repo) deleteTemplate(ctx context.Context, id uint64) error {
	result, err := r.dbtx(ctx).ExecContext(ctx,
		`UPDATE notify_template SET deleted_at=NOW(), updated_at=NOW(), updated_by=$2
		 WHERE id=$1 AND deleted_at IS NULL`, id, store.ActorID(ctx))
	if err != nil {
		return err
	}
	return affected(result)
}

// ---------------------------------------------------------------------------
// logs
// ---------------------------------------------------------------------------

func (r *repo) createLog(ctx context.Context, lg LogRow) error {
	// sent_at is only set on success. Passing it as an explicit nullable
	// parameter avoids the $7 type-inference conflict a CASE WHEN would cause.
	var sentAt any
	if lg.Status == 1 {
		sentAt = time.Now()
	}
	_, err := r.dbtx(ctx).ExecContext(ctx,
		`INSERT INTO notify_log (channel_type, channel_name, template_code, recipient, subject, content, status, error_msg, sent_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		lg.ChannelType, lg.ChannelName, lg.TemplateCode, lg.Recipient, lg.Subject, lg.Content, lg.Status, lg.ErrorMsg, sentAt)
	return err
}

func (r *repo) listLogs(ctx context.Context, channelType string, status *int16, keyword string, page, pageSize int) ([]LogRow, int, error) {
	// status is optional: -1 means "any".
	st := int16(-1)
	if status != nil {
		st = *status
	}
	var total int
	if err := r.dbtx(ctx).QueryRowContext(ctx,
		`SELECT COUNT(*) FROM notify_log
		 WHERE ($1 = '' OR channel_type = $1)
		   AND ($2 = -1 OR status = $2)
		   AND ($3 = '' OR recipient ILIKE '%' || $3 || '%')`, channelType, st, keyword).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.dbtx(ctx).QueryContext(ctx,
		`SELECT id, channel_type, channel_name, template_code, recipient, subject, content, status, error_msg,
		        COALESCE(to_char(sent_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'), ''),
		        to_char(created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"')
		 FROM notify_log
		 WHERE ($1 = '' OR channel_type = $1)
		   AND ($2 = -1 OR status = $2)
		   AND ($3 = '' OR recipient ILIKE '%' || $3 || '%')
		 ORDER BY id DESC LIMIT $4 OFFSET $5`, channelType, st, keyword, pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	items := []LogRow{}
	for rows.Next() {
		var it LogRow
		if err := rows.Scan(&it.ID, &it.ChannelType, &it.ChannelName, &it.TemplateCode, &it.Recipient,
			&it.Subject, &it.Content, &it.Status, &it.ErrorMsg, &it.SentAt, &it.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, it)
	}
	return items, total, rows.Err()
}

func affected(result sql.Result) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return errNotFound
	}
	return nil
}
