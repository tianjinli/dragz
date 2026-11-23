package store

import "context"

// ensureTenantSchema is a no-op. Tenant DDL lives in
// migrations/013_multitenant.sql and in bootstrapStatements. Bootstrap must
// SET app.rls_bypass=on before it runs (see PluginRepository.Bootstrap).
// The function remains so a caller written against the earlier hook still
// builds, without applying a second schema.
func ensureTenantSchema(context.Context, *DB) error {
	return nil
}
