# Deployment and operations scripts

**English** | [中文](README_CN.md)

All one-shot entry points live in `deploy/`. Every script is bash: run it with Git Bash on Windows; it works on Linux/macOS too.

## Local development

| Goal | One command |
|------|----------|
| Full stack up (frontend build + postgres/redis/api/worker/nginx) | `bash deploy/up.sh` |
| Only the backend and its dependencies (skips the frontend build) | `bash deploy/up.sh --api-only` |
| Only postgres + redis, run the backend with `go run` | `bash deploy/dev-infra.sh` |
| Status + health probes | `bash deploy/ps.sh` |
| Follow logs (optionally specify a service) | `bash deploy/logs.sh [admin-api]` |
| Stop (keeps data volumes) | `bash deploy/down.sh` |
| Stop, wipe the database and rebuild | `bash deploy/reset.sh` |

Once up:

- Admin UI <http://127.0.0.1:5279>
- API health check <http://127.0.0.1:5280/api/health>
- Default account `admin` / `Dragz!7xZ` (seeded by `migrations/001_system_init.sql`; change the password on first login)

`up.sh` is idempotent: re-running it only rebuilds the image layers that changed.

### Ports already in use

If 5432 / 6379 / 5280 / 5279 are already taken by other services (on this machine they are permanently occupied by other Postgres/Redis containers), remap the ports with an override file instead of editing the compose files in the repo:

```bash
cp deploy/ports.override.example.yml deploy/ports.override.yml
DRAGZ_EXTRA_COMPOSE="$PWD/deploy/ports.override.yml" bash deploy/up.sh
```

`ports.override.example.yml` uses `!override`, which **replaces** the base file's `ports` list; a normal merge appends, so the conflicting port mapping is still there and nothing starts.

`up.sh` / `ps.sh` read the real mapped ports back with `docker compose port`, so the health probes and the final URLs follow automatically.

> Note: containers still talk over the compose internal network (`postgres:5432`, `redis:6379`); the override only affects host ports.

## Production (k3s)

The production pipeline is unchanged; the entry points are in `deploy/k3s/`:

| Goal | Command |
|------|------|
| Build and push images | `pwsh deploy/k3s/build-and-push.ps1 -Tag vX.Y.Z` |
| Upload deployment files and roll out | `bash deploy/k3s/deploy-manual.sh [key.pem]` |
| Host postgres/redis | `docker compose --env-file deploy/.env.prod -f deploy/docker-compose.prod-infra.yml up -d` |

Production secrets go in `deploy/.env.prod` (gitignored); field descriptions are in `deploy/.env.prod.example`. Detailed steps are in [`k3s/README.md`](k3s/README.md).

## Conventions

- Scripts share `lib.sh`, which is only `source`d, never run on its own.
- Every compose call passes `--project-directory deploy`, so **it makes no difference which directory you run a script from**.
- `curl` always gets `--noproxy '*'`: on Windows the system proxy intercepts `127.0.0.1`, which makes health checks report false failures.
- Destructive operations (`down.sh --volumes`, `reset.sh`) require manually typing `yes` to confirm.

## Three deliberate design decisions

1. **compose pins `name: dragz`**
   Compose defaults to the directory name as the project name, and `deploy/` is a directory name many projects use. `docker compose down` deletes containers by project label, so a name collision takes other projects' containers with it (this happened once on this machine). With an explicit name they no longer interfere.
2. **No `--remove-orphans`**
   For the same reason, orphan cleanup runs within the project scope; across projects it deletes the wrong things. The cost is that after renaming a service the old container lingers and needs a manual `docker rm`.
3. **Postgres init SQL baked into the image** (`deploy/Dockerfile.postgres`)
   It used to be a bind mount of `backend/migrations/*.sql`. When Docker Desktop's File Sharing does not include the repo path the mount fails outright (on this machine not even `C:\Users\snail` is shared), and one-shot startup breaks. Baked into the image it depends on no host path configuration, and first-time database creation behaves exactly as with the mount.

The frontend output `frontend/admin/dist` is cleared by the scripts before the build: it is a pure build artifact, and letting the bundler clear the directory itself triggers filesystem protection in some managed environments.
