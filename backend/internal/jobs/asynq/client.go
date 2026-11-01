package jobasynq

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/zeromicro/go-zero/core/stores/redis"
)

// serverLister is the subset of *asynq.Inspector used for worker discovery.
type serverLister interface {
	Servers() ([]*asynq.ServerInfo, error)
	Close() error
}

// Client is the producer half of asynq: it enqueues tasks and inspects the running servers. The API process holds one and never consumes.
type Client struct {
	inner     *asynq.Client
	inspector serverLister
}

// NewClient builds the client and its inspector from the shared Redis configuration.
func NewClient(conf redis.RedisConf) *Client {
	opt := RedisOpt(conf)
	return &Client{inner: asynq.NewClient(opt), inspector: asynq.NewInspector(opt)}
}

// Close releases the inspector and the client. A nil client is tolerated so tests can run without Redis.
func (c *Client) Close() error {
	if c == nil {
		return nil
	}
	var err error
	if c.inspector != nil {
		err = c.inspector.Close()
	}
	if c.inner != nil {
		if closeErr := c.inner.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}

// ActiveWorkers counts running asynq servers that consume the given queue.
// It is used to fail fast when jobs-runner is not running, instead of
// enqueuing a task nobody will ever process.
func (c *Client) ActiveWorkers(queue string) (int, error) {
	if c == nil || c.inspector == nil {
		return 0, fmt.Errorf("asynq inspector is not initialized")
	}
	servers, err := c.inspector.Servers()
	if err != nil {
		return 0, err
	}
	count := 0
	for _, server := range servers {
		if server == nil {
			continue
		}
		if queue == "" {
			count++
			continue
		}
		// A server whose reported queues are unknown cannot be verified as a
		// consumer of this queue, so it does not count; enqueuing to a queue
		// nobody consumes is worse than a transient "no worker" response.
		if _, ok := server.Queues[queue]; ok {
			count++
		}
	}
	return count, nil
}

// EnqueueJob enqueues one job execution with the job's own retry budget and timeout. A non-positive timeout falls back to the asynq default.
func (c *Client) EnqueueJob(ctx context.Context, jobID uint64, triggerType string, maxRetry int, timeoutSeconds int) (*asynq.TaskInfo, error) {
	if c == nil || c.inner == nil {
		return nil, fmt.Errorf("asynq client is not initialized")
	}
	payload, err := json.Marshal(JobPayload{JobID: jobID, TriggerType: triggerType})
	if err != nil {
		return nil, err
	}
	task := asynq.NewTask(TaskScheduledJob, payload)
	opts := []asynq.Option{
		asynq.Queue(DefaultQueue),
		asynq.MaxRetry(maxRetry),
	}
	if timeoutSeconds > 0 {
		opts = append(opts, asynq.Timeout(time.Duration(timeoutSeconds)*time.Second))
	}
	return c.inner.EnqueueContext(ctx, task, opts...)
}

// RequestReconcile asks jobs-runner to re-read the job table after a CRUD change, so a new or edited cron takes effect without waiting for the 60-second tick.
func (c *Client) RequestReconcile(ctx context.Context) (*asynq.TaskInfo, error) {
	if c == nil || c.inner == nil {
		return nil, fmt.Errorf("asynq client is not initialized")
	}
	task := asynq.NewTask(TaskSchedulerSync, nil)
	return c.inner.EnqueueContext(ctx, task, asynq.Queue(DefaultQueue), asynq.MaxRetry(2))
}
