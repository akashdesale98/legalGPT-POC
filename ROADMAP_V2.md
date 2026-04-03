# Indian Legal GPT — Product Roadmap V2

> **Vision:** Build India's most comprehensive legal AI platform that beats global players on India-specific depth and pricing, delivered via cloud-hosted SaaS with optional on-premise deployment for enterprises.  
> **Last updated:** 2026-04-02 (incorporates Gemini 2.5 Pro architectural review)

---

## Strategy: Cloud-First SaaS with Hexagonal Architecture Core

**Current Phase (POC):** Ollama + local Qdrant to validate the RAG architecture at near-zero cost.

**Transition Plan:** By end of Phase 1 (Month 4), migrate to cloud infrastructure:
- **LLM:** Hexagonal provider interface (Ollama locally) → GPT-4o Mini (routing) + Claude Sonnet (drafting)
- **Vector DB:** Local Qdrant Docker → Qdrant Cloud Standard (managed, scalable)
- **Infra:** Local Docker → AWS ECS Fargate Graviton3 (ap-south-1); no Kubernetes until Phase 5+
- **DB:** SQLite POC → Amazon RDS PostgreSQL (Multi-AZ) with Row-Level Security from Day 1

**Why ECS Fargate over Kubernetes:** A 2-3 person team cannot sustainably operate Kubernetes cluster security, networking, and autoscaling alongside product development. ECS Fargate provides container orchestration, autoscaling, and HA at zero ops overhead. This decision is revisited at Phase 5 when dedicated DevOps headcount joins.

**Primary monetization:** Cloud SaaS subscriptions (freemium → pro → enterprise). On-prem is a niche enterprise add-on, not core business. Free tier is hard-capped at 15–20 queries/month to prevent LLM cost explosion.

---

## LLM Provider Abstraction Layer

**Designed from Day 1.** The backend must implement a Hexagonal Architecture (Ports & Adapters) pattern where the LLM is an external dependency, not a hardcoded implementation. This prevents a full codebase rewrite when switching providers and allows A/B testing of models in production.

### Go Interface (query-service/llm/provider.go)

```go
package llm

import (
    "context"
    "io"
)

// Response holds a complete LLM generation result.
type Response struct {
    Content    string
    TokensIn   int
    TokensOut  int
    ModelUsed  string
    Latency    time.Duration
}

// Token is a single streamed token from an LLM.
type Token struct {
    Text  string
    Done  bool
    Error error
}

// LLMProvider is the port — all LLM backends implement this interface.
type LLMProvider interface {
    // Complete sends a full prompt and waits for the complete response.
    Complete(ctx context.Context, prompt string, opts CompletionOptions) (Response, error)
    // Stream sends a prompt and returns a channel of tokens for SSE delivery.
    Stream(ctx context.Context, prompt string, opts CompletionOptions) (<-chan Token, error)
    // Embed generates vector embeddings for a batch of texts.
    Embed(ctx context.Context, texts []string) ([][]float32, error)
    // Name returns the provider identifier (e.g., "ollama", "anthropic", "openai").
    Name() string
}

// CompletionOptions controls inference parameters.
type CompletionOptions struct {
    Model       string
    MaxTokens   int
    Temperature float32
    SystemPrompt string
}
```

**Concrete Adapters:**
- `OllamaProvider` — POC/local dev; targets `localhost:11434`
- `AnthropicProvider` — Claude Sonnet 4.6 for complex drafting; uses `anthropic-go` SDK
- `OpenAIProvider` — GPT-4o Mini for standard routing; uses `openai-go` SDK

**Config-driven selection:** Provider selected via `LLM_PROVIDER` environment variable or per-tenant config in PostgreSQL. The LLM Router (see Architecture) selects the adapter at runtime based on query complexity classification.

---

## Phase 1 — Foundation (Months 1–4)

**Re-sequenced from original.** The original 3-month Phase 1 was architecturally impossible for 2-3 engineers. The corrected sequence prioritizes a stable backend before any frontend work.

**Goal:** Stable, tested Go API with LLM abstraction, BNS corpus, IPC↔BNS mapping, auth, and RAGAS baseline.

### 1.1 Hexagonal Architecture & LLM Abstraction (Months 1–2)

- [ ] Define `LLMProvider` interface in `query-service/llm/provider.go` (see interface above)
- [ ] Build `OllamaProvider`, `AnthropicProvider`, `OpenAIProvider` adapters
- [ ] Config-driven provider selection via environment variable
- [ ] Unit tests for each adapter with mocked HTTP responses

### 1.2 Go gRPC Microservices Skeleton (Months 1–2)

- [ ] Scaffold `query-service` (search, RAG orchestration, SSE streaming)
- [ ] Scaffold `ingest-service` (document pipeline, embedding, Qdrant upsert)
- [ ] Scaffold `auth-service` (JWT/OIDC, RBAC, tenant management)
- [ ] gRPC proto definitions for all service boundaries
- [ ] `sourcegraph/conc` pool.ContextPool for bounded worker pools in query-service
- [ ] SSE over HTTP/2 for chat token streaming (not WebSocket)

### 1.3 Backpressure & Concurrency Controls (Month 2)

- [ ] Worker pool with explicit goroutine ceiling (configurable via `MAX_CONCURRENT_RAG_QUERIES`)
- [ ] Context cancellation propagation from client disconnect → LLM API call cancellation
- [ ] Rate limiting at API Gateway: distributed sliding window in Redis, per-tier quotas
- [ ] Circuit breaker for external LLM API calls (fail fast on rate limit exhaustion)

### 1.4 PostgreSQL + Row-Level Security (Month 2)

- [ ] RDS PostgreSQL Multi-AZ in ap-south-1
- [ ] Enable RLS on all tenant-scoped tables from migration 0001
- [ ] Schema: `tenants`, `users`, `query_history`, `saved_research`, `ipc_bns_mapping`
- [ ] IPC↔BNS deterministic mapping table loaded from official UP Police / BPRD cross-reference PDFs

### 1.5 BNS Corpus Ingestion with SAC Chunking (Months 1–2)

- [ ] Temporal workflow DAG: acquire → validate → normalize → PII-detect → chunk (SAC) → embed → upsert → verify
- [ ] Summary-Augmented Chunking: parse statute tree (Part → Chapter → Section → Sub-section); generate parent summary; prepend to child chunk
- [ ] Metadata per chunk: `{statute, part, chapter, section, sub_section, effective_date, superseded_by, source_url, content_hash, language}`
- [ ] SHA-256 dedup ledger: embedding pipeline only triggered for novel hashes
- [ ] Ingest BNS (replaces IPC), BNSS (replaces CrPC), BSA (replaces Evidence Act), Constitution

### 1.6 Hybrid Retrieval Pipeline (Month 2)

- [ ] Dense vector search (Qdrant ANN) + BM25 sparse retrieval (exact statute citations)
- [ ] Cross-encoder reranking (Cohere Rerank v3 or BAAI/bge-reranker-v2-m3)
- [ ] Context assembly: token budget algorithm — top reranked chunks + immediate hierarchical parent nodes
- [ ] Query-time intent classifier: detect old statute citations → look up PostgreSQL mapping → inject as system prompt constraint

### 1.7 Auth & RBAC (Month 2)

- [ ] Ory Hydra (OIDC) for identity (self-hosted on ECS Fargate, low ops overhead vs. Keycloak)
- [ ] Short-lived JWTs (15-minute expiry) + HttpOnly refresh tokens
- [ ] Roles: `free`, `pro`, `enterprise`, `admin`
- [ ] Free tier enforcement: 15-query/month hard cap via Redis counter

### 1.8 RAGAS Evaluation Baseline (Month 2)

- [ ] Golden test set: 50 complex Indian legal queries with ground-truth answers and source citations
- [ ] RAGAS metrics integrated into CI pipeline: Context Precision, Faithfulness (Groundedness), Answer Relevance
- [ ] LLM-as-judge pattern using GPT-4o as evaluator against golden set
- [ ] Baseline scores recorded before any production deployment

### Milestone: Stable Go API with BNS+BNSS+BSA+Constitution, hybrid retrieval, IPC↔BNS mapping, auth, and RAGAS CI baseline. API tested via Postman/CLI — no frontend yet.

---

## Phase 2 — MVP & GTM Wedge (Months 3–4)

**Goal:** Ship to users. Launch WhatsApp bot as top-of-funnel before full web launch.

### 2.1 WhatsApp "BNS Quick Reference" Bot (Month 3)

- [ ] WhatsApp Business API integration (Meta Cloud API)
- [ ] Queries backed by same RAG pipeline as web API
- [ ] Hard limit: 10 queries/session for unauthenticated users
- [ ] Magic link upsell: complex queries (case law, drafting) → "View full analysis at [link]"
- [ ] Telegram bot with identical capability (parallel build, same pipeline)
- [ ] Voice message transcription (Bhashini Indic-Conformer for Indian languages)

### 2.2 Next.js Frontend MVP (Month 3–4)

- [ ] Split-pane citation verification UI: left = AI answer with inline citations; right = source document with exact chunk highlighted
- [ ] IPC cross-reference panel: when citing BNS section, show deprecated IPC equivalent
- [ ] Query history per authenticated session
- [ ] Responsive design (mobile-first, Android target)
- [ ] Closed beta launch — gather user feedback before public release

### 2.3 DPDP Act 2023 Consent Management (Month 3)

- [ ] Consent Management System (CMS): verifiable affirmative consent per processing activity
- [ ] Differentiate: core service delivery consent vs. data usage for model improvement
- [ ] User-facing dashboard: view and revoke consent instantly
- [ ] Auto-shred (cryptographic deletion): query history + uploaded docs on account termination or consent withdrawal
- [ ] Consent log with timestamp, IP, action — immutable audit trail

### 2.4 Qdrant Cloud Migration (Month 4)

- [ ] Migrate from local Docker Qdrant to Qdrant Cloud Standard tier (ap-south-1)
- [ ] Single shared collection with payload-based partitioning (`group_id = tenant_id`, `is_tenant: true`)
- [ ] Abandon collection-per-tenant model
- [ ] Caching layer: ristretto in-memory LRU (hot queries) → Redis semantic cache (cosine ≥ 0.98 → return cached response)
- [ ] Cache invalidation webhook: triggered on new gazette/judgment ingestion

### Milestone: Publicly accessible web app + WhatsApp bot. DPDP consent live. Qdrant Cloud migrated. Closed beta user feedback incorporated.

---

## Phase 3 — Depth & Drafting (Months 5–7)

**Goal:** Beat LawCentral on depth. Beat NeuroLaw on India focus.

### 3.1 Compliant Case Law Ingestion (Month 5)

- [ ] eCourts judgment download via Attestr CDN API (asynchronous, legally compliant)
- [ ] Indian Kanoon commercial API license negotiation — JSON metadata + fragment search
- [ ] Incremental ingestion DAG: delta detection on daily SC/HC judgment feeds
- [ ] SHA-256 dedup: skip re-embedding for unchanged chunks
- [ ] MinHash/LSH for near-duplicate detection across judgment corpus
- [ ] Target: 100,000+ SC/HC judgments indexed

### 3.2 Upgrade Embedding Model (Month 5)

- [ ] Replace nomic-embed-text with Vyakyarth-1-Indic (Krutrim AI Labs) or Cohere Embed v3 multilingual
- [ ] Input type optimization: `input_type="search_document"` for corpus, `input_type="search_query"` for queries
- [ ] Re-run RAGAS evaluation: confirm ≥ 5% improvement in Context Precision before full rollout
- [ ] A/B test: Vyakyarth vs. Cohere on Hindi legal query subset

### 3.3 Cross-Reference Engine (Month 5–6)

- [ ] Bidirectional link: statutes ↔ case law ↔ amendments
- [ ] Auto-surface related judgments for statute queries (e.g., Article 21 → Maneka Gandhi, Vishaka)
- [ ] "Related provisions" panel in UI
- [ ] Staleness detection: UI warning when citing content with non-null `superseded_by` field

### 3.4 Document Drafting with BCI Compliance (Month 6)

- [ ] Template engine: petitions, affidavits, legal notices, bail applications, contracts
- [ ] Mandatory UI friction before export: checkbox "I confirm I have personally reviewed this AI-generated draft and take full professional responsibility for its contents, in compliance with Bar Council of India guidelines."
- [ ] Digital watermark on exported PDFs: "AI-assisted draft — requires advocate review"
- [ ] Export to PDF / DOCX
- [ ] Guardrails AI provenance validation: generated text verified against retrieved statutory chunks before display

### 3.5 Hallucination Guardrails & Confidence Scoring (Month 6)

- [ ] Context relevance score computed per query (Faithfulness metric from RAGAS)
- [ ] Abstention threshold: if confidence < 0.65 → return "Insufficient authoritative context is available in the database to answer this legal query accurately. Please consult a qualified advocate."
- [ ] Log all abstention events with query hash (for golden set expansion)
- [ ] Guardrails AI integration: citation grounding validator on every LLM response

### 3.6 Multilingual Support (Month 7)

- [ ] Hindi query support (leverages Vyakyarth-1-Indic)
- [ ] UI language toggle (English / Hindi)
- [ ] Transliteration support for legal terms
- [ ] Deferred to Month 7: ensures core English/Hindi pipelines are flawless before cross-lingual complexity is introduced

### Milestone: Full Indian legal AI with case law (100K+ judgments), compliant drafting, hallucination guardrails, staleness detection, and Hindi support.

---

## Phase 4 — Moat Building (Months 8–12)

**Goal:** Build capabilities Harvey can't replicate and LawCentral can't match.

### 4.1 Workflow Agents with Safety Guardrails (Month 8–9)

- [ ] Multi-step agent chains for complex legal tasks:
  - "Analyze FIR → suggest BNS sections → cite precedents → draft bail application"
  - "Review contract → flag risky clauses → suggest amendments with legal basis"
  - "Research question across statutes + case law → generate memo with citations"
- [ ] **Agent safety requirements (non-negotiable):**
  - System prompt mathematically separated from user input (never concatenated as plain strings)
  - Input sanitization layer: regex + semantic classifier for known injection signatures
  - Principle of least privilege: agents have read-only access to vector DB and relational DB; no direct write access to external systems
  - All agent outputs terminate in secure staging area — explicit human advocate validation required before filing or export
  - Audit trail: every agent action logged with actor, timestamp, input hash, output hash
- [ ] Configurable agent templates
- [ ] Agent execution history in UI

### 4.2 e-Courts Integration (Month 9)

- [ ] Real-time case status via eCourts API
- [ ] Hearing date alerts + calendar sync
- [ ] Order/judgment auto-download into user's research library
- [ ] Case timeline visualization

### 4.3 Regulatory Compliance Tracker (Month 10)

- [ ] Auto-monitor: RBI circulars, SEBI orders, MCA notifications, TRAI regulations
- [ ] Alert subscribed users on changes relevant to their practice area
- [ ] AI-summarized regulatory updates with impact analysis
- [ ] Knowledge staleness notifications: "This statute was amended on [date]. Your saved research may be outdated."

### 4.4 On-Premise / Hybrid Deployment (Month 11)

- [ ] Docker Compose enterprise package: Qdrant + containerized API + cloud LLM calls (hybrid mode)
- [ ] Hybrid mode: on-prem vector DB + embeddings + cloud LLM for reasoning
- [ ] Data residency compliance documentation (DPDP Act 2023)
- [ ] SOC 2 Type II audit initiated
- [ ] Pricing: $50K+/year enterprise tier

### 4.5 API Marketplace (Month 12)

- [ ] REST API exposure for third-party integrations
- [ ] SDKs: Python, JavaScript, Go
- [ ] Usage-based API pricing
- [ ] Integration guides for case management tools

### Milestone: Full-stack legal AI with agents (safe), e-courts, regulatory tracker, on-prem option, and API marketplace.

---

## Phase 5 — Scale (Months 13–18)

**Goal:** Capture Indian legal market. Launch mobile. Institutional partnerships.

### 5.1 Pricing & Market Launch

- [ ] **Free tier:** Constitution + BNS basic search; hard cap 15 queries/month; GPT-4o Mini only
- [ ] **Pro tier** (~₹999–1999/month): Full corpus + case law + drafting + priority support
- [ ] **Enterprise tier** (~₹25,000–50,000/month per 10 seats): Agents + API + dedicated support + optional on-prem
- [ ] Qdrant Tiered Multitenancy: Tenant Promotion for enterprise clients requiring dedicated shards
- [ ] Launch: Product Hunt, Hacker News, Indian legal forums, Bar association newsletters

### 5.2 Mobile App (Android-First)

- [ ] Android app: online mode = full RAG pipeline via API
- [ ] Offline mode: ONNX Runtime + quantized all-MiniLM-L12-v2 + sqlite-vss for BNS/BNSS retrieval only (no LLM generation offline)
- [ ] Voice-first interface: Bhashini Indic-Conformer ASR for Hindi/regional courtroom dictation
- [ ] Push notifications: case updates, regulatory alerts, hearing reminders
- [ ] iOS follow-up after Android market validation

### 5.3 Law College Partnerships (PLG)

- [ ] Free access for law students (1700+ law colleges)
- [ ] Moot court preparation tools
- [ ] Campus ambassador program
- [ ] Legal research assignment helper

### 5.4 Bar Association & Distribution

- [ ] State bar council partnerships for distribution + endorsement
- [ ] District court kiosk deployments
- [ ] Legal aid integration
- [ ] CLE credit integration

### 5.5 Infrastructure Scale

- [ ] SOC 2 Type II and ISO 27001 certifications complete
- [ ] Multi-region Qdrant Cloud (India primary + disaster recovery)
- [ ] Global CDN (CloudFront) for static assets and cached responses
- [ ] Kubernetes evaluation: trigger migration only if ECS Fargate autoscaling cannot meet SLAs

### Milestone: Market-ready product with tiered pricing, mobile, institutional partnerships, and enterprise compliance certifications.

---

## Evaluation & Regression Pipeline

The RAG evaluation framework is a first-class engineering deliverable, not an afterthought. Every change to chunking strategy, embedding model, retrieval pipeline, or LLM prompt must be validated against the golden test set before merging.

### Golden Test Set Construction
- Minimum 100 queries at Phase 1, growing to 500 by Phase 4
- Coverage: statute lookup, cross-code comparison (IPC→BNS), case law retrieval, multi-statute reasoning, Hindi queries, edge cases (superseded statutes, conflicting judgments)
- Each query has: `question`, `ground_truth_answer`, `required_citations[]`, `should_abstain: bool`

### RAGAS Metrics (CI Gate)

| Metric | Definition | Phase 1 Gate | Phase 4 Gate |
|--------|-----------|-------------|-------------|
| **Context Precision** | Fraction of retrieved chunks that are relevant | ≥ 0.70 | ≥ 0.85 |
| **Faithfulness** | Fraction of answer claims grounded in retrieved context | ≥ 0.80 | ≥ 0.92 |
| **Answer Relevance** | Semantic similarity of answer to question | ≥ 0.75 | ≥ 0.88 |
| **Hallucination Rate** | % of responses containing ungrounded claims | ≤ 15% | ≤ 5% |
| **Citation Accuracy** | % of cited sections that exist and are correctly attributed | ≥ 90% | ≥ 98% |

### LLM-as-Judge Pattern
- GPT-4o evaluator scores each RAG response against the golden set
- Prompt: system role = "You are an expert Indian advocate evaluating legal AI responses for accuracy and groundedness."
- Scores stored in PostgreSQL `eval_runs` table; CI fails if any metric drops > 3% from baseline

### CI Integration
- `make eval` runs RAGAS suite against staging environment
- Required before merging any PR that touches `chunker.py`, `retrieval.go`, `prompt_templates/`, or `llm/`
- Evaluation results posted as PR comment via GitHub Actions

---

## Compliance Milestones

### DPDP Act 2023 Obligations by Phase

| Obligation | Phase | Deadline | Implementation |
|-----------|-------|----------|---------------|
| Explicit consent for data processing | Phase 2 | Month 3 | CMS with verifiable per-activity consent |
| Data minimization | Phase 1 | Month 2 | Collect only query text + user ID; no PII in vector DB |
| Data principal rights API (access/correct/delete) | Phase 2 | Month 4 | REST endpoints for data subject requests |
| Breach notification (72-hour SLA) | Phase 2 | Month 4 | Incident response runbook + automated alerting |
| Data residency verification | Phase 1 | Month 2 | All infra in ap-south-1; CloudTrail audit log |
| Auto-shred on consent withdrawal | Phase 2 | Month 3 | Cryptographic deletion job triggered by CMS event |
| Data fiduciary registration | Phase 3 | Month 7 | Legal counsel engagement (not engineering) |

### Bar Council of India (BCI) Guidelines

| Requirement | Implementation | Phase |
|------------|---------------|-------|
| Transparency about AI use | Persistent "AI-assisted" label on all responses | Phase 2 |
| Human judgment irreplaceable | Mandatory review checkbox before document export | Phase 3 |
| No independent strategic litigation decisions | Agents cannot directly file; staging + human approval required | Phase 4 |
| AI-generated submissions must be verified | Digital watermark on exported documents | Phase 3 |
| Confidence disclosure | Abstention message when confidence < threshold | Phase 3 |

---

## Staleness Detection

When statutes are amended or judgments overturned, the system must communicate knowledge currency explicitly.

**Chunk-level:** Every chunk has `effective_date` and `superseded_by` fields. When `superseded_by` is non-null, the chunk is marked stale.

**Response-level:** Context assembly checks `superseded_by` for all chunks used. If any stale chunk is included:
- UI: yellow warning banner — "This response references [Statute X, Section Y] which was amended on [date]. Please verify against the current gazette."
- API: `staleness_warning` field in response JSON with affected chunks and amendment dates.

**Incremental ingestion:** Temporal workflow runs daily; new gazette notifications trigger re-ingestion of affected statute sections + invalidation of Redis semantic cache entries referencing those sections.

---

## Agent Safety Guardrails

For Phase 4 workflow agents:

**Input Validation:**
- System prompt injected via separate `system` parameter (never string concatenation with user input)
- Input sanitizer: regex patterns for known jailbreak signatures + semantic classifier (fine-tuned on legal-domain adversarial examples)
- Maximum input length: 8,192 tokens; truncate with notification

**Principle of Least Privilege:**
- Agents have read-only access to Qdrant and PostgreSQL
- No direct filesystem write access
- No direct eCourts filing API access — output goes to staging table only
- Tool calls are enumerated in agent config; any non-whitelisted tool call → agent halted, event logged

**Action Confirmation:**
- All agent-generated documents require explicit human review before export/filing
- UI: "Review and Approve" workflow with diff view comparing AI draft against template
- Approval logged with advocate user ID, timestamp, and document hash

**Audit Trail:**
- Every agent step logged: `{agent_id, session_id, step, input_hash, output_hash, tool_called, timestamp, status}`
- Audit log is append-only (no UPDATE/DELETE on audit tables)
- Retention: 7 years (Indian legal record-keeping standard)

---

## Tech Stack Evolution (Updated)

| Layer | POC (Now) | Phase 1–2 (Foundation & MVP) | Phase 3–5 (Production) |
|---|---|---|---|
| **LLM** | Ollama llama3.2 | GPT-4o Mini (routing) + Claude Sonnet (drafting) via LLMProvider interface | Dynamic LLM Router; Anthropic/OpenAI; config-driven per tenant |
| **Embeddings** | nomic-embed-text (768d) | Vyakyarth-1-Indic or Cohere Embed v3 multilingual | Domain-adapted Indic embeddings (fine-tune on legal corpus if eval shows gap) |
| **Vector DB** | Qdrant Docker (local) | Qdrant Cloud (shared collection + payload partitioning) | Qdrant Cloud Standard/Enterprise (Tiered Multitenancy + Tenant Promotion) |
| **Backend** | Go CLI | Go gRPC microservices + conc worker pools + SSE streaming | Go microservices + Temporal (ingestion) + event bus |
| **Relational DB** | None | Amazon RDS PostgreSQL Multi-AZ (RLS enabled) | RDS PostgreSQL + read replicas + IPC↔BNS mapping |
| **Frontend** | Terminal REPL | Next.js split-pane citation UI (Vercel) | Next.js + React Native (SQLite VSS + ONNX Runtime offline) |
| **ASR** | None | Standard Web Audio API | Bhashini Indic-Conformer (AI4Bharat) |
| **Auth** | None | Ory Hydra (OIDC) + JWT + RBAC | OIDC + audit logging + DPDP CMS |
| **Infra** | Local Docker | AWS ECS Fargate Graviton3 (ap-south-1) | ECS Fargate + multi-AZ + CloudFront CDN |
| **Ingest** | Python scripts | Temporal DAG (Go SDK) + normalization + SHA-256 dedup | Temporal + Attestr CDN + Indian Kanoon API |
| **Caching** | None | ristretto LRU + Redis semantic cache (event-driven invalidation) | Multi-layer cache with TTL per content type |
| **Observability** | None | OpenTelemetry traces + Grafana Cloud | OTel + RAGAS CI + Grafana dashboards + PagerDuty alerts |
| **Compliance** | None | DPDP CMS + BCI UI friction + RLS | Guardrails AI + abstention scoring + SOC 2 Type II |

---

## Key Metrics to Track

| Metric | Phase 1 Target | Phase 3 Target | Phase 5 Target |
|---|---|---|---|
| Registered users | 500 | 10,000 | 100,000 |
| Daily active users | 50 | 1,000 | 10,000 |
| Statutes ingested | 4 | 20+ | 50+ |
| Judgments indexed | 0 | 100,000 | 500,000+ |
| Languages supported | 1 (English) | 2 (English + Hindi) | 7+ |
| Avg query latency (p50) | < 5s | < 3s | < 2s |
| Context Precision (RAGAS) | ≥ 0.70 | ≥ 0.80 | ≥ 0.85 |
| Faithfulness (RAGAS) | ≥ 0.80 | ≥ 0.88 | ≥ 0.92 |
| Hallucination rate | ≤ 15% | ≤ 8% | ≤ 5% |
| Citation accuracy | ≥ 90% | ≥ 95% | ≥ 98% |
| Monthly recurring revenue | ₹0 | ₹5L+ | ₹50L+ |
| Paid conversion rate | — | 3% | 5–8% |
| Cost per query | — | < ₹2 | < ₹1 |

---

## Immediate Next Steps (Corrected Priority Order)

1. **Define LLMProvider interface** — create `query-service/llm/provider.go` with complete interface; build OllamaProvider adapter first (keeps POC working)
2. **Scaffold gRPC services** — `query-service`, `ingest-service`, `auth-service` with proto definitions; no frontend yet
3. **PostgreSQL RLS migration** — set up RDS PostgreSQL locally (Docker), enable RLS on tenant tables from migration 0001
4. **IPC↔BNS mapping table** — load BPRD correspondence table into PostgreSQL; build intent classifier for old statute detection
5. **SAC chunking upgrade** — update `ingest/chunker.py` to hierarchical SAC approach; ingest BNS with new chunker
6. **RAGAS CI baseline** — 50 golden queries, RAGAS metrics in CI; establish baseline before any frontend work
7. **AnthropicProvider + OpenAIProvider adapters** — config-driven selection; test with real API keys
8. **Hybrid retrieval** — add BM25 sparse + cross-encoder reranking to `query-service`

*Frontend work begins in Month 3 after the API is stable and RAGAS-validated.*

---

*Last updated: 2026-04-02 | Incorporates Gemini 2.5 Pro review — 24 critical fixes, 16 improvements applied*
