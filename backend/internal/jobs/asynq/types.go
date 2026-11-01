// Package jobasynq wraps the asynq client, task types and queue options used
// by both the API process and jobs-runner.
package jobasynq

const (
	// TaskScheduledJob is the task type that executes one job.
	TaskScheduledJob = "scheduled:job"
	// TaskSchedulerSync is the task type that asks jobs-runner to re-read the job table after a CRUD change.
	TaskSchedulerSync = "scheduler:reconcile"
	// DefaultQueue is the queue every task is enqueued on unless configuration says otherwise.
	DefaultQueue = "default"
	// SchedulerEntryPref prefixes the scheduler entry id of a job, so entries can be added and removed by job id.
	SchedulerEntryPref = "job:"
)

// JobPayload is the body of a scheduled job task.
type JobPayload struct {
	JobID       uint64 `json:"jobId"`
	TriggerType string `json:"triggerType"`
}
