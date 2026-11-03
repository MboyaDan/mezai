# Run from backend/ or via the root Makefile.
-include .env
export

GOOSE := go run github.com/pressly/goose/v3/cmd/goose@v3.29.0
SQLC  := go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.31.1

.PHONY: help up down logs ps test lint run migrate migrate-status migrate-create seed setup sqlc

help: ## List targets
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-16s %s\n", $$1, $$2}'

.env:
	cp .env.example .env

up: .env ## Build and start api, postgres, storage and mailpit; wait until healthy
	docker compose up -d --build --wait

down: ## Stop the stack (data volumes are kept)
	docker compose down

logs: ## Follow API logs
	docker compose logs -f api

ps: ## Show service status
	docker compose ps

test: ## Run Go tests (needs `make up` for the database tests)
	go test -race -count=1 ./...

lint: ## gofmt and go vet
	@test -z "$$(gofmt -l .)" || { gofmt -l .; echo "run gofmt -w ."; exit 1; }
	go vet ./...

run: ## Run the API on the host against the compose services
	go run ./cmd/api

migrate: ## Apply migrations to DATABASE_URL (local only; never run against production from here)
	@if ls migrations/*.sql >/dev/null 2>&1; then $(GOOSE) -dir migrations postgres "$(DATABASE_URL)" up; else echo "no migrations yet (Stage 3)"; fi

migrate-status: ## Show migration status
	$(GOOSE) -dir migrations postgres "$(DATABASE_URL)" status

migrate-create: ## Create a migration: make migrate-create name=add_users
	@test -n "$(name)" || { echo "usage: make migrate-create name=..."; exit 1; }
	$(GOOSE) -dir migrations create $(name) sql

seed: ## Load sample data (refuses unless APP_ENV is development or test)
	go run ./cmd/seed

setup: ## Create the first Super Admin from SETUP_ADMIN_* (any environment; no-op once one exists)
	go run ./cmd/setup

sqlc: ## Generate Go code from queries/ (config arrives in Stage 3)
	@if [ -f sqlc.yaml ]; then $(SQLC) generate; else echo "sqlc.yaml not created yet (Stage 3)"; fi
