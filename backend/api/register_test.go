package api

import (
	"context"
	"errors"
	"testing"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/platform/eventbus"
	"dragz-admin/backend/platform/notifier"
)

// The subscription must exist before the first HTTP request. Publishing an
// event is the simplest proof: without wireNotifier the bus would silently
// drop it and Publish would return nil.
func TestWireNotifierSubscribesAtStartup(t *testing.T) {
	ctx := &app.Context{Bus: eventbus.New()}
	wireNotifier(ctx)
	if ctx.Notifier == nil {
		t.Fatal("wireNotifier left Context.Notifier nil")
	}

	err := ctx.Bus.Publish(context.Background(), eventbus.Event{
		Name:   notifier.EventName,
		Source: "test",
		Payload: notifier.SendInput{
			Version:   notifier.CurrentVersion,
			Recipient: "nobody@example.com",
		},
	})
	// No database is wired in this unit test; reaching the provider's
	// fail-closed error proves the handler was registered and invoked.
	if !errors.Is(err, notifier.ErrUnavailable) {
		t.Fatalf("Publish error = %v, want ErrUnavailable", err)
	}
}
