// Package eventbus is the in-process pub/sub used for cross-plugin
// collaboration. Plugins must not import each other; they publish and
// subscribe here (or call platform interfaces) instead.
package eventbus

import (
	"context"
	"sync"
)

// Event is one message on the bus.
type Event struct {
	// Name is the event type, e.g. "example.pinged".
	Name string
	// Source is the producing plugin id (or "platform" / "system").
	Source string
	// Payload is an opaque value; subscribers type-assert what they need.
	Payload any
}

// Handler handles one event. Returning an error does not stop other handlers;
// Publish aggregates errors after all handlers run.
type Handler func(ctx context.Context, ev Event) error

// Bus is a process-local event bus. It is safe for concurrent use.
type Bus struct {
	mu   sync.RWMutex
	subs map[string][]Handler
}

// New returns an empty bus.
func New() *Bus {
	return &Bus{subs: make(map[string][]Handler)}
}

// Subscribe registers h for events named name. Handlers run synchronously in
// registration order on Publish. Empty name is ignored.
func (b *Bus) Subscribe(name string, h Handler) {
	if b == nil || name == "" || h == nil {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[name] = append(b.subs[name], h)
}

// Publish delivers ev to every handler subscribed to ev.Name. Nil bus or empty
// name is a no-op. Delivery continues after a handler failure; errors are joined.
func (b *Bus) Publish(ctx context.Context, ev Event) error {
	if b == nil || ev.Name == "" {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	b.mu.RLock()
	handlers := append([]Handler(nil), b.subs[ev.Name]...)
	b.mu.RUnlock()

	var errs []error
	for _, h := range handlers {
		if err := h(ctx, ev); err != nil {
			errs = append(errs, err)
		}
	}
	return joinErrors(errs)
}

func joinErrors(errs []error) error {
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		msg := errs[0].Error()
		for _, err := range errs[1:] {
			msg += "; " + err.Error()
		}
		return &multiError{msg: msg}
	}
}

type multiError struct{ msg string }

func (e *multiError) Error() string { return e.msg }
