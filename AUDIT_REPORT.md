# AUDIT_REPORT.md — Pre-Refactor Codebase Review

> Date: 2026-04-02  
> Auditor: Claude (automated)  
> Scope: All existing Go + Python source in legalGPT-POC  
> Basis: ARCHITECTURE.md, ROADMAP_V2.md, INGESTION_PIPELINE.md, COMPLIANCE_REGISTER.md, SYNTHESIS_NOTES.md

---

## Section A — Existing Code Inventory

### A.1 `main.go` (Go — 98 lines)

**What it does:** CLI REPL that reads user queries from stdin, embeds via Ollama, retrieves from Qdrant, generates an answer via Ollama chat, and prints results with source citations.

**Does well:**
- Clean sequential pipeline: embed → retrieve → generate → display
- Proper error handling with `continue` on per-query errors (doesn't crash the loop)
- Shows latency per query
- Prints source citations with article numbers and scores

**Does poorly:**
- No `context.Context` anywhere — cannot cancel long-running LLM/Qdrant calls
- No graceful shutdown (Ctrl+C kills mid-request with no cleanup)
- No HTTP server — CLI only, cannot serve web/API clients
- Hardcoded config path `"config.json"` — no env var override
- No concurrency controls — single-threaded, blocks on each query
- No timeout on any network call (Ollama or Qdrant could hang forever)
- Uses `log.Fatalf` for startup errors — correct for CLI, but needs rework for service

### A.2 `rag.go` (Go — 114 lines)

**What it does:** Defines `SearchResult` struct, `SearchQdrant` function for ANN search, and `BuildRAGPrompt` for assembling LLM messages with retrieved context.

**Does well:**
- Clean Qdrant query using official `go-client` with `QueryPoints`
- Properly extracts all payload fields (text, article_number, article_title, part, source, document_type)
- Prompt is well-structured: separate system and user messages (not concatenated)
- System prompt correctly constrains the LLM to answer from context only
- Error wrapping with `fmt.Errorf("qdrant query: %w", err)`

**Does poorly:**
- No hybrid retrieval — cosine-similarity-only, no BM25 sparse, no reranking
- Hardcodes collection name as parameter but no tenant_id filtering (multi-tenancy gap)
- No re-ranking step — raw ANN results go directly to context assembly
- No token budget management in `BuildRAGPrompt` — all results are concatenated regardless of total token count
- No deduplication of chunks (could include overlapping content)
- No staleness check (no `superseded_by` handling)
- No abstention/guardrail logic — always generates an answer regardless of retrieval confidence
- System prompt is constitution-specific ("expert on Indian Constitutional Law") — not generalised for BNS/BNSS/BSA
- `context.Background()` used directly — no timeout, no cancellation propagation
- No IPC→BNS mapping awareness

### A.3 `ollama.go` (Go — 104 lines)

**What it does:** HTTP client wrapper for Ollama REST API — embedding (`/api/embeddings`) and chat completion (`/api/chat`).

**Does well:**
- Clean struct-based request/response types
- Proper HTTP status code checking
- Empty embedding validation (`len(result.Embedding) == 0`)
- Error messages include endpoint path for debuggability

**Does poorly:**
- No `context.Context` on any method — cannot cancel or timeout requests
- `json.Marshal` error silently ignored on line 59 (`body, _ := json.Marshal(...)`)
- `json.Marshal` error silently ignored on line 87 (`body, _ := json.Marshal(...)`)
- `http.Client{}` has no timeout configured — requests can hang indefinitely
- No retry logic — a single 429/503 from Ollama fails the whole query
- No streaming support — `Stream: false` hardcoded; cannot do SSE token streaming
- Not behind an interface — tightly coupled; cannot swap to Claude/OpenAI without rewriting callers
- No token counting or usage reporting
- No connection keepalive or pooling configuration

### A.4 `config.go` (Go — 51 lines)

**What it does:** Loads configuration from `config.json` with hardcoded defaults.

**Does well:**
- Provides sensible defaults via `defaultConfig()` — binary works without a config file
- Clean JSON deserialization

**Does poorly:**
- Reads from JSON file only — no environment variable support (secrets in JSON = committed to git)
- `config.json` is tracked in git and contains connection strings (not secrets yet, but establishes a dangerous pattern)
- No validation — empty strings, port 0, negative topK are all silently accepted
- No struct tags for env var binding (needs Viper or similar)
- Missing fields for: LLM provider selection, API keys, database URL, Redis URL, JWT config, OTEL endpoint
- Silently returns defaults on file open error (`return cfg, nil`) — user has no idea config was ignored

### A.5 `config.json` (10 lines)

**What it does:** JSON config with Qdrant, Ollama, and model settings.

**Risk:** Currently contains no secrets, but the pattern of using a tracked JSON file for config means future additions (API keys, DB passwords) could easily be committed. Must be replaced with `.env` approach and added to `.gitignore`.

### A.6 `go.mod` (14 lines)

**What it does:** Go module definition.

**Issues:**
- Module name is `testqdrant` — not a proper module path (should be project-specific)
- Only dependency: `github.com/qdrant/go-client v1.17.1` — correct for current POC
- Indirect deps: `google.golang.org/grpc`, `google.golang.org/protobuf` — needed by Qdrant client
- Missing all dependencies needed for Phase 1: Gin, Viper, anthropic-sdk-go, openai-go, genai, otel, pgx, go-redis, conc

### A.7 `ingest/ingest.py` (122 lines)

**What it does:** CLI entry point for the ingestion pipeline. Parses args, calls chunker, embedder, and Qdrant loader in sequence.

**Does well:**
- Clean 3-step pipeline: chunk → embed → load
- `--dry-run` flag for testing without writing to Qdrant
- Validates chunk output (exits if no chunks produced)
- Sample chunk display for visual verification
- Configurable via argparse (collection, source, model, etc.)
- Auto-updates vector size if model returns different dimensions

**Does poorly:**
- No error handling around embedding step — a single failed embed crashes the whole run
- No failed document tracking — no retry, no dead letter
- No content hashing / deduplication
- No quality gates (no OCR error check, no PII detection, no minimum coherence)
- No async/concurrent processing — embeds one chunk at a time
- No structured logging (uses `print()`)
- Hardcoded default PDF path `../data/constitution.pdf`

### A.8 `ingest/pdf_parser.py` (95 lines)

**What it does:** Extracts text from PDF using PyMuPDF and parses Indian Constitution structure (Part/Chapter/Article).

**Does well:**
- Regex-based parsing of PART, CHAPTER, and ARTICLE headings
- Tracks part/chapter context as it iterates through articles
- Extracts clause boundaries

**Does poorly:**
- **Executed at module import time** — lines 91-95 run `extract_pdf_text("constitution.pdf")` and `parse_constitution(text)` at import, which crashes if the file doesn't exist. This is a bug.
- Hardcoded path `"constitution.pdf"` on line 91
- Duplicate of `chunker.py` functionality — `chunker.py` has a more mature version of the same parser
- No error handling on file open
- `LegalSection` dataclass is similar to but incompatible with `Chunk` dataclass in chunker.py
- Not imported by any other file (standalone script that was superseded by chunker.py)

### A.9 `ingest/chunker.py` (239 lines)

**What it does:** Article-level chunking for Indian Constitution PDFs with page-level fallback.

**Does well:**
- Robust regex patterns for PART, ARTICLE, and SCHEDULE detection
- Auto-detection mode: tries article parsing, falls back to page-level if < 10 chunks found
- Long article splitting with `MAX_CHUNK_WORDS = 400`
- Schedule handling (treats each schedule as a single chunk)
- Metadata per chunk: source, document_type, part, article_number, article_title, chunk_index
- CLI mode for quick inspection

**Does poorly:**
- **Article-level chunking only** — ARCHITECTURE.md and SYNTHESIS_NOTES.md identify this as 🔴 CRITICAL. Severs logical connections between definitions, penalties, and conditions in hierarchical statutes
- No Summary-Augmented Chunking (SAC) — no parent summary prepended to chunks
- No hierarchical metadata (missing: chapter, sub_section, effective_date, superseded_by, content_hash, language, tenant_id)
- Constitution-specific only — cannot parse BNS, BNSS, BSA, or judgment prose
- No sliding window chunker for judgment documents
- No content hashing for deduplication
- No language detection

### A.10 `ingest/embedder.py` (66 lines)

**What it does:** Wraps Ollama `/api/embeddings` endpoint for generating vectors.

**Does well:**
- Model availability check on init (`_verify_model`)
- Progress reporting during batch embedding
- Timeout on HTTP calls (120s for embed, 5s for model check)
- Clean single/batch API

**Does poorly:**
- Sequential embedding only — no concurrent/async batching
- No retry on failure (429, 503, timeout)
- No tqdm progress bar (manual print-based progress)
- Ollama-only — no Cohere, no other embedding provider
- No dimension validation (doesn't check that returned vectors match expected size)
- No rate limiting / backoff
- Bare `except` is not used, but `requests.RequestException` catch in `_verify_model` prints warning and continues silently

### A.11 `ingest/qdrant_loader.py` (110 lines)

**What it does:** Manages Qdrant collection creation and batch upsert of embedded chunks.

**Does well:**
- Idempotent collection creation (`ensure_collection`)
- Reset/recreate option for clean re-ingestion
- Batch upsert with configurable `BATCH_SIZE = 64`
- UUID point IDs (correct for Qdrant)
- Collection info helper for post-load verification
- Assertion: `len(chunks) == len(embeddings)` safety check

**Does poorly:**
- No retry on upsert failure — network blip loses the entire batch
- No connection retry / reconnect logic
- UUID generated per-run — re-running upserts duplicates (not idempotent by content)
- No tenant_id in payload (no multi-tenancy support)
- No payload index creation (tenant_id, statute, section filters will be slow)
- Hardcoded default collection name "constitution"
- No error handling on `client.get_collections()` in `ensure_collection`

### A.12 `ingest/requirements.txt` (3 lines)

**Dependencies:**
- `PyMuPDF>=1.23.0` — PDF text extraction (correct, needed)
- `requests>=2.28.0` — HTTP client for Ollama (correct, needed)
- `qdrant-client>=1.9.0` — Qdrant Python client (correct, needed)

**Missing for Phase 1:**
- `pydantic` / `pydantic-settings` — config validation
- `structlog` — structured logging
- `tqdm` — progress bars
- `langdetect` — language detection
- `pdfplumber` — alternative PDF parser
- `cohere` — Cohere embedding/reranking (Phase 1)
- `python-dotenv` — .env loading

---

## Section B — Gap Analysis Table

| Component | Exists? | Location | Gap vs ARCHITECTURE.md | Action |
|-----------|---------|----------|----------------------|--------|
| LLM Provider Interface | No | — | ARCHITECTURE.md §4 defines `LLMProvider` interface with Complete/Stream/Embed/Name. Nothing exists. | BUILD_NEW |
| Ollama Provider | Partial | `ollama.go` | HTTP calls exist but not behind interface, no context, no retry, no streaming | REFACTOR |
| Claude Provider | No | — | ARCHITECTURE.md specifies `AnthropicProvider` with anthropic-sdk-go | BUILD_NEW |
| Gemini Provider | No | — | Instruction specifies GeminiProvider with google.golang.org/genai | BUILD_NEW |
| LLM Router | No | — | ARCHITECTURE.md §4 defines complexity-based routing between standard/premium providers | BUILD_NEW (Phase 2+) |
| LLM Factory | No | — | Config-driven provider construction | BUILD_NEW |
| Qdrant Vector Search | Yes | `rag.go:31-73` | Working ANN search, but no tenant filtering, no hybrid, no reranking | REFACTOR |
| VectorStore Interface | No | — | ARCHITECTURE.md §2 implies abstraction | BUILD_NEW |
| Hybrid Retrieval | No | — | Dense + BM25 + cross-encoder per ARCHITECTURE.md §3.2 | BUILD_NEW |
| Context Assembler | Partial | `rag.go:82-114` | Concatenates all results without token budget, no dedup, no staleness check | REFACTOR |
| Guardrail / Abstention | No | — | COMPLIANCE_REGISTER §4.1 requires confidence scoring + abstention below 0.65 | BUILD_NEW |
| HTTP API Server | No | — | ARCHITECTURE.md §2.1 defines REST + SSE endpoints | BUILD_NEW |
| Rate Limiter | No | — | Tiered per-tenant limits in Redis | BUILD_NEW |
| Auth Middleware | No | — | JWT validation, RBAC | BUILD_NEW |
| Config (env-based) | No | `config.go` | JSON file only, no env vars, no validation, no Viper | REFACTOR |
| Graceful Shutdown | No | — | Signal handler + drain timeout | BUILD_NEW |
| Telemetry / OTEL | No | — | ARCHITECTURE.md §7 specifies full tracing + metrics | BUILD_NEW |
| Database Layer | No | — | PostgreSQL schema, migrations, sqlc queries | BUILD_NEW |
| Ingestion Pipeline Runner | Yes | `ingest/ingest.py` | Working 3-step pipeline, but no DAG, no quality gates, no error recovery | REFACTOR |
| PDF Parser | Yes | `ingest/pdf_parser.py` | Superseded by chunker.py, has module-level execution bug | DELETE |
| Chunker (Constitution) | Yes | `ingest/chunker.py` | Article-level only, no SAC, no hierarchy, no BNS/judgment support | REFACTOR |
| Chunker (Judgment) | No | — | Sliding window with case metadata | BUILD_NEW |
| Embedder | Yes | `ingest/embedder.py` | Ollama-only, sequential, no retry | REFACTOR |
| Qdrant Loader | Yes | `ingest/qdrant_loader.py` | Working upsert, but no idempotent-by-content, no retry, no tenant_id | REFACTOR |
| Pipeline Config | No | — | Pydantic BaseSettings for ingest | BUILD_NEW |
| Quality Gates | No | — | Post-ingest verification | BUILD_NEW |
| IPC→BNS Mapping | No | — | JSON seed data + PostgreSQL table | BUILD_NEW |
| Docker Compose (dev) | No | — | Qdrant + Redis + Postgres stack | BUILD_NEW |
| CI Pipeline | No | — | GitHub Actions: lint + test + build + scan | BUILD_NEW |
| Makefile | No | — | Dev targets | BUILD_NEW |

---

## Section C — Dependency Audit

### Go — `go.mod`

| Dependency | Version | Status | Notes |
|-----------|---------|--------|-------|
| `github.com/qdrant/go-client` | v1.17.1 | **KEEP** | Core dependency, actively maintained |
| `golang.org/x/net` | v0.50.0 | indirect | Required by gRPC, auto-managed |
| `golang.org/x/sys` | v0.41.0 | indirect | Required by gRPC, auto-managed |
| `golang.org/x/text` | v0.34.0 | indirect | Required by gRPC, auto-managed |
| `google.golang.org/genproto/googleapis/rpc` | v0.0.0-20260209... | indirect | Required by Qdrant gRPC client |
| `google.golang.org/grpc` | v1.78.0 | indirect | Required by Qdrant client |
| `google.golang.org/protobuf` | v1.36.11 | indirect | Required by Qdrant client |

**Module name:** `testqdrant` — must be renamed to a proper path (e.g., `github.com/yashsam99/legalgpt`)

**Missing dependencies (needed for Phase 1):**
- `github.com/gin-gonic/gin` — HTTP router
- `github.com/spf13/viper` — config management
- `github.com/anthropics/anthropic-sdk-go` — Claude provider
- `google.golang.org/genai` — Gemini provider
- `github.com/jackc/pgx/v5` — PostgreSQL driver
- `github.com/redis/go-redis/v9` — Redis client
- `github.com/sourcegraph/conc` — structured concurrency
- `go.opentelemetry.io/otel` — OpenTelemetry
- `github.com/pressly/goose/v3` — database migrations

### Python — `ingest/requirements.txt`

| Dependency | Version Pin | Status | Notes |
|-----------|------------|--------|-------|
| `PyMuPDF` | >=1.23.0 | **KEEP** | PDF extraction, actively used |
| `requests` | >=2.28.0 | **KEEP** | HTTP client for Ollama API |
| `qdrant-client` | >=1.9.0 | **KEEP** | Qdrant Python client, actively used |

**No unused dependencies.**

**Missing dependencies (needed for Phase 1):**
- `pydantic>=2.0` + `pydantic-settings>=2.0` — config validation
- `structlog>=23.0` — structured logging
- `tqdm>=4.60` — progress bars
- `langdetect>=1.0.9` — language detection
- `pdfplumber>=0.10.0` — alternative PDF parser (fallback)
- `python-dotenv>=1.0` — .env file loading
- `cohere>=5.0` — Cohere embedding (Phase 1)
- `ruff` — linter (dev dependency)
- `pytest>=7.0` — testing (dev dependency)

---

## Section D — Risk Flags

### D.1 — No `context.Context` propagation (Go) — P0

**Files:** `main.go`, `rag.go`, `ollama.go`

- `rag.go:38` — `ctx := context.Background()` — creates a background context with no timeout; a Qdrant query could hang indefinitely
- `ollama.go:58-79` — `Embed()` method has no context parameter; HTTP POST to Ollama has no timeout (default `http.Client{}` has zero timeout)
- `ollama.go:82-104` — `Chat()` method has no context parameter; same infinite timeout risk
- `main.go:63-78` — No timeouts on embed, search, or chat calls; a hanging Ollama process blocks the REPL forever

**Fix:** Add `ctx context.Context` as first parameter to all network-calling functions. Configure `http.Client{Timeout: 30 * time.Second}` for LLM calls, 5s for Qdrant.

### D.2 — Swallowed `json.Marshal` errors (Go) — P0

**File:** `ollama.go`

- Line 59: `body, _ := json.Marshal(embedRequest{...})` — error discarded
- Line 87: `body, _ := json.Marshal(chatRequest{...})` — error discarded

These structs are simple and `json.Marshal` won't fail for them in practice, but the pattern is unsafe. If a field is added that can't be marshaled (e.g., `chan`, `func`), the nil body silently sends an empty POST.

**Fix:** Return error or use `must`-style helper.

### D.3 — `config.json` tracked in git with connection details — P0

**File:** `config.json`

Currently contains no secrets (all localhost), but:
1. The pattern encourages adding API keys to the same file
2. The file is tracked in git (not in `.gitignore`)
3. `config.go` loads from this file with no env var override

**Fix:** Delete `config.json`, add to `.gitignore`, create `.env.example` with keys-only template, switch to Viper for env-based config.

### D.4 — No graceful shutdown (Go) — P0

**File:** `main.go`

No signal handler. Ctrl+C during an LLM call leaves the process in an undefined state. When this becomes an HTTP server, in-flight requests will be dropped.

**Fix:** Add `signal.NotifyContext` for SIGTERM/SIGINT, server.Shutdown with 30s drain.

### D.5 — Module-level code execution in `pdf_parser.py` — P0

**File:** `ingest/pdf_parser.py:91-95`

```python
text = extract_pdf_text("constitution.pdf")
sections = parse_constitution(text)
print(sections[0])
```

This runs at import time. If any other module imports `pdf_parser`, it will crash if `constitution.pdf` doesn't exist in the current working directory. This is a latent import-time crash bug.

**Fix:** Wrap in `if __name__ == "__main__":` guard, or delete file (superseded by `chunker.py`).

### D.6 — No HTTP timeout on `http.Client` (Go) — P0

**File:** `ollama.go:51`

```go
return &OllamaClient{
    BaseURL: baseURL,
    http:    &http.Client{},
}
```

Default `http.Client` has **zero timeout** — requests can hang indefinitely if the Ollama server is unresponsive.

**Fix:** `&http.Client{Timeout: 30 * time.Second}` minimum.

### D.7 — No tenant isolation in Qdrant queries — P0

**File:** `rag.go:39-44`

```go
points, err := client.Query(ctx, &qdrant.QueryPoints{
    CollectionName: collection,
    Query:          qdrant.NewQuery(vector...),
    WithPayload:    qdrant.NewWithPayload(true),
    Limit:          qdrant.PtrOf(uint64(topK)),
})
```

No `Filter` field — queries return all points regardless of tenant. Per ARCHITECTURE.md §6 and COMPLIANCE_REGISTER, every query must include a mandatory `tenant_id` filter. Without this, multi-tenant deployment would leak data between tenants.

**Fix:** Add mandatory `tenant_id` filter to all Qdrant queries (inject at retriever level, not bypassable by callers).

### D.8 — No input sanitization — P0

**Files:** `main.go:51`, `rag.go:82-114`

User input goes directly from stdin to the embedding API and into the LLM prompt. No:
- Length limit (could send megabytes of text)
- Null byte stripping
- Prompt injection detection
- Input validation

**Fix:** Add sanitization: strip null bytes, limit to 2000 chars, check injection patterns in guardrail.

### D.9 — Silent config fallback (Go) — P1

**File:** `config.go:43`

```go
f, err := os.Open(path)
if err != nil {
    return cfg, nil // file missing → use defaults silently
}
```

If config file is missing or unreadable, silently uses defaults. In production, this means a misconfigured deployment runs with localhost defaults instead of failing fast.

**Fix:** For required config (API keys, DB URL), fail fast with descriptive error. For optional config with sensible defaults, log a warning.

---

## Section E — Reuse Decisions

### Logic blocks to PRESERVE (verbatim or near-verbatim)

| File | Lines | Logic | Reason |
|------|-------|-------|--------|
| `ollama.go:58-79` | `Embed()` HTTP call | Core Ollama embedding call — correct endpoint, payload, response parsing | Wrap in OllamaProvider struct implementing LLMProvider interface |
| `ollama.go:82-104` | `Chat()` HTTP call | Core Ollama chat call — correct endpoint, message format | Wrap in OllamaProvider, add streaming variant |
| `rag.go:31-73` | `SearchQdrant()` | Qdrant ANN search with payload extraction — correct use of go-client API | Move into QdrantStore, add tenant filter + timeout |
| `rag.go:82-114` | `BuildRAGPrompt()` | Prompt structure with system/user separation | Move to context assembler, generalise system prompt, add token budget |
| `config.go:11-20` | `Config` struct fields | Current config field names are fine | Extend with new fields, switch to Viper |
| `ingest/chunker.py:26-30` | `extract_text()` | PyMuPDF text extraction | Preserve, add pdfplumber fallback |
| `ingest/chunker.py:93-159` | `parse_constitution()` | Article-level Constitution parser with Part/Schedule tracking | Preserve core regex + iteration logic, extend with SAC and hierarchical metadata |
| `ingest/chunker.py:166-191` | `chunk_by_pages()` | Generic fallback chunker | Preserve as-is for unknown document types |
| `ingest/chunker.py:197-225` | `chunk_pdf()` entry point | Auto-detection + fallback logic | Preserve, extend with statute/judgment modes |
| `ingest/embedder.py:13-59` | `OllamaEmbedder` class | Model verification, single/batch embedding | Preserve core logic, add retry + provider abstraction |
| `ingest/qdrant_loader.py:26-109` | `QdrantLoader` class | Collection management + batch upsert | Preserve upsert logic, add retry + content-hash-based IDs + tenant_id |

### Logic blocks to REWRITE (with justification)

| File | Lines | Logic | Justification |
|------|-------|-------|---------------|
| `main.go` (entire) | 1-98 | CLI REPL | Must become HTTP server with Gin, SSE streaming, middleware chain, graceful shutdown. CLI logic is not reusable. |
| `config.go:22-51` | Config loading | JSON file loader | Must switch to Viper (env vars + .env + validation). Struct definition preserved, loading logic rewritten. |
| `ingest/pdf_parser.py` (entire) | 1-95 | Standalone constitution parser | Superseded by `chunker.py`. Has module-level execution bug. Delete entirely. |
| `rag.go:95-102` | System prompt | Constitution-specific prompt | Must be generalised for all Indian legal corpora (BNS, BNSS, BSA, Constitution, judgments). Templated with compliance disclaimers from COMPLIANCE_REGISTER.md. |

---

*Audit complete. This report must be committed before any code changes begin.*
