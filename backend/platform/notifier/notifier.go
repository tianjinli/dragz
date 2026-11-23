// Package notifier is the platform seam for outbound notifications.
//
// Plugins must not import plugins/notify. A plugin that needs to send mail or
// SMS depends on this package, calls Notifier.Send through the shared
// app.Context, and declares DependsOn: []string{PluginID} so plugincenter
// keeps it off whenever the notification provider is off. The event bus name
// EventName is kept as a second entry point for plugins that prefer fire-and-
// forget delivery; both paths end up in the same implementation.
//
// This package deliberately imports only the standard library. That keeps it
// usable from internal/app without creating an import cycle.
package notifier

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	// PluginID is the provider plugin other plugins must depend on.
	PluginID = "notify"
	// EventName is the eventbus topic carrying a SendInput payload.
	EventName = "notify.send"
	// CurrentVersion is the version of SendInput accepted by this build.
	CurrentVersion = 1
)

var (
	// ErrUnavailable means no active provider can deliver the message. It is
	// returned when the notify plugin is disabled or not installed, and by
	// Noop when the wiring layer has not installed a provider at all.
	ErrUnavailable = errors.New("notifier: no active notification provider")
	// ErrUnsupportedVersion means the caller sent a payload from a future
	// (or otherwise incompatible) contract version.
	ErrUnsupportedVersion = errors.New("notifier: unsupported payload version")
)

// SendInput is the stable, versioned request contract. Recipient carries the
// destination in the form the chosen channel expects: an email address for
// SMTP and a phone number for SMS. ChannelType and ChannelID are alternatives;
// when both are empty the provider picks its first enabled channel.
type SendInput struct {
	// Version is optional on the wire; absent means CurrentVersion so older
	// callers and the legacy event payload keep working.
	Version      int            `json:"version,omitempty"`
	ChannelType  string         `json:"channelType,omitempty"`
	ChannelID    uint64         `json:"channelId,omitempty"`
	TemplateCode string         `json:"templateCode,omitempty"`
	Recipient    string         `json:"recipient"`
	Subject      string         `json:"subject,omitempty"`
	Content      string         `json:"content,omitempty"`
	Vars         map[string]any `json:"vars,omitempty"`
}

// Validate checks the fields every delivery needs. Channel selection and
// template resolution are provider concerns and are intentionally left out.
func (in SendInput) Validate() error {
	if in.Version != 0 && in.Version != CurrentVersion {
		return fmt.Errorf("%w: %d", ErrUnsupportedVersion, in.Version)
	}
	if strings.TrimSpace(in.Recipient) == "" {
		return errors.New("notifier: recipient is required")
	}
	return nil
}

// SendResult is the stable response contract. Sent is false only when an
// error is also returned; keeping the flag makes the JSON shape self-describing.
type SendResult struct {
	Sent        bool   `json:"sent"`
	ChannelType string `json:"channelType,omitempty"`
}

// Notifier is the narrow capability other plugins consume.
type Notifier interface {
	Send(ctx context.Context, in SendInput) (SendResult, error)
}

// Noop is the default installed into app.Context before the wiring layer has
// located the real provider. It fails closed instead of silently dropping a
// message, so a missing provider is visible to the caller.
type Noop struct{}

var _ Notifier = Noop{}

// Send reports that no provider is available.
func (Noop) Send(context.Context, SendInput) (SendResult, error) {
	return SendResult{}, ErrUnavailable
}

// DecodePayload turns either a typed SendInput or an eventbus-shaped map into
// the contract. Accepting maps keeps events published before this version
// decodable, while typed values avoid a needless marshal/unmarshal round trip.
func DecodePayload(payload any) (SendInput, error) {
	var in SendInput
	switch value := payload.(type) {
	case SendInput:
		in = value
	case *SendInput:
		if value == nil {
			return in, errors.New("notifier: nil payload")
		}
		in = *value
	default:
		raw, err := json.Marshal(payload)
		if err != nil {
			return in, fmt.Errorf("notifier: marshal payload: %w", err)
		}
		if err := json.Unmarshal(raw, &in); err != nil {
			return in, fmt.Errorf("notifier: unmarshal payload: %w", err)
		}
	}
	if in.Version == 0 {
		in.Version = CurrentVersion
	}
	if in.Version != CurrentVersion {
		return in, fmt.Errorf("%w: %d", ErrUnsupportedVersion, in.Version)
	}
	return in, nil
}
