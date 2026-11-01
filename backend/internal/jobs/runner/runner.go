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

	"dragz-admin/backend/internal/config"
	"dragz-admin/backend/internal/jobs"
	jobasynq "dragz-admin/backend/internal/jobs/asynq"
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
	r := &Runner{
		server:     server,
		scheduler:  scheduler,
		reconciler: NewReconciler(repo, scheduler),
		mux:        asynq.NewServeMux(),
		executor:   executor,
		repo:       repo,
	}
	r.mux.HandleFunc(jobasynq.TaskScheduledJob, r.handleScheduledJob)
	r.mux.HandleFunc(jobasynq.TaskSchedulerSync, r.handleSchedulerSync)
	return r, nil
}

// Run performs one synchronous reconcile, then starts the scheduler, the worker and the 60-second reconcile loop. It blocks until the context is cancelled or one of the components dies.
func (r *Runner) Run(ctx context.Context) error {
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
	job, err := r.repo.JobByID(ctx, payload.JobID)
	if err != nil {
		return err
	}
	if !jobs.CanRun(job, payload.TriggerType) {
		return nil
	}
	attempt := 1
	if retryCount, ok := asynq.GetRetryCount(ctx); ok {
		attempt = retryCount + 1
	}
	result, runErr := r.executor.ExecuteOnce(ctx, job, payload.TriggerType, attempt)
	if runErr != nil {
		return errors.New(result.Error)
	}
	return nil
}
