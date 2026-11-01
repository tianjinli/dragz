package jobs

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"

	"dragz-admin/backend/internal/store"
)

type fakeRepo struct {
	enabled bool
	job     store.ScheduledJob
	err     error
}

func (f fakeRepo) FeatureEnabled(context.Context, string, bool) (bool, error) {
	return f.enabled, f.err
}

func (f fakeRepo) JobByID(context.Context, uint64) (store.ScheduledJob, error) {
	return f.job, f.err
}

type fakeClient struct {
	workers    int
	workersErr error
	enqueued   bool
	jobID      uint64
	trigger    string
	maxRetry   int
	timeout    int
}

func (f *fakeClient) EnqueueJob(_ context.Context, jobID uint64, triggerType string, maxRetry, timeoutSeconds int) (*asynq.TaskInfo, error) {
	f.enqueued, f.jobID, f.trigger, f.maxRetry, f.timeout = true, jobID, triggerType, maxRetry, timeoutSeconds
	return &asynq.TaskInfo{ID: "task-1"}, nil
}

func (f *fakeClient) RequestReconcile(context.Context) (*asynq.TaskInfo, error) { return nil, nil }
func (f *fakeClient) ActiveWorkers(string) (int, error)                         { return f.workers, f.workersErr }
func (f *fakeClient) Close() error                                              { return nil }

func TestCanRun(t *testing.T) {
	enabled := store.ScheduledJob{Status: 1}
	disabled := store.ScheduledJob{Status: 0}
	for _, tt := range []struct {
		name    string
		job     store.ScheduledJob
		trigger string
		want    bool
	}{
		{"enabled scheduled", enabled, "scheduled", true},
		{"disabled scheduled", disabled, "scheduled", false},
		{"enabled manual", enabled, "manual", true},
		{"disabled manual", disabled, "manual", true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := CanRun(tt.job, tt.trigger); got != tt.want {
				t.Fatalf("CanRun(%v, %q) = %v, want %v", tt.job.Status, tt.trigger, got, tt.want)
			}
		})
	}
}

func newTestService(repo fakeRepo, client *fakeClient) *Service {
	return &Service{repo: repo, client: client}
}

func TestRunNowFailsWhenFeatureDisabled(t *testing.T) {
	client := &fakeClient{workers: 1}
	service := newTestService(fakeRepo{enabled: false}, client)
	if _, err := service.RunNow(context.Background(), 1); err == nil {
		t.Fatal("expected error when scheduler feature is disabled")
	}
	if client.enqueued {
		t.Fatal("task must not be enqueued when feature is disabled")
	}
}

func TestRunNowFailsWithoutWorker(t *testing.T) {
	client := &fakeClient{workers: 0}
	service := newTestService(fakeRepo{enabled: true, job: store.ScheduledJob{ID: 7, Status: 1}}, client)
	_, err := service.RunNow(context.Background(), 7)
	if !errors.Is(err, ErrNoWorker) {
		t.Fatalf("expected ErrNoWorker, got %v", err)
	}
	if client.enqueued {
		t.Fatal("task must not be enqueued when no worker is running")
	}
}

func TestRunNowEnqueuesManualTask(t *testing.T) {
	client := &fakeClient{workers: 2}
	job := store.ScheduledJob{ID: 9, Status: 0, MaxRetries: 3, TimeoutSeconds: 120}
	service := newTestService(fakeRepo{enabled: true, job: job}, client)

	result, err := service.RunNow(context.Background(), 9)
	if err != nil {
		t.Fatalf("RunNow returned error: %v", err)
	}
	if !result.Submitted || result.JobID != 9 || result.TaskID != "task-1" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !client.enqueued || client.jobID != 9 || client.trigger != "manual" || client.maxRetry != 3 || client.timeout != 120 {
		t.Fatalf("unexpected enqueue args: %+v", client)
	}
}

func TestRunNowPropagatesWorkerProbeError(t *testing.T) {
	client := &fakeClient{workersErr: errors.New("redis down")}
	service := newTestService(fakeRepo{enabled: true, job: store.ScheduledJob{ID: 1, Status: 1}}, client)
	if _, err := service.RunNow(context.Background(), 1); err == nil {
		t.Fatal("expected worker probe error to propagate")
	}
}
