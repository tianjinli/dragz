# Backend Norm Gaps and Conventions

**English** | [中文](NORMS_GAP_CN.md)

This document records the decided conventions and the remediation boundaries. For the component inventory see [COMPONENTS.md](COMPONENTS.md); for layering conventions see [CONVENTIONS.md](CONVENTIONS.md).

## Decided Conventions

1. **Data access**: use **sqlc**, do not introduce ent. Currently **a single engine: `postgresql`**.
2. **i18n**: user-visible copy always goes through locale keys (`errors.*` / `common.*`); literal Chinese or English strings in `fail`/`failData`/`HTTPError.Message` are forbidden.
3. **Multi-database extension**: no second database in this round; but logic/httpapi **must not** depend directly on `internal/store/sqlc` generated code — go through repository interfaces so implementations such as MySQL can be added later.

## Multi-database Extension Playbook (Backlog)

Switching databases = switching the store implementation + DDL/queries, not the business layer.

1. Add `migrations/<driver>/` DDL.
2. Add the corresponding `engine` to `sqlc.yaml`, generate into `internal/store/<driver>/sqlc` (or an equivalent path).
3. Implement the same set of `internal/store` repository interfaces.
4. Register the driver via `Database.Driver` + `store.Open(driver, dsn)`.
5. Add a smoke test for that database in CI (login, user CRUD).

### Concrete Blockers (surveyed 2026-10-30, must be resolved before starting)

These are not solved by "switching a driver name"; ordered from largest to smallest effort:

| Blocker | Location | Notes |
|--------|------|------|
| Placeholder dialect | `queries/*.sql` | All use the `$1` form. MySQL needs `?`, which means a dialect version of every query file; sqlc does not convert automatically |
| Migration engine SQL | `plugins/*/migrations/*.sql`, `migrations/001_init.sql` | `BIGSERIAL`, `TIMESTAMPTZ`, `JSONB`, `ON CONFLICT` are all PG-specific |
| Migration serialization | `store.AcquireMigrationLock` | Uses `pg_advisory_lock`. MySQL requires `GET_LOCK()`, and the semantics (timeout, connection binding, release with the connection) are not fully equivalent |
| Nullable JSON columns | `nullRawArray`, `pqtype.NullRawMessage` in `store/sqlc_helpers.go` | Bound to `database/sql` pq types; the MySQL side needs `sql.NullString` |
| Menu cycle detection | `menu_parent_is_descendant` (schema function + recursive CTE) | The sqlc analyzer does not support recursive CTEs, and MySQL 8 CTE syntax differs; must be rewritten at the application layer or reimplemented |
| Case sensitivity and collation | All queries | PG folds identifiers to lowercase and sorts case-sensitively by default; MySQL on Windows is case-insensitive by default, which changes unique index behavior |

Conclusion: this is a "parallel implementation + duplicate SQL" effort, not a configuration item. Stay on a single engine
until there is a real need, and block misconfiguration at startup with the `Database.Driver` whitelist (`config.Validate` and `store.Open` each block once).

## Done / In Progress This Round

| Item | Status |
|----|------|
| Removed old `job-worker` / `jobworker`; unified as `cmd/jobs-runner` + `internal/jobs/runner` | Done |
| Docker Compose mounts only `001_init.sql` (including `token_version`); existing volumes missing the column need a one-time ALTER or rebuild | Done |
| `token_version` (JWT invalidation), `GET /api/ready`, secret validation / `admin-api.example.yaml` | Done |
| `Database.Driver` (defaults to postgres) | Done |
| i18n: all httpapi `fail`/`failData`/`HTTPError.Message` converted to keys; zh/en locale files aligned; auth/lockout/feature + remaining handlers | Done |
| sqlc single engine, schema alignment, repository interfaces | **Done** |
| Full pass over fail copy wording, MySQL implementation | Backlog |

### sqlc Migration Progress

- **Schema**: `schema/tables.sql` is synced from the `migrations/001_init.sql` DDL (including `token_version`, `menu_parent_is_descendant`).
- **Migrated to sqlc**: users / refresh_tokens / auth_rbac / jobs / settings / oauth / dictionaries / roles·menus·menu_buttons / organizations·positions / notices / files / webhooks / audit (including list filtering) / data_scope. Optional filters uniformly use `sqlc.narg` (see the `ListRolesFiltered` pattern).
- **Menu cycle detection**: the sqlc analyzer does not support recursive CTEs, so the schema function `menu_parent_is_descendant` is used (only in `001_init.sql` / `schema/tables.sql`); query `MenuParentIsDescendant`.
- **Boundaries**: httpapi does **not** import `internal/store/sqlc`; `repos.go` extends `AuthStore` / `UserStore` / `JobStore` / `SettingsStore`. Business SQL lives only in `queries/`.

## Secrets

- `etc/*.yaml` in the repository is for local development only; production must inject via environment/mounts.
- See `etc/admin-api.example.yaml` for an example.
- At startup, if a known default secret is detected and development mode is not explicitly allowed, startup is refused (see `config.Validate`).
