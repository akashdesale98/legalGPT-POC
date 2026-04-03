# Local Development & Debugging Guide

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.25+ | Query service |
| Python | 3.12+ | Ingest pipeline |
| Docker & Docker Compose | Latest | Qdrant, PostgreSQL, Redis |
| Ollama | Latest | Local LLM + embeddings |
| Git | Latest | Version control |

Optional: `golangci-lint`, `ruff` (linting), `goose` (DB migrations).

---

## 1. Clone & Setup

```bash
git clone <repo-url>
cd legalGPT-POC
```

### Go dependencies

```bash
go mod download
```

### Python virtual environment

```bash
cd ingest
python -m venv .venv

# Windows
.venv\Scripts\activate

# Linux/macOS
source .venv/bin/activate

pip install -r requirements.txt
cd ..
```

---

## 2. Start Infrastructure (Docker)

Start Qdrant, PostgreSQL, and Redis:

```bash
make docker-up
# or
docker compose -f deployments/docker/docker-compose.yml up -d
```

Verify containers are running:

```bash
docker compose -f deployments/docker/docker-compose.yml ps
```

| Service | Port | Health check |
|---------|------|--------------|
| Qdrant (HTTP) | `6333` | `curl http://localhost:6333/healthz` |
| Qdrant (gRPC) | `6334` | Used by Go query service |
| PostgreSQL | `5432` | `pg_isready -h localhost -U legalgpt` |
| Redis | `6379` | `redis-cli ping` |

To stop infrastructure:

```bash
make docker-down
```

---

## 3. Install & Configure Ollama

```bash
# Install from https://ollama.com
# Then pull required models:
ollama pull llama3.2           # Chat model
ollama pull nomic-embed-text   # Embedding model
```

Verify Ollama is running:

```bash
curl http://localhost:11434/api/tags
```

---

## 4. Environment Configuration

```bash
cp .env.example .env
```

Edit `.env` with your values. Minimum config for local Ollama setup:

```env
LLM_PROVIDER=ollama
OLLAMA_URL=http://localhost:11434
EMBED_MODEL=nomic-embed-text
CHAT_MODEL=llama3.2
VECTOR_SIZE=768
QDRANT_HOST=localhost
QDRANT_PORT=6334
API_PORT=8080
DEFAULT_COLLECTION=constitution
DEV_MODE=true
DEV_TOKEN=dev-secret-token-123
```

For cloud LLM providers instead of Ollama:

```env
# Claude
LLM_PROVIDER=claude
CLAUDE_API_KEY=sk-ant-...

# Gemini
LLM_PROVIDER=gemini
GEMINI_API_KEY=AIza...
```

---

## 5. Run Database Migrations (Optional)

Only needed if using PostgreSQL features (sessions, audit log):

```bash
export DATABASE_URL="postgres://legalgpt:devpassword@localhost:5432/legalgpt?sslmode=disable"
make migrate
```

---

## 6. Ingest Data

### Dry run (no embedding or Qdrant writes)

```bash
python -m ingest.pipeline.run --file data/constitution.pdf --dry-run
```

### Full ingest (Constitution)

```bash
python -m ingest.pipeline.run \
  --file data/constitution.pdf \
  --collection constitution \
  --source-name constitution_of_india \
  --doc-type constitutional_provision \
  --act-name "Constitution of India" \
  --mode auto
```

### Ingest with IPC-BNS mapping

```bash
python -m ingest.pipeline.run \
  --file data/bns.pdf \
  --collection bns \
  --source-name bns_2023 \
  --doc-type criminal_statute \
  --act-name "Bharatiya Nyaya Sanhita 2023" \
  --ipc-bns-map scripts/ipc_bns_map.json
```

### Reset a collection (drop + recreate)

```bash
python -m ingest.pipeline.run --file data/constitution.pdf --collection constitution --reset
```

### Verify ingestion

```bash
curl http://localhost:6333/collections/constitution
```

---

## 7. Run the Query Service

### Dev mode (with hot reload via `go run`)

```bash
make dev
# or
go run ./services/query/...
```

The API starts on `http://localhost:8080`.

### Build binary

```bash
make build
./bin/query-service
```

---

## 8. Test the API

### Health checks

```bash
# Liveness (always 200 if process is up)
curl http://localhost:8080/api/v1/health/live

# Readiness (checks Qdrant + LLM connectivity)
curl http://localhost:8080/api/v1/health/ready
```

### Search (non-streaming)

```bash
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer dev-secret-token-123" \
  -d '{
    "query": "What is the right to equality?",
    "collections": ["constitution"],
    "top_k": 5
  }'
```

### Chat (SSE streaming)

```bash
curl -N -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer dev-secret-token-123" \
  -d '{
    "query": "What is Article 21 of the Indian Constitution?",
    "collections": ["constitution"]
  }'
```

The response is a stream of Server-Sent Events:

```
event: chunk
data: {"delta":"Article 21 guarantees...","done":false}

event: sources
data: {"citations":[{"article_number":"21","score":0.89}]}

event: meta
data: {"latency_ms":1234,"provider":"ollama"}

event: done
data: {"abstained":false}
```

---

## 9. Running Tests

### Go tests

```bash
make test
# or
go test -race ./...
```

### Python tests

```bash
cd ingest
python -m pytest -v
```

### Linting

```bash
make lint        # Run both Go + Python linters
make lint-fix    # Auto-fix what's possible
```

---

## 10. Debugging

### Go query service (VS Code)

Add to `.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Query Service",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/services/query",
      "envFile": "${workspaceFolder}/.env"
    }
  ]
}
```

### Go query service (GoLand/IntelliJ)

1. Run > Edit Configurations > + Go Build
2. Package path: `testqdrant/services/query`
3. Environment: load from `.env` file
4. Run or Debug

### Python ingest (VS Code)

Add to `.vscode/launch.json`:

```json
{
  "name": "Ingest Pipeline",
  "type": "debugpy",
  "request": "launch",
  "module": "ingest.pipeline.run",
  "args": ["--file", "data/constitution.pdf", "--dry-run"],
  "cwd": "${workspaceFolder}",
  "envFile": "${workspaceFolder}/.env"
}
```

### Debugging tips

- **Qdrant dashboard**: Open `http://localhost:6333/dashboard` to inspect collections, points, and run test queries.
- **Ollama logs**: `ollama logs` or check the Ollama system tray/terminal for model loading errors.
- **Query service logs**: All logs go to stdout. Set `LOG_LEVEL=debug` in `.env` for verbose output.
- **SSE debugging**: Use `curl -N` (no buffering) or browser DevTools EventSource to inspect streaming responses.
- **Rate limiting**: Dev token gets `pro` tier (500 req/hr). Unauthenticated dev mode gets `free` tier (20 req/hr).

---

## 11. Project Structure

```
legalGPT-POC/
├── internal/                  # Go shared libraries
│   ├── config/                #   Environment config loader
│   ├── llm/                   #   LLM providers (Ollama, Claude, Gemini)
│   ├── store/                 #   Vector store interface + Qdrant impl
│   ├── rag/                   #   Retrieval, context assembly, guardrails
│   ├── telemetry/             #   OpenTelemetry stubs
│   └── db/migrations/         #   SQL migration files
├── services/query/            # Go query API service
│   ├── main.go                #   Entry point, wiring
│   ├── handler/               #   HTTP handlers (chat, search, health)
│   └── middleware/             #   Auth, rate limiting
├── ingest/                    # Python ingestion pipeline
│   ├── pipeline/              #   CLI entry point + config
│   ├── chunker/               #   PDF parsing (statute, judgment)
│   ├── embedder/              #   Ollama/Cohere embedding
│   ├── store/                 #   Qdrant loader
│   └── quality/               #   Post-ingest quality gates
├── deployments/docker/        # Dockerfiles + compose
├── data/                      # PDF source documents
├── scripts/                   # Utility data (IPC-BNS mapping)
└── .env.example               # Environment template
```

---

## Common Issues

| Problem | Fix |
|---------|-----|
| `ollama /api/chat returned 404` | Run `ollama pull llama3.2` to download the model |
| `qdrant health check: connection refused` | Run `make docker-up` to start Qdrant |
| `CLAUDE_API_KEY is required` | Set `LLM_PROVIDER=ollama` in `.env` or provide the key |
| `No chunks produced` | Check that the PDF path is correct and the file exists |
| `embedding dimensions mismatch` | Ensure `VECTOR_SIZE` matches your embed model (768 for nomic-embed-text) |
| `rate limit exceeded` | Use the dev token (`Authorization: Bearer <DEV_TOKEN>`) for pro-tier limits |
| Python `ModuleNotFoundError: ingest` | Run commands from the project root, not from inside `ingest/` |
