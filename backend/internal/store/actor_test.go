package store

import (
	"context"
	"testing"

	"dragz-admin/backend/internal/pkg/ctxkey"
)

// actorID is the only place audit columns read their value, so its two cases
// are the contract: a request names the user, everything else records 0.
func TestActorIDFallsBackToUnknown(t *testing.T) {
	if got := actorID(context.Background()); got != ActorUnknown {
		t.Fatalf("background context: got %d, want %d", got, ActorUnknown)
	}
	if got := actorID(nil); got != ActorUnknown {
		t.Fatalf("nil context: got %d, want %d", got, ActorUnknown)
	}
}

func TestActorIDReadsRequestUser(t *testing.T) {
	ctx := ctxkey.WithUserID(context.Background(), 42)
	if got := actorID(ctx); got != 42 {
		t.Fatalf("got %d, want 42", got)
	}
}

// A zero id is not a user: it must stay indistinguishable from "no context".
func TestActorIDTreatsZeroAsUnknown(t *testing.T) {
	ctx := ctxkey.WithUserID(context.Background(), 0)
	if got := actorID(ctx); got != ActorUnknown {
		t.Fatalf("got %d, want %d", got, ActorUnknown)
	}
}
