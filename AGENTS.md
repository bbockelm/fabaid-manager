# Agent Notes — FabAID Manager

Last updated: 2026-07-11

## Project Purpose

NSF grant/project tracking web application. Tracks effort/personnel by WBS area, planned budgets, subawards, invoice PDFs, and annual statements of work. The user (bbockelm) is a PI or project manager on NSF-funded research.

## Tech Stack

| Component | Technology | Notes |
|-----------|------------|-------|
| Backend | Go 1.22, chi/v5 router | `cmd/server/main.go` entry point |
| Database | PostgreSQL 16 | pgx/v5 driver, goose/v3 migrations |
| Object Storage | S3 (MinIO for dev) | minio-go/v7; stores invoice PDFs, signed SOWs |
| Frontend | Next.js 14 (App Router) | React 18, TanStack React Query v5, Tailwind CSS 3 |
| Dev Environment | VS Code DevContainer | docker-compose: app + postgres + minio |

## Project Layout

```
cmd/server/main.go          — Go server entry point
internal/config/             — envconfig-based configuration
internal/db/db.go            — pgxpool connection
internal/db/migrate.go       — goose migration runner
internal/db/queries.go       — All SQL queries (hand-written, no ORM)
internal/db/migrations/      — SQL migration files
internal/models/models.go    — Go structs with JSON tags
internal/storage/storage.go  — S3 upload/download/delete
internal/router/router.go    — chi routes + CORS + middleware
internal/handlers/handlers.go — REST handlers for all entities
internal/handlers/backup.go  — Backup endpoint (tar.gz of DB + S3 docs)
internal/frontend/embed.go   — //go:embed (build tag: embed_frontend)
internal/frontend/embed_dev.go — no-op stub (build tag: !embed_frontend)
internal/frontend/handler.go — SPA file server with index.html fallback
frontend/src/app/            — Next.js App Router pages
frontend/src/components/     — React components (Sidebar so far)
frontend/src/lib/api.ts      — Typed API client for all endpoints
```

## Database Schema (migration 001)

Tables: `grants`, `wbs_areas`, `personnel`, `budget_items`, `subawards`, `invoices`, `documents`, `statements_of_work`. All have UUID primary keys, timestamps, and foreign keys back to grants (or subawards for invoices). See `internal/db/migrations/001_initial_schema.sql`.

## Current State of the Code

### What's done (scaffolding)
- Full backend: config, DB layer, migrations, S3 storage, REST API, backup handler
- Full frontend scaffold: dashboard, grants list, grant detail, backup page, API client
- DevContainer with Postgres + MinIO
- Production Dockerfile (multi-stage), Kubernetes manifests
- Dev-mode all-in-one Dockerfile with supervisord
- Makefile with all common targets
- README with full docs

### What has NOT been done yet
- `go.sum` does not exist — run `go mod tidy` before building
- `npm install` has not been run in `frontend/`
- No tests exist (Go or frontend)
- No authentication/authorization middleware (OIDC config fields exist but aren't wired)
- No budget/expenditure forecasting logic
- No auto-generation of SOW documents
- Frontend pages are functional but minimal — no edit/delete UI for most entities
- No form validation beyond basic HTML
- No error boundary or loading skeleton components

## Important Patterns

- **Queries**: All in `internal/db/queries.go` as methods on a `Queries` struct wrapping `*pgxpool.Pool`. Raw SQL, no ORM.
- **Handlers**: All in `internal/handlers/handlers.go` as methods on a `Handler` struct holding config, queries, and storage.
- **File uploads**: Multipart form, 50MB max. Files go to S3 via `storage.Upload()`, metadata stored in `documents` table.
- **API proxy**: Next.js `next.config.js` rewrites `/api/*` → `http://localhost:8080/api/*` in dev.
- **CORS**: Allows `localhost:3000` and `localhost:8080` in dev (see `router.go`).
- **Embedded frontend**: Production builds use `-tags embed_frontend` to embed the Next.js static export (`output: 'export'`) into the Go binary. The Go server serves the SPA for all non-API routes via `internal/frontend/handler.go`. In dev, the frontend is NOT embedded; the separate Next.js dev server handles it.
- **Embedded migrations**: DB migrations are embedded via `//go:embed migrations/*.sql` in `internal/db/migrations_embed.go` (`MigrationsFS`) and applied by `db.RunMigrations` (`internal/db/migrate.go`), which is called at server startup from `cmd/server/main.go`. **goose is built into the binary — do NOT run standalone goose.** Just starting the server (`make dev-backend` / running the built binary) applies all pending migrations automatically. The `make migrate*` targets and the `goose ... up` line in `.devcontainer/post-create.sh` are legacy/optional; adding a new `internal/db/migrations/NNN_*.sql` file and (re)starting the server is all that's needed. Follow the existing `-- +goose Up` / `-- +goose Down` format, numbering sequentially after the highest existing file in `internal/db/migrations/`.
- **Build targets**: `make build-prod` builds the Next.js static export, copies it to `internal/frontend/dist/`, builds Go with embed tag, then cleans up. `make build-backend` builds without frontend (dev).

## Dev Environment — how Postgres & MinIO are provided

The devcontainer is a **docker-compose stack** (`.devcontainer/docker-compose.yml`), not a single container. Four services on one compose network:

- **`app`** — the dev container you work in (`service: app` in `devcontainer.json`). Go 1.22 + Node 20 + docker-in-docker features.
- **`db`** — `postgres:16-alpine`, user/pass/db all `fabaid`, with a `pg_isready` healthcheck; `app` waits on `condition: service_healthy`.
- **`minio`** — `minio/minio:latest`, root user/pass `minioadmin`, API on `:9000`, console on `:9001`.
- **`minio-init`** — one-shot `minio/mc` job that creates the `fabaid-documents` bucket, then exits.

**Reach them by compose service hostname, not `localhost`.** The `app` container talks to `db:5432` and `minio:9000`; these are injected into `app`'s environment by compose, so the running server picks them up automatically:

```
DATABASE_URL=postgres://fabaid:fabaid@db:5432/fabaid?sslmode=disable
S3_ENDPOINT=http://minio:9000   S3_BUCKET=fabaid-documents
S3_ACCESS_KEY=minioadmin        S3_SECRET_KEY=minioadmin   S3_USE_PATH_STYLE=true
```

The `Makefile` defaults to `localhost:5432` / `localhost:9000`, but those only work from the **host machine** (ports are published) — inside `app`, rely on the compose-provided env vars (host `db` / `minio`). Ports 8080/3000/5432/9000 are forwarded to the host. If `getent hosts db` doesn't resolve or `pg_isready -h db` fails, the compose stack isn't up in this session (in that case you can build/vet/typecheck but not run live). MinIO client init in `storage.go` strips the `http://` scheme before `minio.New()` (expects `host:port`).

## Known Issues / Gotchas

2. **MinIO client initialization**: `storage.go` strips the `http://` or `https://` scheme from the endpoint before passing to `minio.New()` — the minio-go client expects just `host:port`.

## User Preferences

- The user's web developer prefers **Next.js** (this drove the frontend choice).
- The user wants **Kubernetes deployment with SSO** for production.
- The user wants a **backup dump** (tarball of PDFs + DB) and the ability to run in **development mode from a backup** in a single container.

## How to Run

```bash
# In devcontainer:
make dev            # starts backend (air hot-reload) + frontend (next dev)

# Or separately:
make dev-backend    # just Go with air
make dev-frontend   # just Next.js

# Migrations:
make migrate        # apply
make migrate-down   # rollback one
make migrate-status # check status
```

## Suggested Next Steps (in rough priority)

1. Run `go mod tidy` and verify the backend compiles
2. Run `npm install` in `frontend/` and verify Next.js builds
3. Add edit/delete UI for grants, WBS areas, personnel, subawards
4. Add form validation (both client and server side)
5. Wire up OIDC authentication middleware
6. Add tests (start with handler tests using httptest)
7. Build budget forecasting feature
8. Add SOW document auto-generation
