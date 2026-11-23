package notifier

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestNoopFailsClosed(t *testing.T) {
	_, err := Noop{}.Send(context.Background(), SendInput{Recipient: "nobody@example.com"})
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Noop.Send error = %v, want ErrUnavailable", err)
	}
}

func TestDecodePayloadAcceptsTypedAndLegacyMap(t *testing.T) {
	want := SendInput{Version: CurrentVersion, ChannelType: "smtp", Recipient: "a@example.com", Subject: "hi"}
	cases := map[string]any{
		"typed":     want,
		"typed ptr": &want,
		"map": map[string]any{
			"channelType": "smtp",
			"recipient":   "a@example.com",
			"subject":     "hi",
		},
	}

	for name, payload := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := DecodePayload(payload)
			if err != nil {
				t.Fatalf("DecodePayload: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("DecodePayload = %+v, want %+v", got, want)
			}
		})
	}
}

func TestDecodePayloadRejectsFutureVersion(t *testing.T) {
	_, err := DecodePayload(map[string]any{"version": CurrentVersion + 1, "recipient": "a@example.com"})
	if !errors.Is(err, ErrUnsupportedVersion) {
		t.Fatalf("DecodePayload error = %v, want ErrUnsupportedVersion", err)
	}
}

func TestValidateRequiresRecipient(t *testing.T) {
	if err := (SendInput{Version: CurrentVersion}).Validate(); err == nil {
		t.Fatal("Validate accepted an empty recipient")
	}
	if err := (SendInput{Version: CurrentVersion, Recipient: "a@example.com"}).Validate(); err != nil {
		t.Fatalf("Validate rejected a valid input: %v", err)
	}
}
