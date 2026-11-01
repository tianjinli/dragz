package jobasynq

import (
	"errors"
	"testing"

	"github.com/hibiken/asynq"
)

type fakeServerLister struct {
	servers []*asynq.ServerInfo
	err     error
}

func (f fakeServerLister) Servers() ([]*asynq.ServerInfo, error) { return f.servers, f.err }
func (f fakeServerLister) Close() error                          { return nil }

func TestActiveWorkersCountsQueueConsumers(t *testing.T) {
	client := &Client{inspector: fakeServerLister{servers: []*asynq.ServerInfo{
		{ID: "a", Queues: map[string]int{"default": 3}},
		{ID: "b", Queues: map[string]int{"default": 1, "critical": 2}},
		{ID: "c", Queues: map[string]int{"critical": 1}},
		nil,
	}}}
	count, err := client.ActiveWorkers("default")
	if err != nil {
		t.Fatalf("ActiveWorkers returned error: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 workers on default queue, got %d", count)
	}
	all, err := client.ActiveWorkers("")
	if err != nil {
		t.Fatalf("ActiveWorkers(all) returned error: %v", err)
	}
	if all != 3 {
		t.Fatalf("expected 3 servers, got %d", all)
	}
}

func TestActiveWorkersSkipsUnknownQueues(t *testing.T) {
	// A server that does not report its queues cannot be verified as a
	// consumer of a specific queue, so it must not count for one.
	client := &Client{inspector: fakeServerLister{servers: []*asynq.ServerInfo{
		{ID: "known", Queues: map[string]int{"default": 1}},
		{ID: "unknown", Queues: map[string]int{}},
	}}}
	count, err := client.ActiveWorkers("default")
	if err != nil {
		t.Fatalf("ActiveWorkers returned error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 known default consumer, got %d", count)
	}
}

func TestActiveWorkersPropagatesError(t *testing.T) {
	client := &Client{inspector: fakeServerLister{err: errors.New("redis down")}}
	if _, err := client.ActiveWorkers("default"); err == nil {
		t.Fatal("expected inspector error to propagate")
	}
}
