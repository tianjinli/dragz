package runner

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	"github.com/robfig/cron/v3"
	"github.com/zeromicro/go-zero/core/logx"

	jobasynq "dragz-admin/backend/internal/jobs/asynq"
	"dragz-admin/backend/internal/store"
)

type scheduledEntry struct {
	cron    string
	entryID string
}

// Reconciler keeps the asynq scheduler in step with the job table. The table is the source of truth; the scheduler is a derived view, rebuilt from scratch on every sync.
type Reconciler struct {
	repo      *store.AuthRepository
	scheduler *asynq.Scheduler
	mu        sync.Mutex
	entries   map[string]scheduledEntry // job code -> cron + asynq entryID
}

// NewReconciler builds a reconciler with an empty entry registry.
func NewReconciler(repo *store.AuthRepository, scheduler *asynq.Scheduler) *Reconciler {
	return &Reconciler{
		repo:      repo,
		scheduler: scheduler,
		entries:   make(map[string]scheduledEntry),
	}
}

// SyncAll diffs the desired set of enabled jobs against the registered entries and applies the difference: an unchanged cron is left alone, a changed cron is re-registered, and a removed or disabled job is unregistered. When the scheduler feature is off, every entry is dropped.
func (r *Reconciler) SyncAll(ctx context.Context) error {
	if r == nil || r.scheduler == nil {
		return nil
	}
	enabled, err := r.repo.FeatureEnabled(ctx, "feature.scheduler.enabled", true)
	if err != nil {
		return err
	}
	jobs, err := r.repo.ListEnabledJobs(ctx)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	desired := make(map[string]store.ScheduledJob, len(jobs))
	for _, job := range jobs {
		desired[job.Code] = job
	}

	if !enabled {
		for code := range r.entries {
			r.unregisterLocked(code)
		}
		return nil
	}

	for code, job := range desired {
		if current, ok := r.entries[code]; ok && current.cron == job.CronExpression {
			if err := r.updateNextRunAt(ctx, job); err != nil {
				logx.Errorw("update job next run failed", logx.Field("jobId", job.ID), logx.Field("error", err.Error()))
			}
			continue
		}
		if _, ok := r.entries[code]; ok {
			if !r.unregisterLocked(code) {
				continue
			}
		}
		if err := r.registerLocked(ctx, job); err != nil {
			logx.Errorw("register scheduled job failed",
				logx.Field("jobId", job.ID),
				logx.Field("code", job.Code),
				logx.Field("error", err.Error()),
			)
		}
	}

	for code := range r.entries {
		if _, ok := desired[code]; !ok {
			r.unregisterLocked(code)
		}
	}
	return nil
}

func (r *Reconciler) registerLocked(ctx context.Context, job store.ScheduledJob) error {
	payload, err := json.Marshal(jobasynq.JobPayload{JobID: job.ID, TriggerType: "scheduled"})
	if err != nil {
		return err
	}
	task := asynq.NewTask(jobasynq.TaskScheduledJob, payload,
		asynq.Queue(jobasynq.DefaultQueue),
		asynq.MaxRetry(job.MaxRetries),
		asynq.Timeout(time.Duration(job.TimeoutSeconds)*time.Second),
	)
	entryID, err := r.scheduler.Register(job.CronExpression, task, asynq.TaskID(jobasynq.SchedulerEntryPref+job.Code))
	if err != nil {
		return err
	}
	r.entries[job.Code] = scheduledEntry{
		cron:    job.CronExpression,
		entryID: entryID,
	}
	return r.updateNextRunAt(ctx, job)
}

func (r *Reconciler) unregisterLocked(code string) bool {
	entry, ok := r.entries[code]
	if !ok {
		return true
	}
	if err := r.scheduler.Unregister(entry.entryID); err != nil {
		logx.Errorw("unregister scheduled job failed", logx.Field("code", code), logx.Field("error", err.Error()))
		return false
	}
	delete(r.entries, code)
	return true
}

func (r *Reconciler) updateNextRunAt(ctx context.Context, job store.ScheduledJob) error {
	schedule, err := cron.ParseStandard(job.CronExpression)
	if err != nil {
		return err
	}
	next := schedule.Next(time.Now())
	return r.repo.UpdateJobNextRunAt(ctx, job.ID, next)
}
