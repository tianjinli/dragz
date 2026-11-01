package jobs

import (
	"context"
	"errors"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"dragz-admin/backend/internal/security"
	"dragz-admin/backend/internal/store"
)

// ExecutionResult is the outcome of one job attempt: whether it succeeded, how many attempts were used, and the truncated output or error.
type ExecutionResult struct {
	Success  bool   `json:"success"`
	Attempts int    `json:"attempts"`
	Output   string `json:"output"`
	Error    string `json:"error"`
}

// Executor runs one job with its own timeout and records both the attempt row and the result. Only jobs-runner holds one.
type Executor struct {
	repo    *store.AuthRepository
	db      *store.DB
	Secrets *security.SecretBox
}

// NewExecutor binds the executor to the repository, the database handle and the secret box the storage handlers need.
func NewExecutor(repo *store.AuthRepository, db *store.DB, secrets *security.SecretBox) *Executor {
	return &Executor{repo: repo, db: db, Secrets: secrets}
}

// ExecuteOnce opens a job log, runs the handler under the job's timeout, then closes the log. Output is capped at 64 KiB and the error message at 4000 characters, so a chatty handler cannot bloat the log table.
func (e *Executor) ExecuteOnce(parent context.Context, job store.ScheduledJob, triggerType string, attempt int) (ExecutionResult, error) {
	result := ExecutionResult{Attempts: attempt}
	logID, err := e.repo.StartJobLog(parent, job.ID, triggerType, attempt)
	if err != nil {
		return result, err
	}
	timeout := time.Duration(job.TimeoutSeconds) * time.Second
	runContext, cancel := context.WithTimeout(parent, timeout)
	output, runErr := RunHandler(runContext, HandlerEnv{
		DB: e.db, Repo: e.repo, Secrets: e.Secrets,
	}, job)
	cancel()
	if len(output) > 64*1024 {
		output = output[:64*1024]
	}
	errorMessage := ""
	if runErr != nil {
		errorMessage = runErr.Error()
		if len(errorMessage) > 4000 {
			errorMessage = errorMessage[:4000]
		}
	}
	if err := e.repo.FinishJobLog(context.Background(), job.ID, logID, runErr == nil, output, errorMessage); err != nil {
		logx.Errorw("finish job log failed",
			logx.Field("jobId", job.ID),
			logx.Field("logId", logID),
			logx.Field("error", err.Error()),
		)
	}
	if runErr == nil {
		result.Success, result.Output = true, output
		return result, nil
	}
	result.Error = errorMessage
	return result, runErr
}

// ExecuteWithRetries retries ExecuteOnce until max_retries is exhausted and returns the last result together with the last error.
func (e *Executor) ExecuteWithRetries(parent context.Context, job store.ScheduledJob, triggerType string) (ExecutionResult, error) {
	var lastResult ExecutionResult
	var lastErr error
	for attempt := 1; attempt <= job.MaxRetries+1; attempt++ {
		lastResult, lastErr = e.ExecuteOnce(parent, job, triggerType, attempt)
		if lastErr == nil {
			return lastResult, nil
		}
	}
	if lastErr == nil {
		lastErr = errors.New(lastResult.Error)
	}
	return lastResult, lastErr
}
