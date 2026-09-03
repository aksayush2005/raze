.PHONY: help db-up db-down api ai api-run ai-run migrate test build down ps tui

help: ## Show available targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-12s %s\n", $$1, $$2}'

db-up: ## Start PostgreSQL (and Redis, when used) via Docker
	docker compose up -d postgres

db-down: ## Stop database containers
	docker compose down

down: ## Stop all containers
	docker compose down

api-run: ## Run the Go API control plane
	cd services/api && go run ./cmd/api

ai-run: ## Run the Python AI service
	cd ai && python -m uvicorn app.main:app --reload --port 8090

migrate: ## Apply database migrations
	cd services/api && go run ./cmd/migrate

test: ## Run Go tests
	cd services/api && go test ./...

build: ## Build all Go binaries
	cd services/api && go build ./cmd/...

up: ## Full demo stack (requires images built)
	docker compose up --build

tui: ## Run the terminal UI (boots the stack if needed)
	bash scripts/tui.sh
