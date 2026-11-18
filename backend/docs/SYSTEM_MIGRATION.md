# system plugin vs internal/httpapi migration checklist

**中文** | [中文](SYSTEM_MIGRATION_CN.md)

`plugins/system` currently re-exports `httpapi.Routes()`. That is transitional (P6). Goal: core routes stay plugin-declared; handlers leave the httpapi “god package”.

## Boundaries

| Area | Today | Target |
|------|-------|--------|
| Route table | `system` → `httpapi.Routes()` | Keep declarations in system (or split packages); handlers by domain |
| Middleware | `httpapi.Hooks` | Stay in httpapi or move to platform; kernel only sees `plugin.Hooks` |
| Orchestration | Partial `httpapi/logic` | Handlers must not call store directly |
| Import exception | Only `plugins/system` may import `internal/httpapi` | Remove after migration; update `check-imports.sh` |

## Disable gate (already shipped)

Chain: `Localize → Gate → Authenticate → …`. Disabled plugin routes must return **404**, never 401-before-missing. See `internal/httpapi/plugin.go`.

## Scope note

P0–P1 does **not** require finishing this migration; this doc freezes the contract so new code does not grow httpapi further without ownership.
