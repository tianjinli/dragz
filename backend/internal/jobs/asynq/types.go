// Package jobasynq wraps the asynq client, task types and queue options used
// by both the API process and jobs-runner.
package jobasynq

const (
	// TaskScheduledJob is the task type that executes one job.
	TaskScheduledJob = "scheduled:job"
	// TaskSchedulerSync is the task type that asks jobs-runner to re-read the job table after a CRUD change.
	TaskSchedulerSync = "scheduler:reconcile"
	// TaskOutboxRelay names the outbox relay. jobs-runner performs it as the
	// scheduled handler outbox.publish (see internal/jobs), which is registered
	// on TaskScheduledJob like every other maintenance job.
	TaskOutboxRelay = "outbox.relay"
	// DefaultQueue is the queue every task is enqueued on unless configuration says otherwise.
	DefaultQueue = "default"
)

// JobPayload is the body of a scheduled job task.
// TenantID is set when the enqueueing request is inside a tenant. Zero means
// the task is not scoped; the outbox handler still sets the tenant per row.
type JobPayload struct {
	JobID       uint64 `json:"jobId"`
	TriggerType string `json:"triggerType"`
	TenantID    uint64 `json:"tenantId,omitempty"`
	// Deferred counts how many times the runner postponed this job because the
	// tenant was already at its concurrency limit. It bounds the deferral loop
	// so a permanently saturated tenant cannot enqueue copies forever.
	Deferred int `json:"deferred,omitempty"`
}
