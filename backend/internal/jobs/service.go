// Package jobs defines the scheduled-job contract shared by the API process
// (enqueue only) and jobs-runner (execute).
package jobs

import (
	"context"
	"errors"

	"github.com/hibiken/asynq"

	jobasynq "dragz-admin/backend/internal/jobs/asynq"
	"dragz-admin/backend/internal/store"
)

// ErrNoWorker is returned when no jobs-runner is consuming the queue.
var ErrNoWorker = errors.New("job runner is not available")

// ErrNotInitialized is returned when the service was built without an asynq
// client, which only happens in tests and in a misconfigured deployment.
var ErrNotInitialized = errors.New("job service is not initialized")

// ErrSchedulerDisabled is returned when feature.scheduler.enabled is off.
var ErrSchedulerDisabled = errors.New("scheduler feature is disabled")

type jobRepo interface {
	FeatureEnabled(ctx context.Context, key string, defaultValue bool) (bool, error)
	JobByID(ctx context.Context, jobID uint64) (store.ScheduledJob, error)
}

type jobClient interface {
	EnqueueJob(ctx context.Context, jobID uint64, triggerType string, maxRetry int, timeoutSeconds int) (*asynq.TaskInfo, error)
	RequestReconcile(ctx context.Context) (*asynq.TaskInfo, error)
	ActiveWorkers(queue string) (int, error)
	Close() error
}

// Service is the API process's view of the scheduler: it only enqueues work and asks for a reconcile. It never executes a job.
type Service struct {
	repo   jobRepo
	client jobClient
}

// RunNowResult is returned immediately after enqueue; execution progress
// is observed via job logs.
type RunNowResult struct {
	Submitted bool   `json:"submitted"`
	JobID     uint64 `json:"jobId"`
	TaskID    string `json:"taskId,omitempty"`
}

// NewService binds the job service to the repository and the asynq client.
func NewService(repo *store.AuthRepository, client *jobasynq.Client) *Service {
	return &Service{repo: repo, client: client}
}

// CanRun reports whether a job may execute for the given trigger type. Manual
// triggers bypass the enabled switch so operators can run a disabled job once.
func CanRun(job store.ScheduledJob, triggerType string) bool {
	return job.Status == 1 || triggerType == "manual"
}

// Close releases the asynq client. A nil service is tolerated so tests can skip the scheduler.
func (s *Service) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

// RequestReconcile asks jobs-runner to re-read the job table. It is fire-and-forget: a nil client is a no-op.
func (s *Service) RequestReconcile(ctx context.Context) error {
	if s == nil || s.client == nil {
		return nil
	}
	_, err := s.client.RequestReconcile(ctx)
	return err
}

// RunNow enqueues a manual run and returns immediately. The caller polls the job log for the result, because an HTTP request cannot outlive a job whose timeout is measured in minutes.
func (s *Service) RunNow(ctx context.Context, jobID uint64) (RunNowResult, error) {
	if s == nil || s.client == nil {
		return RunNowResult{}, ErrNotInitialized
	}
	enabled, err := s.repo.FeatureEnabled(ctx, "feature.scheduler.enabled", true)
	if err != nil {
		return RunNowResult{}, err
	}
	if !enabled {
		return RunNowResult{}, ErrSchedulerDisabled
	}
	job, err := s.repo.JobByID(ctx, jobID)
	if err != nil {
		return RunNowResult{}, err
	}
	workers, err := s.client.ActiveWorkers(jobasynq.DefaultQueue)
	if err != nil {
		return RunNowResult{}, err
	}
	if workers == 0 {
		return RunNowResult{}, ErrNoWorker
	}
	// Async enqueue only: the HTTP layer has a 10s timeout while job
	// timeout_seconds can be 120-300s, so waiting synchronously for the
	// jobs-runner to finish (or timing out when the runner is down)
	// surfaces as HTTP 503. The caller polls execution via job logs.
	info, err := s.client.EnqueueJob(ctx, job.ID, "manual", job.MaxRetries, job.TimeoutSeconds)
	if err != nil {
		return RunNowResult{}, err
	}
	result := RunNowResult{Submitted: true, JobID: job.ID}
	if info != nil {
		result.TaskID = info.ID
	}
	return result, nil
}
