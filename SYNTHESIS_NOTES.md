# SYNTHESIS_NOTES.md — Gemini Review Triage Contract

> Generated: 2026-04-02  
> Basis: GEMINI_REVIEW.md (10 axes) × PRODUCT_ROADMAP.md  
> Team assumption: 2–3 senior engineers (1 backend, 1 frontend, 1 ML/data)

---

## Triage Table

| # | Axis | Finding | Classification | Action | Output File |
|---|------|---------|---------------|--------|-------------|
| 1 | RAG Architecture | Article-level chunking severs logical connections in hierarchical Indian statutes (definitions ↔ penalties ↔ conditions) | 🔴 CRITICAL | Replace with Summary-Augmented Chunking (SAC): parse statute tree → generate parent-node summary → prepend to child chunk | ROADMAP_V2, ARCHITECTURE, INGESTION_PIPELINE |
| 2 | RAG Architecture | `multilingual-e5-large` is inadequate for Indic orthographic/semantic complexity | 🔴 CRITICAL | Adopt Vyakyarth-1-Indic (Krutrim) or Cohere Embed v3 multilingual as primary embedding model | ROADMAP_V2, ARCHITECTURE |
| 3 | RAG Architecture | Cosine-similarity-only retrieval fails for exact statute citations (e.g., "Section 103 BNS") | 🔴 CRITICAL | Implement hybrid retrieval: dense vector + BM25 sparse + cross-encoder reranking before context assembly | ARCHITECTURE, ROADMAP_V2 |
| 4 | RAG Architecture | No LLM cost routing — frontier models at scale break Pro-tier unit economics | 🟡 IMPROVE | Implement LLM router: GPT-4o Mini for standard queries/routing; Claude 3.5 Sonnet for complex drafting | ROADMAP_V2, ARCHITECTURE, COST_MODEL |
| 5 | RAG Architecture | RAG-first architecture is the correct foundation | 🟢 CONFIRM | No change needed | — |
| 6 | RAG Architecture | Ollama for POC, cloud LLMs for production is architecturally sound | 🟢 CONFIRM | No change needed | — |
| 7 | Go Backend | Direct CLI → HTTP API without backpressure = goroutine exhaustion under load | 🔴 CRITICAL | Use `sourcegraph/conc` pool.ContextPool with explicit goroutine limits for worker pools | ARCHITECTURE, ROADMAP_V2 |
| 8 | Go Backend | Monolithic service = cannot isolate enterprise on-prem or scale independently | 🔴 CRITICAL | Decompose into gRPC microservices: query-svc, ingest-svc, draft-svc, agent-svc, auth-svc, notification-svc | ARCHITECTURE, ROADMAP_V2 |
| 9 | Go Backend | No caching strategy — legal queries have high long-tail redundancy | 🔴 CRITICAL | Dual-layer cache: ristretto (in-memory LRU) + Redis semantic cache (cosine ≥ 0.98 threshold); event-driven invalidation | ARCHITECTURE, ROADMAP_V2 |
| 10 | Go Backend | WebSocket vs SSE — WebSocket adds unnecessary bidirectional overhead for token streaming | 🟡 IMPROVE | Use SSE over HTTP/2 for token streaming (browser-native, lower overhead, aligns with Anthropic/OpenAI APIs) | ARCHITECTURE |
| 11 | Go Backend | Go is optimal for I/O-bound RAG orchestration | 🟢 CONFIRM | No change needed | — |
| 12 | Data Ingestion | Web scraping Indian Kanoon/eCourts likely violates IT Act 2000 Section 43 | 🔴 CRITICAL | Halt scraper approach. Use Attestr CDN API for eCourts judgment downloads; negotiate commercial license with Indian Kanoon API | INGESTION_PIPELINE, COMPLIANCE_REGISTER, RISK_REGISTER |
| 13 | Data Ingestion | IPC→BNS mapping via semantic similarity = hallucination risk for legal precision | 🔴 CRITICAL | Implement deterministic PostgreSQL relational mapping table; query-time intent classifier routes old citations to exact BNS equivalents | INGESTION_PIPELINE, ARCHITECTURE |
| 14 | Data Ingestion | Indian legal texts have severe OCR artifacts, inconsistent para numbering, unstructured footnotes | 🔴 CRITICAL | Add explicit normalization stage: regex stripping, footer/header removal, footnote separation, OCR error rate gate | INGESTION_PIPELINE |
| 15 | Data Ingestion | Use Temporal (not Airflow) for DAG — Go-native, superior retry policies for flaky external APIs | 🟡 IMPROVE | Replace Airflow reference with Temporal for ingestion orchestration | ROADMAP_V2, INGESTION_PIPELINE |
| 16 | Data Ingestion | MinHash/LSH deduplication for semantic near-duplicates | 🟡 IMPROVE | Add dedup stage before chunking using content hash (SHA-256) + LSH for near-duplicate detection | INGESTION_PIPELINE |
| 17 | Data Ingestion | SHA-256 incremental ingestion to avoid re-embedding unchanged chunks | 🟡 IMPROVE | Maintain dedup ledger; only trigger embedding pipeline if hash is novel | INGESTION_PIPELINE |
| 18 | Data Ingestion | IPC→BNS transition as primary product wedge is the strongest commercial strategy | 🟢 CONFIRM | Retain and amplify | — |
| 19 | Multi-Tenancy | Collection-per-tenant in Qdrant = memory overhead + cluster collapse at scale | 🔴 CRITICAL | Use single shared collection with payload-based partitioning (group_id = tenant_id, is_tenant: true); Tenant Promotion for enterprise | ARCHITECTURE, ROADMAP_V2 |
| 20 | Multi-Tenancy | No PostgreSQL Row-Level Security = software bug can expose tenant data | 🔴 CRITICAL | Enable RLS on all tenant-scoped tables in PostgreSQL from Phase 1 | ARCHITECTURE, COMPLIANCE_REGISTER |
| 21 | Multi-Tenancy | Basic JWT insufficient for enterprise — OIDC (Keycloak/Ory Hydra) required | 🟡 IMPROVE | Adopt Keycloak or Ory Hydra for OIDC; short-lived JWTs + HttpOnly refresh tokens | ARCHITECTURE, ROADMAP_V2 |
| 22 | Multi-Tenancy | Rate limiting must be tiered by subscription and enforced at network edge | 🟡 IMPROVE | Distributed sliding window algorithm in Redis at API Gateway, quotas per tier | ARCHITECTURE |
| 23 | Multi-Tenancy | On-premise as enterprise option is architecturally mature | 🟢 CONFIRM | No change needed | — |
| 24 | Compliance | DPDP Act 2023 obligations completely absent from roadmap | 🔴 CRITICAL | Build Consent Management System: verifiable affirmative consent per processing activity, revocation dashboard, auto-shred on termination | COMPLIANCE_REGISTER, ROADMAP_V2 |
| 25 | Compliance | BCI AI guidelines mandate transparency + human judgment; no UI friction for AI-generated submissions | 🔴 CRITICAL | Add mandatory review checkbox before document export; digital watermark on AI-generated docs | COMPLIANCE_REGISTER, ROADMAP_V2, ARCHITECTURE |
| 26 | Compliance | Hallucination guardrails insufficient — system prompt alone cannot prevent legal harm | 🔴 CRITICAL | Integrate Guardrails AI provenance validation; confidence scoring; hardcoded abstention fallback below threshold | ARCHITECTURE, ROADMAP_V2, COMPLIANCE_REGISTER |
| 27 | Compliance | Need programmatic abstention (not just prompt-based) | 🟡 IMPROVE | Implement context-relevance score gate: if score < threshold → return abstention message, log event | ARCHITECTURE |
| 28 | Compliance | AWS Mumbai deployment for data residency is a compliance advantage | 🟢 CONFIRM | No change needed | — |
| 29 | Frontend/UX | Mobile offline with full LLM + 768d embeddings infeasible on mid-range Android | 🔴 CRITICAL | Offline = ONNX Runtime + quantized all-MiniLM-L12-v2 + sqlite-vss for retrieval only; full reasoning requires connectivity | ROADMAP_V2, ARCHITECTURE |
| 30 | Frontend/UX | Generic ASR (Whisper) fails on Indian code-switching and regional dialects | 🔴 CRITICAL | Integrate Bhashini Indic-Conformer (AI4Bharat) for courtroom voice interface | ROADMAP_V2, ARCHITECTURE |
| 31 | Frontend/UX | Citation UI must link to highlighted text in source panel, not just append URL | 🟡 IMPROVE | Split-pane UI: left = AI response with inline citations; right = source document with exact text highlighted + IPC cross-reference | ARCHITECTURE, ROADMAP_V2 |
| 32 | Frontend/UX | WhatsApp bot as GTM wedge before full web app launch | 🟡 IMPROVE | Launch WhatsApp "BNS Quick Reference Pocket Bot" first; magic link upsell to web for complex tasks | ROADMAP_V2 |
| 33 | Frontend/UX | Android-first priority is correct for India | 🟢 CONFIRM | No change needed | — |
| 34 | Phasing | Phase 1 (3 months) for API + LLM + frontend + 4 statutes + auth is unrealistic for 2-3 engineers | 🔴 CRITICAL | Re-sequence: Months 1-2 = backend/API/auth/ingestion only; Month 3-4 = frontend + closed beta; Months 4-5 = depth; Month 6 = expansion | ROADMAP_V2 |
| 35 | Phasing | No LLM abstraction layer = massive refactor when switching providers | 🔴 CRITICAL | Hexagonal Architecture from Day 1: LLMProvider interface with OllamaAdapter, AnthropicAdapter, OpenAIAdapter | ROADMAP_V2, ARCHITECTURE |
| 36 | Phasing | Roadmap does not mention evaluation pipeline at all | 🔴 CRITICAL | Add RAGAS evaluation framework into CI/CD pipeline with golden test set from Day 1 | ROADMAP_V2, ARCHITECTURE |
| 37 | Phasing | POC on Ollama before cloud commitment is correct | 🟢 CONFIRM | No change needed | — |
| 38 | Competitive Moat | Freemium with no query ceiling = LLM cost explosion on viral spike | 🔴 CRITICAL | Hard cap: 15-20 queries/month on free tier; free tier uses GPT-4o Mini exclusively | ROADMAP_V2, COST_MODEL |
| 39 | Competitive Moat | True moat = proprietary data structures + local integrations, not LLM capabilities alone | 🟡 IMPROVE | Prioritize IPC↔BNS mapping engine, eCourts integration, local templates as defensible assets | ROADMAP_V2 |
| 40 | Competitive Moat | WhatsApp bot → magic link → web platform conversion funnel | 🟡 IMPROVE | Bot design: teaser snippet + magic link for complex tasks requiring full web platform | ROADMAP_V2 |
| 41 | Competitive Moat | Law college campus ambassador program is strong PLG | 🟢 CONFIRM | No change needed | — |
| 42 | Competitive Moat | ₹999-1999/month Pro tier is well-positioned for tier-2 city advocates | 🟢 CONFIRM | No change needed | — |
| 43 | Infrastructure | No egress cost model — DTO fees often become third-largest cloud expense | 🔴 CRITICAL | Keep EC2/Redis/RDS in same AZ; proxy external LLM calls through optimized layers; model DTO in cost estimates | COST_MODEL, ROADMAP_V2 |
| 44 | Infrastructure | CLI → Docker → Kubernetes jump requires dedicated DevOps the team lacks | 🔴 CRITICAL | Use AWS ECS Fargate (Graviton3/arm64 m7g family) — serverless, no K8s management, 25-34% better price-perf for Go | ROADMAP_V2, ARCHITECTURE, COST_MODEL |
| 45 | Infrastructure | Self-hosted Qdrant on EC2 viable at POC; managed Qdrant Cloud better past 1M vectors | 🟡 IMPROVE | Migrate to Qdrant Cloud Standard when corpus > 1M chunks (~Phase 2) | COST_MODEL, ROADMAP_V2 |
| 46 | Infrastructure | AWS Mumbai region satisfies DPDP data residency | 🟢 CONFIRM | No change needed | — |
| 47 | Missing Capabilities | No RAG evaluation framework — changes to chunking/embedding/LLM = flying blind | 🔴 CRITICAL | Integrate RAGAS (Context Precision + Faithfulness) into CI/CD; golden test set of complex Indian legal queries; LLM-as-judge pattern | ROADMAP_V2, ARCHITECTURE |
| 48 | Missing Capabilities | Phase 3 agents have no safety guardrails — prompt injection risk | 🔴 CRITICAL | Separate system prompt from user input mathematically; input sanitization for injection signatures; least-privilege agent tools; all outputs to staging, not direct filing | ROADMAP_V2, ARCHITECTURE |
| 49 | Missing Capabilities | No staleness detection for amended statutes or overturned precedents | 🟡 IMPROVE | Implement effective_date + superseded_by metadata per chunk; UI warning when citing superseded content | INGESTION_PIPELINE, ROADMAP_V2 |
| 50 | Missing Capabilities | Kubernetes in Phase 3+ requires DevOps not on team | ⚪ REJECT | Use ECS Fargate throughout Phase 1-3; evaluate Kubernetes only if autoscaling needs outgrow Fargate at Phase 4. Reason: K8s ops burden exceeds 2-3 person team capacity | ROADMAP_V2 |

---

## Summary Counts

| Classification | Count |
|---------------|-------|
| 🔴 CRITICAL (must apply) | 24 |
| 🟡 IMPROVE (should apply) | 16 |
| 🟢 CONFIRM (validates roadmap) | 11 |
| ⚪ REJECT (out of scope) | 1 |
| **Total** | **52** |

---

## Rejected Recommendation Detail

| # | Finding | Reason for Rejection |
|---|---------|---------------------|
| 50 | Kubernetes in Phase 3+ | A 2-3 person founding team cannot responsibly manage Kubernetes cluster operations, custom networking, pod autoscaling, and security hardening simultaneously with product development. AWS ECS Fargate provides 90% of K8s benefits (container orchestration, scaling, HA) at zero ops overhead. Migration to Kubernetes is a Phase 5+ consideration triggered by dedicated DevOps headcount, not a timeline milestone. |

*Edit contract complete. All output files may now be written.*
