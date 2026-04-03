.PHONY: dev build test lint migrate ingest docker-up docker-down

# --- Development ---
dev:
	go run ./services/query/...

build:
	go build -o bin/query-service ./services/query/...

# --- Testing ---
test:
	go test -race -coverprofile=coverage.out ./...
	@echo "Coverage report: coverage.out"

test-python:
	cd ingest && python -m pytest -v

# --- Linting ---
lint:
	golangci-lint run ./...
	cd ingest && ruff check .

lint-fix:
	golangci-lint run --fix ./...
	cd ingest && ruff check --fix .

# --- Database ---
migrate:
	goose -dir internal/db/migrations postgres "$(DATABASE_URL)" up

migrate-down:
	goose -dir internal/db/migrations postgres "$(DATABASE_URL)" down

# --- Ingestion ---
ingest:
	cd ingest && python -m pipeline.run $(ARGS)

# --- Docker ---
docker-up:
	docker compose -f deployments/docker/docker-compose.yml up -d

docker-down:
	docker compose -f deployments/docker/docker-compose.yml down

# --- Utilities ---
clean:
	rm -rf bin/ coverage.out
	find . -type d -name __pycache__ -exec rm -rf {} + 2>/dev/null || true
