.PHONY: help dev dev-backend dev-frontend build test migrate backup restore clean docker docker-dev

# Default environment
DATABASE_URL ?= postgres://fabaid:fabaid@localhost:5432/fabaid?sslmode=disable
S3_ENDPOINT ?= http://localhost:9000
S3_BUCKET ?= fabaid-documents
S3_ACCESS_KEY ?= minioadmin
S3_SECRET_KEY ?= minioadmin

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# --- Development ---

dev: ## Run backend + frontend concurrently (requires devcontainer or local deps)
	@echo "Starting backend and frontend..."
	@make dev-backend &
	@make dev-frontend
	@wait

dev-backend: ## Run Go backend with hot reload (air)
	cd $(CURDIR) && air -c .air.toml

dev-frontend: ## Run Next.js dev server
	cd frontend && npm run dev

# --- Build ---

build: build-backend build-frontend ## Build everything

build-backend: ## Build Go binary (dev — no embedded frontend)
	CGO_ENABLED=0 go build -o bin/fabaid-server ./cmd/server

build-frontend: ## Build Next.js frontend (static export to frontend/out/)
	cd frontend && npm run build

build-prod: build-frontend ## Build single production binary with embedded frontend
	rm -rf internal/frontend/dist
	cp -r frontend/out internal/frontend/dist
	CGO_ENABLED=0 go build -tags embed_frontend -o bin/fabaid-server ./cmd/server
	rm -rf internal/frontend/dist

# --- Database ---

migrate: ## Run database migrations
	goose -dir internal/db/migrations postgres "$(DATABASE_URL)" up

migrate-down: ## Roll back last migration
	goose -dir internal/db/migrations postgres "$(DATABASE_URL)" down

migrate-status: ## Show migration status
	goose -dir internal/db/migrations postgres "$(DATABASE_URL)" status

# --- Testing ---

test: ## Run Go tests
	go test ./... -v

lint: ## Lint Go code
	golangci-lint run ./...

lint-frontend: ## Lint frontend
	cd frontend && npm run lint

# --- Docker ---

docker: ## Build production Docker image
	docker build -t fabaid-manager:latest .

docker-dev: ## Build all-in-one dev Docker image
	docker build -t fabaid-manager:latest .
	docker build -t fabaid-manager-dev:latest -f Dockerfile.dev .

# --- Backup / Restore ---

backup: ## Download backup via API
	curl -o fabaid-backup-$$(date +%Y%m%d-%H%M%S).tar.gz http://localhost:8080/api/v1/backup

restore: ## Restore from backup (usage: make restore BACKUP=<file.tar.gz>)
	@if [ -z "$(BACKUP)" ]; then echo "Usage: make restore BACKUP=<file.tar.gz>"; exit 1; fi
	DATABASE_URL="$(DATABASE_URL)" \
	S3_ENDPOINT="$(S3_ENDPOINT)" \
	S3_BUCKET="$(S3_BUCKET)" \
	S3_ACCESS_KEY="$(S3_ACCESS_KEY)" \
	S3_SECRET_KEY="$(S3_SECRET_KEY)" \
	bash scripts/restore.sh "$(BACKUP)"

# --- Cleanup ---

clean: ## Remove build artifacts
	rm -rf bin/ tmp/
	rm -rf frontend/.next frontend/out
	rm -rf frontend/node_modules
	rm -rf internal/frontend/dist
