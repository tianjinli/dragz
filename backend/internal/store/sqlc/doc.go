// Package sqlcstore holds the code sqlc generates from backend/queries/*.sql
// and the schema under backend/schema. Nothing in this package is written by
// hand: editing a *_sql.go file here is always wrong, because the next
// `sqlc generate` silently reverts it.
//
// To change a query, edit the .sql file and regenerate:
//
//	sqlc generate
//
// The rest of the backend talks to it only through internal/store, which wraps
// these calls and converts them into the domain errors declared in
// internal/store/errors.go. Hand-written helpers shared by those wrappers live
// in internal/store/sqlc_helpers.go, not here.
//
// This doc.go is the one file in the directory that sqlc does not own, so it
// survives regeneration.
package sqlcstore
