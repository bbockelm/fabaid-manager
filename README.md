# FabAID Manager

NSF Grant & Project Tracking System — track effort/personnel by WBS area, planned budgets, subawards, invoices, and statements of work.

## Architecture

| Layer | Technology |
|-------|------------|
| **Backend API** | Go (chi router) |
| **Frontend** | Next.js 14 (App Router) + Tailwind CSS |
| **Database** | PostgreSQL 16 |
| **Object Storage** | S3-compatible (MinIO for dev) |
| **Migrations** | goose |

## Quick Start (Dev Container)

1. Open the project in VS Code
2. When prompted, click **"Reopen in Container"** (or run `Dev Containers: Reopen in Container` from the command palette)
3. The devcontainer starts PostgreSQL, MinIO, and installs all dependencies
4. Run:
   ```bash
   make dev
   ```
   - Backend: http://localhost:8080
   - Frontend: http://localhost:3000
   - MinIO Console: http://localhost:9001 (minioadmin / minioadmin)

## Project Structure

```
fabaid-manager/
├── cmd/server/             # Go entrypoint
├── internal/
│   ├── config/             # Environment-based configuration
│   ├── db/                 # Database connection, migrations, queries
│   │   └── migrations/     # SQL migration files (goose)
│   ├── handlers/           # HTTP handlers
│   ├── models/             # Data models
│   ├── router/             # HTTP router setup
│   └── storage/            # S3 storage layer
├── frontend/               # Next.js application
│   └── src/
│       ├── app/            # App Router pages
│       ├── components/     # React components
│       └── lib/            # API client & utilities
├── deploy/
│   ├── k8s/                # Kubernetes manifests
│   └── dev/                # Dev mode supervisor config
├── scripts/
│   ├── start.sh            # Production container start script
│   └── restore.sh          # Backup restore script
├── .devcontainer/          # VS Code devcontainer setup
├── Dockerfile              # Production multi-stage build
├── Dockerfile.dev          # All-in-one dev container
├── Makefile                # Project commands
└── .air.toml               # Go hot-reload config
```

## Key Features

- **Grant Management**: Create and track NSF grants/awards
- **WBS Areas**: Break down grants into Work Breakdown Structure areas
- **Personnel Tracking**: Track effort percentages, funded months, and salaries by WBS
- **Budget Management**: Planned vs actual spending by category and fiscal year
- **Subawards**: Track subaward institutions, PIs, and expenditures
- **Invoice Management**: Upload invoice PDFs, track approval status
- **Statements of Work**: Annual SOW generation and signed PDF storage
- **Budget Summary**: Aggregated budget vs actuals view
- **Backup/Restore**: Full system backup as a tarball (DB dump + all documents)

## API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/healthz` | Health check |
| GET | `/api/v1/grants` | List grants |
| POST | `/api/v1/grants` | Create grant |
| GET | `/api/v1/grants/:id` | Get grant |
| PUT | `/api/v1/grants/:id` | Update grant |
| DELETE | `/api/v1/grants/:id` | Delete grant |
| GET | `/api/v1/grants/:id/budget-summary` | Budget summary |
| GET/POST | `/api/v1/grants/:id/wbs` | WBS areas |
| GET/POST | `/api/v1/grants/:id/personnel` | Personnel |
| GET/POST | `/api/v1/grants/:id/budget` | Budget items |
| GET/POST | `/api/v1/grants/:id/subawards` | Subawards |
| GET/POST | `/api/v1/grants/:gid/subawards/:sid/invoices` | Invoices |
| POST | `.../invoices/:iid/upload` | Upload invoice PDF |
| GET/POST | `.../subawards/:sid/sow` | Statements of Work |
| POST | `.../sow/:sowid/upload-signed` | Upload signed SOW |
| GET | `/api/v1/documents/:id/download` | Download document |
| GET | `/api/v1/backup` | Download full backup |

## Database Migrations

```bash
make migrate          # Apply pending migrations
make migrate-down     # Roll back last migration
make migrate-status   # Show migration status
```

## Backup & Restore

**Create a backup** (downloads all PDFs + DB dump as `.tar.gz`):
```bash
make backup
# or via the web UI: /backup page
```

**Restore from backup**:
```bash
make restore BACKUP=fabaid-backup-20260304-120000.tar.gz
```

## Development Mode (Single Container)

For running from a backup in a single container (e.g., inside VS Code):
```bash
make docker-dev
docker run -p 8080:8080 -p 3000:3000 -p 5432:5432 fabaid-manager-dev:latest
```

To restore a backup into the dev container:
```bash
docker cp fabaid-backup.tar.gz <container>:/tmp/
docker exec <container> bash /app/restore.sh /tmp/fabaid-backup.tar.gz
```

## Production Deployment (Kubernetes)

1. Build and push the Docker image
2. Update secrets in `deploy/k8s/deployment.yaml`
3. Apply manifests:
   ```bash
   kubectl apply -f deploy/k8s/deployment.yaml
   ```
4. Configure your SSO/OIDC provider and update the `OIDC_*` environment variables

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | Yes | — | PostgreSQL connection string |
| `S3_ENDPOINT` | Yes | — | S3/MinIO endpoint |
| `S3_BUCKET` | No | `fabaid-documents` | S3 bucket name |
| `S3_ACCESS_KEY` | Yes | — | S3 access key |
| `S3_SECRET_KEY` | Yes | — | S3 secret key |
| `S3_USE_PATH_STYLE` | No | `true` | Use path-style S3 URLs |
| `S3_USE_SSL` | No | `false` | Use SSL for S3 |
| `APP_ENV` | No | `development` | `development` or `production` |
| `APP_PORT` | No | `8080` | Backend API port |
| `OIDC_ISSUER` | No | — | OIDC issuer URL (production SSO) |
| `OIDC_CLIENT_ID` | No | — | OIDC client ID |
| `OIDC_CLIENT_SECRET` | No | — | OIDC client secret |

## Future Enhancements

- [ ] Budget & expenditure forecasting (spend rate projections)
- [ ] OIDC/SSO authentication middleware
- [ ] Auto-generate annual SOW documents
- [ ] Email notifications for invoice approvals
- [ ] Dashboard charts and visualizations
- [ ] Audit logging
