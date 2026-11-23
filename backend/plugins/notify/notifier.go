package notify

import (
	"context"
	"fmt"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/platform/eventbus"
	"dragz-admin/backend/platform/notifier"
)

// Service adapts the plugin's delivery dispatcher to the platform seam. The
// wiring layer builds one instance at startup and puts it on app.Context;
// other plugins never import this package.
type Service struct {
	appCtx *app.Context
}

var _ notifier.Notifier = (*Service)(nil)

// NewNotifier returns the provider bound to the live application context.
// It does no I/O: schema state and plugin state are read on every Send so a
// runtime enable/disable takes effect without a restart.
func NewNotifier(appCtx *app.Context) *Service {
	return &Service{appCtx: appCtx}
}

// Send implements notifier.Notifier. It fails closed when the provider is
// absent, has no database, or is currently disabled.
func (s *Service) Send(ctx context.Context, in notifier.SendInput) (notifier.SendResult, error) {
	if err := in.Validate(); err != nil {
		return notifier.SendResult{}, err
	}
	if s == nil || s.appCtx == nil || s.appCtx.DB == nil {
		return notifier.SendResult{}, fmt.Errorf("%w: notify has no database", notifier.ErrUnavailable)
	}
	// A nil runtime means a test or a pre-plugin binary: treat it as enabled,
	// matching pluginGate's behaviour. A live runtime is authoritative.
	if s.appCtx.Plugins != nil && !s.appCtx.Plugins.Enabled(notifier.PluginID) {
		return notifier.SendResult{}, fmt.Errorf("%w: notify plugin is disabled", notifier.ErrUnavailable)
	}
	return dispatch(ctx, s.appCtx, in)
}

// HandleEvent is the eventbus entry point. The payload is decoded into the
// same typed contract used by Send, so HTTP calls and events cannot drift.
func (s *Service) HandleEvent(ctx context.Context, ev eventbus.Event) error {
	in, err := notifier.DecodePayload(ev.Payload)
	if err != nil {
		return err
	}
	_, err = s.Send(ctx, in)
	return err
}
