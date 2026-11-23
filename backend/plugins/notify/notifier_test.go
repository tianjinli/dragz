package notify

import (
	"context"
	"errors"
	"testing"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pluginstate"
	"dragz-admin/backend/platform/eventbus"
	"dragz-admin/backend/platform/notifier"
)

type disabledRuntime struct{}

func (disabledRuntime) Enabled(string) bool { return false }
func (disabledRuntime) Builtin(string) bool { return false }
func (disabledRuntime) List(context.Context) ([]pluginstate.Info, error) {
	return nil, nil
}
func (disabledRuntime) Install(context.Context, string) error   { return nil }
func (disabledRuntime) Enable(context.Context, string) error    { return nil }
func (disabledRuntime) Disable(context.Context, string) error   { return nil }
func (disabledRuntime) Uninstall(context.Context, string) error { return nil }

func TestServiceSendFailsWhenPluginDisabled(t *testing.T) {
	svc := NewNotifier(&app.Context{Plugins: disabledRuntime{}})
	_, err := svc.Send(context.Background(), notifier.SendInput{
		Version:   notifier.CurrentVersion,
		Recipient: "nobody@example.com",
	})
	if !errors.Is(err, notifier.ErrUnavailable) {
		t.Fatalf("Send error = %v, want ErrUnavailable", err)
	}
}

func TestServiceSendValidatesInput(t *testing.T) {
	svc := NewNotifier(nil)
	_, err := svc.Send(context.Background(), notifier.SendInput{Version: notifier.CurrentVersion})
	if err == nil {
		t.Fatal("Send accepted an empty recipient")
	}
}

func TestServiceHandleEventDecodesLegacyPayload(t *testing.T) {
	svc := NewNotifier(nil)
	err := svc.HandleEvent(context.Background(), eventbus.Event{
		Name:    notifier.EventName,
		Source:  "test",
		Payload: map[string]any{"recipient": "nobody@example.com"},
	})
	if !errors.Is(err, notifier.ErrUnavailable) {
		t.Fatalf("HandleEvent error = %v, want ErrUnavailable", err)
	}
}

func TestServiceHandleEventRejectsFutureVersion(t *testing.T) {
	svc := NewNotifier(nil)
	err := svc.HandleEvent(context.Background(), eventbus.Event{
		Name:   notifier.EventName,
		Source: "test",
		Payload: map[string]any{
			"version":   notifier.CurrentVersion + 1,
			"recipient": "nobody@example.com",
		},
	})
	if !errors.Is(err, notifier.ErrUnsupportedVersion) {
		t.Fatalf("HandleEvent error = %v, want ErrUnsupportedVersion", err)
	}
}
