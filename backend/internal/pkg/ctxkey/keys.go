// Package ctxkey defines the request-scoped context keys shared by the
// middleware chain and the handlers.
package ctxkey

// key is an unexported type so a context key from this package can never
// collide with one written by another package.
type key int

// Request-scoped context keys. They are set by the middleware chain and read by
// the handlers; nothing else should write them.
const (
	// UserID holds the authenticated user id.
	UserID key = iota + 1
	// Username holds the authenticated username, kept for audit logging.
	Username
	// TokenScopes holds the scopes of an API-token caller.
	TokenScopes
	// Translator holds the request-scoped localizer.
	Translator
)
