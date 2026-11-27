package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/sqlc-dev/pqtype"

	"dragz-admin/backend/internal/pkg/ctxkey"
	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// ScheduledJob is one sys_scheduled_job row. The database stays the source of truth for the admin UI; jobs-runner mirrors it into asynq.
type ScheduledJob struct {
	ID             uint64          `json:"id"`
	TenantID       uint64          `json:"tenantId"`
	Name           string          `json:"name"`
	Code           string          `json:"code"`
	Handler        string          `json:"handler"`
	CronExpression string          `json:"cronExpression"`
	Payload        json.RawMessage `json:"payload"`
	TimeoutSeconds int             `json:"timeoutSeconds"`
	MaxRetries     int             `json:"maxRetries"`
	Status         int8            `json:"status"`
	LastRunAt      *time.Time      `json:"lastRunAt"`
	NextRunAt      *time.Time      `json:"nextRunAt"`
	CreatedAt      time.Time       `json:"createdAt"`
	UpdatedAt      time.Time       `json:"updatedAt"`
}

// SaveJobInput is the payload shared by create and update.
type SaveJobInput struct {
	Name           string
	Code           string
	Handler        string
	CronExpression string
	Payload        json.RawMessage
	TimeoutSeconds int
	MaxRetries     int
	Status         int8
	NextRunAt      *time.Time
}

// JobLog is one execution attempt. Success is a pointer so "still running" is distinguishable from "failed".
type JobLog struct {
	ID           uint64     `json:"id"`
	JobID        uint64     `json:"jobId"`
	TriggerType  string     `json:"triggerType"`
	StartedAt    time.Time  `json:"startedAt"`
	FinishedAt   *time.Time `json:"finishedAt"`
	Success      *bool      `json:"success"`
	Attempt      int        `json:"attempt"`
	Output       string     `json:"output"`
	ErrorMessage string     `json:"errorMessage"`
	CreatedAt    time.Time  `json:"createdAt"`
}

func mapJobFromList(row sqlcstore.ListJobsRow) ScheduledJob {
	return ScheduledJob{
		ID: uint64(row.ID), Name: row.Name, Code: row.Code, Handler: row.Handler,
		CronExpression: row.CronExpression, Payload: rawFromNull(row.Payload),
		TimeoutSeconds: int(row.TimeoutSeconds), MaxRetries: int(row.MaxRetries),
		Status: int8(row.Status), LastRunAt: nullTimePtr(row.LastRunAt), NextRunAt: nullTimePtr(row.NextRunAt),
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func mapJobFromEnabled(row sqlcstore.ListEnabledJobsRow) ScheduledJob {
	return ScheduledJob{
		ID: uint64(row.ID), TenantID: row.TenantID, Name: row.Name, Code: row.Code, Handler: row.Handler,
		CronExpression: row.CronExpression, Payload: rawFromNull(row.Payload),
		TimeoutSeconds: int(row.TimeoutSeconds), MaxRetries: int(row.MaxRetries),
		Status: int8(row.Status), LastRunAt: nullTimePtr(row.LastRunAt), NextRunAt: nullTimePtr(row.NextRunAt),
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

func mapJobFromGet(row sqlcstore.GetJobByIDRow) ScheduledJob {
	return ScheduledJob{
		ID: uint64(row.ID), Name: row.Name, Code: row.Code, Handler: row.Handler,
		CronExpression: row.CronExpression, Payload: rawFromNull(row.Payload),
		TimeoutSeconds: int(row.TimeoutSeconds), MaxRetries: int(row.MaxRetries),
		Status: int8(row.Status), LastRunAt: nullTimePtr(row.LastRunAt), NextRunAt: nullTimePtr(row.NextRunAt),
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
}

// ListJobs returns every scheduled job for the admin UI.
func (r *AuthRepository) ListJobs(ctx context.Context) ([]ScheduledJob, error) {
	rows, err := r.queries.ListJobs(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]ScheduledJob, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapJobFromList(row))
	}
	return items, nil
}

// withSession runs fn on a transaction that carries the tenant session from ctx.
func (r *AuthRepository) withSession(ctx context.Context, fn func(*sqlcstore.Queries) error) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(r.qtx(tx)); err != nil {
		return err
	}
	return tx.Commit()
}

// JobByID loads one scheduled job by primary key.
// The read runs in a transaction so row-level security sees the caller's tenant
// (or the bypass flag, which jobs-runner sets).
func (r *AuthRepository) JobByID(ctx context.Context, jobID uint64) (ScheduledJob, error) {
	var job ScheduledJob
	err := r.withSession(ctx, func(q *sqlcstore.Queries) error {
		row, err := q.GetJobByID(ctx, int64(jobID))
		if err != nil {
			return err
		}
		job = mapJobFromGet(row)
		return nil
	})
	return job, err
}

func jobPayload(value json.RawMessage) pqtype.NullRawMessage {
	if len(value) == 0 {
		return pqtype.NullRawMessage{RawMessage: json.RawMessage(`{}`), Valid: true}
	}
	return jsonNullRaw(value)
}

// CreateJob inserts a scheduled job.
func (r *AuthRepository) CreateJob(ctx context.Context, input SaveJobInput) (uint64, error) {
	id, err := r.queries.CreateJob(ctx, sqlcstore.CreateJobParams{
		Name: input.Name, Code: input.Code, Handler: input.Handler, CronExpression: input.CronExpression,
		Payload: jobPayload(input.Payload), TimeoutSeconds: int32(input.TimeoutSeconds),
		MaxRetries: int32(input.MaxRetries), Status: int16(input.Status), NextRunAt: nullTimeFromPtr(input.NextRunAt),
		CreatedBy: actorID(ctx),
	})
	return uint64(id), err
}

// UpdateJob rewrites a scheduled job.
func (r *AuthRepository) UpdateJob(ctx context.Context, jobID uint64, input SaveJobInput) error {
	affected, err := r.queries.UpdateJob(ctx, sqlcstore.UpdateJobParams{
		Name: input.Name, Code: input.Code, Handler: input.Handler, CronExpression: input.CronExpression,
		Payload: jobPayload(input.Payload), TimeoutSeconds: int32(input.TimeoutSeconds),
		MaxRetries: int32(input.MaxRetries), Status: int16(input.Status),
		NextRunAt: nullTimeFromPtr(input.NextRunAt), ID: int64(jobID), UpdatedBy: actorID(ctx),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// DeleteJob soft deletes a scheduled job.
func (r *AuthRepository) DeleteJob(ctx context.Context, jobID uint64) error {
	affected, err := r.queries.SoftDeleteJob(ctx, sqlcstore.SoftDeleteJobParams{
		ID: int64(jobID), UpdatedBy: actorID(ctx),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// ListEnabledJobs returns the jobs the reconciler has to register with the scheduler.
// The scheduler is process-wide, so the read bypasses tenant filters.
func (r *AuthRepository) ListEnabledJobs(ctx context.Context) ([]ScheduledJob, error) {
	ctx = ctxkey.WithRLSBypass(ctx, true)
	var items []ScheduledJob
	err := r.withSession(ctx, func(q *sqlcstore.Queries) error {
		rows, err := q.ListEnabledJobs(ctx)
		if err != nil {
			return err
		}
		items = make([]ScheduledJob, 0, len(rows))
		for _, row := range rows {
			items = append(items, mapJobFromEnabled(row))
		}
		return nil
	})
	return items, err
}

// UpdateJobNextRunAt records when the scheduler will fire the job next, so the list can show it.
// The scheduler writes this for every tenant, so the update bypasses tenant filters.
func (r *AuthRepository) UpdateJobNextRunAt(ctx context.Context, jobID uint64, next time.Time) error {
	ctx = ctxkey.WithRLSBypass(ctx, true)
	return r.withSession(ctx, func(q *sqlcstore.Queries) error {
		return q.UpdateJobNextRunAt(ctx, sqlcstore.UpdateJobNextRunAtParams{
			NextRunAt: sql.NullTime{Time: next, Valid: true}, ID: int64(jobID),
		})
	})
}

func mapJobLog(id int64, jobID uint64, triggerType string, startedAt time.Time, finishedAt sql.NullTime, success sql.NullInt16, attempt int32, output, errMsg string, createdAt time.Time) JobLog {
	item := JobLog{
		ID: uint64(id), JobID: jobID, TriggerType: triggerType, StartedAt: startedAt,
		Attempt: int(attempt), Output: output, ErrorMessage: errMsg, CreatedAt: createdAt,
		FinishedAt: nullTimePtr(finishedAt),
	}
	if success.Valid {
		v := success.Int16 != 0
		item.Success = &v
	}
	return item
}

// LatestJobLogSince returns the newest attempt of one trigger type after a point in time, which is how a manual run waits for its result.
func (r *AuthRepository) LatestJobLogSince(ctx context.Context, jobID uint64, triggerType string, since time.Time) (JobLog, error) {
	row, err := r.queries.LatestJobLogSince(ctx, sqlcstore.LatestJobLogSinceParams{
		JobID: jobID, TriggerType: triggerType, StartedAt: since,
	})
	if err != nil {
		return JobLog{}, err
	}
	return mapJobLog(row.ID, row.JobID, row.TriggerType, row.StartedAt, row.FinishedAt, row.Success, row.Attempt, row.Output, row.ErrorMessage, row.CreatedAt), nil
}

// StartJobLog opens an attempt row before the handler runs.
// Worker bookkeeping bypasses tenant filters: the runner is not inside one tenant.
func (r *AuthRepository) StartJobLog(ctx context.Context, jobID uint64, triggerType string, attempt int) (uint64, error) {
	ctx = ctxkey.WithRLSBypass(ctx, true)
	var id int64
	err := r.withSession(ctx, func(q *sqlcstore.Queries) error {
		var insertErr error
		id, insertErr = q.StartJobLog(ctx, sqlcstore.StartJobLogParams{
			JobID: jobID, TriggerType: triggerType, Attempt: int32(attempt),
		})
		return insertErr
	})
	return uint64(id), err
}

// FinishJobLog closes an attempt and stamps the job's last_run_at. The stamp is best effort: the attempt result is the record that matters.
func (r *AuthRepository) FinishJobLog(ctx context.Context, jobID, logID uint64, success bool, output, errorMessage string) error {
	ctx = ctxkey.WithRLSBypass(ctx, true)
	return r.withSession(ctx, func(q *sqlcstore.Queries) error {
		err := q.FinishJobLog(ctx, sqlcstore.FinishJobLogParams{
			Success:      nullBoolSmallInt(success),
			Output:       sql.NullString{String: output, Valid: true},
			ErrorMessage: sql.NullString{String: errorMessage, Valid: true},
			ID:           int64(logID), JobID: jobID,
		})
		if err != nil {
			return err
		}
		_ = q.TouchJobLastRunAt(ctx, int64(jobID))
		return nil
	})
}

// ListJobLogs returns one page of attempts for one job plus the total count.
func (r *AuthRepository) ListJobLogs(ctx context.Context, jobID uint64, page, pageSize int) ([]JobLog, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	total, err := r.queries.CountJobLogs(ctx, jobID)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.queries.ListJobLogs(ctx, sqlcstore.ListJobLogsParams{
		JobID: jobID, Limit: int32(pageSize), Offset: int32((page - 1) * pageSize),
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]JobLog, 0, len(rows))
	for _, row := range rows {
		items = append(items, mapJobLog(row.ID, row.JobID, row.TriggerType, row.StartedAt, row.FinishedAt, row.Success, row.Attempt, row.Output, row.ErrorMessage, row.CreatedAt))
	}
	return items, total, nil
}
