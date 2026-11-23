// Package runner implements jobs-runner: an asynq server, an asynq scheduler
// and the reconciler that keeps them in step with the job table.
package runner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/logx"
	redisstore "github.com/zeromicro/go-zero/core/stores/redis"

	"dragz-admin/backend/internal/config"
	"dragz-admin/backend/internal/fairgate"
	"dragz-admin/backend/internal/jobs"
	jobasynq "dragz-admin/backend/internal/jobs/asynq"
	"dragz-admin/backend/internal/pkg/ctxkey"
	"dragz-admin/backend/internal/store"
)

// Runner is the jobs-runner process: an asynq server, an asynq scheduler and the reconciler that keeps the two in step with the database.
type Runner struct {
	server     *asynq.Server
	scheduler  *asynq.Scheduler
	reconciler *Reconciler
	mux        *asynq.ServeMux
	executor   *jobs.Executor
	repo       *store.AuthRepository
	// gate bounds one tenant's concurrent scheduled runs. It is what makes the
	// scheduler fair across tenants: a tenant that enqueues a slow job at :00
	// cannot occupy every worker and delay everyone else.
	gate *fairgate.Gate
	// client re-enqueues a job that the gate turned away. The original task
	// completes successfully, so asynq's retry budget is untouched.
	client *asynq.Client
}

// NewRunner builds the server, the scheduler and the handler mux. Configuration defaults are applied here so an empty asynq block still yields a working runner.
func NewRunner(c config.Config, repo *store.AuthRepository, executor *jobs.Executor) (*Runner, error) {
	redisOpt := jobasynq.RedisOpt(c.Redis)
	concurrency := c.Asynq.Concurrency
	if concurrency <= 0 {
		concurrency = 10
	}
	queues := c.Asynq.Queues
	if len(queues) == 0 {
		queues = map[string]int{jobasynq.DefaultQueue: 3}
	}
	server := asynq.NewServer(redisOpt, asynq.Config{
		Concurrency: concurrency,
		Queues:      queues,
	})
	scheduler := asynq.NewScheduler(redisOpt, &asynq.SchedulerOpts{Location: time.Local})
	// The gate shares the Redis instance asynq already uses. A second pool is
	// cheap compared with the cost of a tenant starving the queue, and keeping
	// it here means the runner owns its own lifecycle.
	cache, err := redisstore.NewRedis(c.Redis)
	if err != nil {
		return nil, fmt.Errorf("connect redis for the fairness gate: %w", err)
	}
	gate := fairgate.New(cache, c.Asynq.PerTenantConcurrency, time.Hour)
	r := &Runner{
		server:     server,
		scheduler:  scheduler,
		reconciler: NewReconciler(repo, scheduler),
		mux:        asynq.NewServeMux(),
		executor:   executor,
		repo:       repo,
		gate:       gate,
		client:     asynq.NewClient(redisOpt),
	}
	r.mux.HandleFunc(jobasynq.TaskScheduledJob, r.handleScheduledJob)
	r.mux.HandleFunc(jobasynq.TaskSchedulerSync, r.handleSchedulerSync)
	return r, nil
}

// Run performs one synchronous reconcile, then starts the scheduler, the worker and the 60-second reconcile loop. It blocks until the context is cancelled or one of the components dies.
func (r *Runner) Run(ctx context.Context) error {
	defer func() {
		if r.client != nil {
			_ = r.client.Close()
		}
	}()
	if err := r.reconciler.SyncAll(ctx); err != nil {
		return fmt.Errorf("initial scheduler sync: %w", err)
	}
	errCh := make(chan error, 2)
	go func() {
		if err := r.scheduler.Run(); err != nil {
			errCh <- fmt.Errorf("scheduler stopped: %w", err)
		}
	}()
	go func() {
		if err := r.server.Run(r.mux); err != nil {
			errCh <- fmt.Errorf("worker stopped: %w", err)
		}
	}()
	go r.reconcileLoop(ctx)
	select {
	case <-ctx.Done():
		r.scheduler.Shutdown()
		r.server.Shutdown()
		return ctx.Err()
	case err := <-errCh:
		r.scheduler.Shutdown()
		r.server.Shutdown()
		return err
	}
}

func (r *Runner) reconcileLoop(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.reconciler.SyncAll(ctx); err != nil {
				logx.Errorw("scheduled job reconcile failed", logx.Field("error", err.Error()))
			}
		}
	}
}

func (r *Runner) handleSchedulerSync(ctx context.Context, _ *asynq.Task) error {
	return r.reconciler.SyncAll(ctx)
}

// maxDeferrals bounds how long a job may keep waiting for a tenant slot.
// Fifteen deferrals at ~30s is roughly eight minutes; a tenant that is still
// saturated after that is better served by a logged skip than by an unbounded
// re-enqueue loop.
const maxDeferrals = 15

// deferralDelay spreads retries so N deferred jobs do not all wake at once.
func deferralDelay(attempt int) time.Duration {
	base := 20 * time.Second
	if attempt > 1 {
		base += time.Duration(attempt) * 5 * time.Second
	}
	if base > 2*time.Minute {
		base = 2 * time.Minute
	}
	return base
}

func (r *Runner) handleScheduledJob(ctx context.Context, task *asynq.Task) error {
	var payload jobasynq.JobPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}
	if payload.TriggerType == "" {
		payload.TriggerType = "scheduled"
	}
	enabled, err := r.repo.FeatureEnabled(ctx, "feature.scheduler.enabled", true)
	if err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	// The runner is not inside one tenant. Bypass is required to load the job
	// row; the handler sets the tenant itself when a row carries one.
	job, err := r.repo.JobByID(ctxkey.WithRLSBypass(ctx, true), payload.JobID)
	if err != nil {
		return err
	}
	if !jobs.CanRun(job, payload.TriggerType) {
		return nil
	}
	// Admission control per tenant. A run that cannot take a slot is put back
	// with a delay instead of being executed: the alternative is that one
	// tenant's long job delays every other tenant's cron for as long as it runs.
	release, admitted := r.gate.Acquire(ctx, payload.TenantID)
	if !admitted {
		if payload.Deferred >= maxDeferrals {
			logx.Errorw("scheduled job skipped: tenant is at its concurrency limit",
				logx.Field("jobId", payload.JobID), logx.Field("tenantId", payload.TenantID))
			return nil
		}
		payload.Deferred++
		body, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		_, err = r.client.Enqueue(asynq.NewTask(jobasynq.TaskScheduledJob, body,
			asynq.Queue(jobasynq.DefaultQueue),
			asynq.ProcessIn(deferralDelay(payload.Deferred)),
			asynq.MaxRetry(job.MaxRetries),
			asynq.Timeout(time.Duration(job.TimeoutSeconds)*time.Second),
		))
		return err
	}
	defer release()
	attempt := 1
	if retryCount, ok := asynq.GetRetryCount(ctx); ok {
		attempt = retryCount + 1
	}
	runCtx := ctx
	if payload.TenantID != 0 {
		runCtx = ctxkey.WithTenantID(ctx, payload.TenantID)
	}
	result, runErr := r.executor.ExecuteOnce(runCtx, job, payload.TriggerType, attempt)
	if runErr != nil {
		return errors.New(result.Error)
	}
	return nil
}
