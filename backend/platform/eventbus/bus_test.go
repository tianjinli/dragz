package eventbus

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
)

func TestPublishSubscribe(t *testing.T) {
	bus := New()
	var hits atomic.Int32
	bus.Subscribe("demo.tick", func(context.Context, Event) error {
		hits.Add(1)
		return nil
	})
	if err := bus.Publish(context.Background(), Event{Name: "demo.tick", Source: "test"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits=%d want 1", hits.Load())
	}
}

func TestPublishContinuesAfterHandlerError(t *testing.T) {
	bus := New()
	var hits atomic.Int32
	bus.Subscribe("demo.err", func(context.Context, Event) error {
		hits.Add(1)
		return errors.New("boom")
	})
	bus.Subscribe("demo.err", func(context.Context, Event) error {
		hits.Add(1)
		return nil
	})
	err := bus.Publish(context.Background(), Event{Name: "demo.err", Source: "test"})
	if err == nil {
		t.Fatal("expected error")
	}
	if hits.Load() != 2 {
		t.Fatalf("hits=%d want 2", hits.Load())
	}
}

func TestNilBusNoop(t *testing.T) {
	var bus *Bus
	if err := bus.Publish(context.Background(), Event{Name: "x"}); err != nil {
		t.Fatalf("nil bus publish: %v", err)
	}
	bus.Subscribe("x", func(context.Context, Event) error { return nil })
}
