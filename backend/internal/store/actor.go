package store

import (
	"context"

	"dragz-admin/backend/internal/pkg/ctxkey"
)

// ActorUnknown is what created_by/updated_by hold when the write has no human
// behind it: startup bootstrap, seeds, plugin migrations, scheduled jobs and
// payment callbacks. The columns deliberately have no foreign key, so 0 can
// never collide with a real user row (ids start at 1).
const ActorUnknown uint64 = 0

// actorID is the single read point for "who is performing this write". Every
// audit column takes its value from here so the rule stays uniform: a request
// carries ctxkey.UserID, everything else records ActorUnknown.
func actorID(ctx context.Context) uint64 {
	if id, ok := ctxkey.UserIDFrom(ctx); ok {
		return id
	}
	return ActorUnknown
}

// ActorID is the plugin-facing alias of actorID. A plugin owns its tables and
// writes them with plain database/sql, so it cannot reuse the sqlc queries
// here, but the audit rule must stay identical.
func ActorID(ctx context.Context) uint64 {
	return actorID(ctx)
}
