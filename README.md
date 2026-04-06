# Indian Legal GPT — POC

RAG-powered legal AI for Indian law — BNS, BNSS, BSA, and the Constitution of India.

Built with Go (HTTP service), Python (ingest pipeline), Qdrant (vector DB), and pluggable LLM providers (Ollama, Claude, Gemini).

---

## Architecture

```
                         ┌──────────────────────────┐
                         │     Python Ingest CLI     │
                         │  chunker → embedder →     │
                         │  sparse encoder → loader  │
                         └────────────┬─────────────┘
                                      │ upsert (dense + sparse vectors)
                                      ▼
┌──────────┐   POST /chat   ┌─────────────────────────────────────────┐
│  Client  │ ◄──── SSE ──── │          Go Query Service (Gin)         │
└──────────┘                │                                         │
                            │  Auth (RS256 JWT) → Rate Limiter        │
                            │  → Worker Pool → Guardrail              │
                            │  → Embed → Hybrid Retriever (RRF)       │
                            │  → Reranker → Context Assembler         │
                            │  → IPC Detector → LLM Router → Stream   │
                            └──────┬──────────┬──────────┬────────────┘
                                   │          │          │
                              ┌────▼───┐ ┌────▼───┐ ┌────▼──────┐
                              │ Qdrant │ │Postgres│ │ LLM (any) │
                              │ (gRPC) │ │ (pgx)  │ │Ollama/    │
                              │        │ │  RLS   │ │Claude/    │
                              └────────┘ └────────┘ │Gemini     │
                                                    └───────────┘
```

---

## Features (Phase 1)

- **Hybrid retrieval** — Dense (Qdrant ANN) + BM25 sparse vectors with Reciprocal Rank Fusion
- **Cross-encoder reranking** — Cohere Rerank API or local Ollama-hosted model (optional)
- **LLM complexity router** — Routes simple queries to Gemini, complex queries to Claude
- **IPC intent detection** — Auto-detects old IPC section references, maps to BNS equivalents
- **RS256 JWT auth** — Algorithm-pinned, role-based (free/pro/enterprise/admin)
- **Backpressure** — Semaphore worker pool + circuit breaker + per-tier rate limiting
- **PostgreSQL with RLS** — Tenant isolation via `SET LOCAL app.tenant_id`
- **SSE streaming** — Real-time token streaming with source citations
- **Guardrails** — Abstention when confidence < 0.72, prompt injection detection
- **RAGAS evaluation** — CI pipeline with 50 golden queries, faithfulness gate

---

## Prerequisites

| Tool | Version | Install |
|------|---------|---------|
| Go | 1.25+ | https://go.dev/dl/ |
| Python | 3.12+ | https://www.python.org/downloads/ |
| Docker | Latest | https://docs.docker.com/get-docker/ |
| Ollama | Latest | https://ollama.com/download |

---

## Quick Start

### 1. Start infrastructure

```bash
make docker-up   # Qdrant (6333/6334) + Redis (6379) + Postgres (5432)
```

### 2. Pull Ollama models

```bash
ollama pull nomic-embed-text   # 768-dim embeddings
ollama pull llama3.2           # chat model
```

### 3. Ingest documents

```bash
cd ingest
pip install -r requirements.txt
python -m pipeline.run --source constitution_of_india --file ../data/constitution.pdf
```

### 4. Run the query service

```bash
# Dev mode (no auth required)
DEV_MODE=true make dev
```

### 5. Query

```bash
# SSE streaming chat
curl -N -X POST http://localhost:8080/api/v1/chat \
  -H "Content-Type: application/json" \
  -d '{"query": "What does Article 21 say about right to life?"}'

# Non-streaming search
curl -X POST http://localhost:8080/api/v1/search \
  -H "Content-Type: application/json" \
  -d '{"query": "fundamental rights", "top_k": 5}'
```

---

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/v1/health/live` | None | Liveness probe |
| GET | `/api/v1/health/ready` | None | Readiness probe (Qdrant + LLM + Postgres) |
| POST | `/api/v1/auth/token` | None | Issue RS256 JWT (client_credentials) |
| POST | `/api/v1/chat` | JWT/Dev | SSE streaming chat with RAG pipeline |
| POST | `/api/v1/search` | JWT/Dev | Non-streaming vector search |
| GET | `/api/v1/admin/routing-stats` | JWT (admin) | LLM routing distribution |

### SSE event types (`/chat`)

| Event | Description |
|-------|-------------|
| `chunk` | Streamed token delta |
| `sources` | Retrieved source citations |
| `meta` | Token counts, latency, provider used |
| `abstain` | Confidence too low to answer |
| `error` | Processing error |
| `done` | Stream complete |

---

## Configuration

All configuration is via environment variables. No `.env` files in git.

| Variable | Default | Description |
|----------|---------|-------------|
| `LLM_PROVIDER` | `ollama` | `ollama`, `claude`, `gemini`, or `router` |
| `OLLAMA_URL` | `http://localhost:11434` | Ollama base URL |
| `CLAUDE_API_KEY` | — | Required for Claude provider |
| `GEMINI_API_KEY` | — | Required for Gemini provider |
| `QDRANT_HOST` | `localhost` | Qdrant host |
| `QDRANT_PORT` | `6334` | Qdrant gRPC port |
| `EMBED_MODEL` | `nomic-embed-text` | Embedding model |
| `CHAT_MODEL` | `llama3.2` | Chat model |
| `VECTOR_SIZE` | `768` | Must match embedding model dimensions |
| `API_PORT` | `8080` | HTTP server port |
| `DEV_MODE` | `false` | Skip auth when `true` |
| `DEV_TOKEN` | — | Static dev auth token |
| `POSTGRES_DSN` | — | PostgreSQL connection string |
| `JWT_PUBLIC_KEY_PATH` | — | RSA public key for JWT validation |
| `JWT_PRIVATE_KEY_PATH` | — | RSA private key for JWT issuance |
| `WORKER_POOL_SIZE` | `50` | Max concurrent requests |
| `LLM_ROUTE_THRESHOLD` | `500` | Token threshold for router |
| `RERANK_PROVIDER` | — | `cohere`, `local`, or empty (disabled) |
| `RERANK_MODEL` | — | Override default rerank model |
| `COHERE_API_KEY` | — | Required for Cohere reranking |

---

## Commands

```bash
# Development
make dev                    # Run query service locally
make docker-up              # Start Qdrant + Redis + Postgres
make docker-down            # Stop infrastructure

# Testing
make test                   # Go tests with race detector + coverage
make test-python            # Python pytest
go test -run TestName ./internal/rag/...   # Single test

# Linting
make lint                   # golangci-lint + ruff
make lint-fix               # Auto-fix

# Build
make build                  # Binary → bin/query-service

# Evaluation
make eval                   # Run RAGAS evaluation suite
```

---

## Project Structure

```
legalGPT-POC/
├── services/query/              # Go HTTP service (Gin)
│   ├── main.go                  # Entry point, wiring
│   ├── handler/                 # HTTP handlers (chat, search, health, auth, admin)
│   └── middleware/              # JWT auth, rate limiting
├── internal/
│   ├── llm/                     # LLM providers (Ollama, Claude, Gemini, Router)
│   │   ├── provider.go          # LLMProvider interface
│   │   ├── factory.go           # Config-driven provider creation
│   │   ├── router.go            # Complexity-based routing
│   │   └── breaker.go           # Circuit breaker wrapper
│   ├── rag/                     # RAG pipeline
│   │   ├── hybrid.go            # Dense + BM25 hybrid retriever (RRF)
│   │   ├── rerank.go            # Cross-encoder reranker (Cohere + local)
│   │   ├── ipc_detect.go        # IPC section detection → BNS mapping
│   │   ├── context.go           # Token-budget context assembly
│   │   ├── guardrail.go         # Abstention + injection detection
│   │   ├── retriever.go         # System prompt + message builder
│   │   └── vocab.go             # BM25 vocabulary loader
│   ├── store/                   # Data stores
│   │   ├── vector.go            # VectorStore interface
│   │   ├── qdrant.go            # Qdrant client (Search, SearchHybrid, SearchMulti)
│   │   └── postgres.go          # PostgreSQL repo (pgx/v5, WithTenant RLS)
│   ├── db/                      # Migrations + IPC loader
│   ├── config/                  # Environment-based config
│   ├── worker/                  # Semaphore worker pool
│   └── telemetry/               # OpenTelemetry stub
├── ingest/                      # Python ingest pipeline
│   ├── chunker/                 # SAC chunking (Part → Chapter → Section)
│   ├── embedder/                # Dense + BM25 sparse encoding
│   ├── store/                   # Qdrant batch upsert
│   ├── quality/                 # Post-ingest validation
│   └── pipeline/                # CLI runner
├── eval/                        # RAGAS evaluation
│   ├── golden_set.jsonl         # 50 golden queries
│   ├── ragas_eval.py            # Evaluation runner
│   └── judge_prompt.py          # LLM-as-judge scorer
├── deployments/docker/          # Docker Compose for local infra
├── scripts/                     # IPC→BNS mapping JSON
├── data/                        # Source PDFs + vocab files
├── .github/workflows/           # CI (lint/test/build) + RAGAS eval
└── CLAUDE.md                    # Codebase conventions and architecture
```

---

## Domain Context

- **BNS** — Bharatiya Nyaya Sanhita 2023 (replaces IPC)
- **BNSS** — Bharatiya Nagarik Suraksha Sanhita 2023 (replaces CrPC)
- **BSA** — Bharatiya Sakshya Adhiniyam 2023 (replaces Indian Evidence Act)
- IPC-to-BNS mapping loaded from `scripts/ipc_bns_map.json`

---

## License

Private — not open source.
