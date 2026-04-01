# Indian Legal GPT — Product Roadmap

> **Vision:** Build India's most comprehensive legal AI platform that beats global players on India-specific depth and pricing, delivered via cloud-hosted SaaS with optional on-premise deployment for enterprises.

---

## Strategy: Cloud-First SaaS with Temporary POC Stack

**Current Phase (POC):** We use Ollama + local Qdrant to validate the RAG architecture and product-market fit at near-zero infrastructure cost.

**Transition Plan:** By end of Phase 1 (Month 3), migrate to cloud infrastructure:
- **LLM:** Ollama → Claude API / GPT-4 (100x better answer quality, proven for legal reasoning)
- **Vector DB:** Local Qdrant Docker → Qdrant Cloud (managed, scalable, multi-region)
- **Infra:** Local VMs → Cloud (AWS/GCP Mumbai) for reliability, compliance, and global reach

**Why this matters:**
- Ollama is a prototyping tool, not a production system. For a legal AI serving thousands of users, answer quality is non-negotiable.
- Cloud hosting ensures India data residency (DPDP Act 2023 compliance), SOC 2, and uptime that individual law firms can't achieve on their own.
- Optional on-prem deployment (Phase 3+) remains available as a $50K+/year premium tier for enterprises with extreme data sensitivity.

**Primary monetization:** Cloud SaaS subscriptions (freemium → pro → enterprise). On-prem is a niche enterprise add-on, not our core business.

---

## Competitive Landscape

### 1. Harvey AI — [harvey.ai](https://www.harvey.ai)

- **Valuation:** $11B (backed by Sequoia Capital & GIC)
- **Market:** Global enterprise — US, UK, expanding to India (Bengaluru office 2025)
- **Products:** Assistant, Vault (document store), Knowledge (research), Workflow Agents, Ecosystem integrations
- **India Partners:** PwC, Shardul Amarchand Mangaldas & Co., S&A Law Offices
- **Pricing:** Enterprise contracts ($$$$) — inaccessible to 90%+ of Indian lawyers
- **Weakness:** US-centric, cloud-only (no on-premise), extremely expensive, no deep Indian statute coverage

### 2. NeuroLaw AI — [neurolaw.ai](https://neurolaw.ai)

- **Parent:** Jinvaani AI / Om Rishabhdev Technologies Pvt. Ltd. (India-origin)
- **Market:** Enterprise legal teams
- **Products:** AI-powered legal research, intelligent document drafting, compliance automation
- **Tech:** Next.js, enterprise SaaS
- **Rating:** 4.9/5 (50 reviews)
- **Weakness:** Generic platform with no visible India-specific depth — no BNS/BNSS support, no court integration, no regional language support

### 3. LawCentral AI — [lawcentral.ai](https://lawcentral.ai)

- **Parent:** Partwigo Labs (New Delhi, founded 2024)
- **Market:** Individual Indian lawyers and advocates
- **Products:** AI legal research, case management, e-Courts integration, WhatsApp Legal Assistant
- **Coverage:** Supreme Court & High Court databases, BNS & BNSS support
- **Languages:** English + Hindi
- **Pricing:** Freemium (free app on Google Play Store)
- **Weakness:** Early-stage, limited AI depth (basic search, not RAG), thin team (Gmail-based support), no document drafting, no on-premise option

### Competitive Gap Summary

| Capability | Harvey | NeuroLaw | LawCentral | **Indian Legal GPT (Target)** |
|---|:---:|:---:|:---:|:---:|
| Deep Indian statute coverage | Partial | No | Yes | **Yes** |
| Case law (SC/HC judgments) | No | No | Yes | **Yes** |
| New criminal codes (BNS/BNSS/BSA) | No | No | Partial | **Yes** |
| RAG-based AI answers | Yes | Unknown | No | **Yes** |
| Document drafting | Yes | Yes | No | **Yes** |
| Cloud SaaS (multi-tenant) | Yes | Yes | No | **Yes** |
| Regional language support | No | No | Hindi only | **Yes** |
| WhatsApp / Telegram bot | No | No | Yes | **Yes** |
| Workflow agents | Yes | No | No | **Yes** |
| Affordable for individual lawyers | No | No | Yes | **Yes** |
| e-Courts integration | No | No | Yes | **Yes** |
| Optional on-premise (enterprise) | No | No | No | **Yes** |

---

## Unfair Advantages

| Advantage | Why It Matters |
|---|---|
| **Deep India-first design** | Every feature, every statute, every judgment selection is built for India. Harvey is global + India; LawCentral and NeuroLaw lack depth. |
| **Go + Python stack** | Go for high-performance cloud API, Python for ML pipelines and ingest. This is the right architecture for rapid scale. |
| **Exceptional product timing** | BNS/BNSS/BSA replaced IPC/CrPC/Evidence Act in 2024. 90% of Indian lawyers are still confused. An AI that deeply understands the transition is immediately valuable. |
| **RAG-first architecture** | Article-level chunking, semantic search, and cited source retrieval built from day one. LawCentral relies on basic search; we deliver reasoning. |
| **Cloud-native from Phase 1** | Moving to cloud LLMs and managed infrastructure early prevents technical debt. By Phase 3, we'll have proven, scalable cloud infra while on-prem remains an enterprise option. |

---

## Phase 1 — Foundation (Months 1-3)

**Goal:** Turn the CLI into a web-accessible product with expanded legal corpus. Initial iteration on Ollama; migrate to cloud LLMs by end of phase.

**Infrastructure Note:** Start with local Ollama for rapid prototyping, then switch to Claude API / GPT-4 by Month 2-3 to ensure production-grade answer quality and scalability.

### 1.1 Web API Layer

- [ ] Build a Go REST API on top of existing `rag.go` + `ollama.go` (Gin or Chi router)
- [ ] Endpoints: `/api/search`, `/api/chat`, `/api/collections`, `/api/health`
- [ ] Streaming response support for chat (SSE or WebSocket)
- [ ] Rate limiting and request logging

### 1.2 Frontend MVP

- [ ] Next.js chat UI with citation side-panel
- [ ] Display article numbers, relevance scores, and full source text
- [ ] Query history per session
- [ ] Responsive design (mobile-friendly from day one)

### 1.3 Expanded Legal Corpus

- [ ] Ingest **Bharatiya Nyaya Sanhita (BNS)** — replaced IPC
- [ ] Ingest **Bharatiya Nagrik Suraksha Sanhita (BNSS)** — replaced CrPC
- [ ] Ingest **Bharatiya Sakshya Adhiniyam (BSA)** — replaced Indian Evidence Act
- [ ] Ingest **Constitution of India** (already done)
- [ ] Add old-to-new section mapping metadata (e.g., IPC 302 -> BNS 103)

### 1.4 Multi-Collection Search

- [ ] Modify `rag.go` to query across multiple Qdrant collections simultaneously
- [ ] Weighted/ranked results merging across collections
- [ ] Collection filter in API (search specific statute or all)

### 1.5 Auth & Tenancy

- [ ] User registration and login (JWT-based)
- [ ] Per-user query history and saved research
- [ ] Basic role system (free / pro / admin)

### Milestone: Deployable web app with Constitution + BNS + BNSS + BSA, multi-statute search, and user accounts.

---

## Phase 2 — Differentiation (Months 4-6)

**Goal:** Beat LawCentral on depth. Beat NeuroLaw on India focus.

### 2.1 Supreme Court & High Court Judgments

- [ ] Build scrapers for Indian Kanoon / eCourts APIs
- [ ] Ingest landmark SC and HC judgments as new collections
- [ ] Extract metadata: case name, citation, bench, date, statutes cited
- [ ] Incremental ingestion pipeline for new judgments

### 2.2 Cross-Reference Engine

- [ ] Auto-surface related judgments when answering statute questions
  - e.g., Article 21 query -> Maneka Gandhi v. Union of India, Vishaka v. State of Rajasthan
- [ ] Link statutes <-> case law <-> amendments bidirectionally
- [ ] "Related provisions" panel in UI

### 2.3 Multilingual Support

- [ ] Switch to multilingual embedding model (`multilingual-e5-large` or equivalent)
- [ ] Support Hindi, Marathi, Tamil, Telugu, Bengali, Kannada queries
- [ ] Transliteration support for legal terms
- [ ] UI language toggle

### 2.4 Document Drafting

- [ ] Template-based legal document generation using LLM + retrieved context
- [ ] Document types: petitions, affidavits, legal notices, bail applications, contracts
- [ ] Editable drafts with citation insertion
- [ ] Export to PDF / DOCX

### 2.5 WhatsApp & Telegram Bot

- [ ] WhatsApp Business API integration for legal research queries
- [ ] Telegram bot with inline citation support
- [ ] Same RAG pipeline backing all channels
- [ ] Voice message transcription + query (accessibility for non-typing users)

### Milestone: Full Indian legal AI platform with case law, drafting, multilingual support, and messaging bots.

---

## Phase 3 — Moat Building (Months 7-12)

**Goal:** Build capabilities that Harvey can't replicate and LawCentral can't match.

### 3.1 On-Premise / Hybrid Deployment (Optional Enterprise Feature)

- [ ] Productize optional on-premise deployment for enterprise law firms requiring strict data residency
- [ ] Docker Compose setup for firms that want to self-host (Qdrant + containerized API + cloud LLM calls)
- [ ] Hybrid mode: on-premise vector DB + embeddings + cloud LLM for reasoning (balances performance + data privacy)
- [ ] Data residency compliance documentation (India DPDP Act 2023)
- [ ] **Note:** Primary monetization is cloud SaaS; on-prem is $50K+/year premium tier for enterprises only

### 3.2 Workflow Agents

- [ ] Multi-step agent chains for complex legal tasks:
  - "Analyze this FIR -> suggest relevant BNS sections -> cite precedents -> draft bail application"
  - "Review this contract -> flag risky clauses -> suggest amendments with legal basis"
  - "Research this legal question across statutes + case law -> generate memo with citations"
- [ ] Configurable agent templates for common workflows
- [ ] Agent execution history and audit trail

### 3.3 e-Courts Integration

- [ ] Real-time case status tracking via eCourts API
- [ ] Hearing date alerts and calendar sync
- [ ] Order and judgment auto-download
- [ ] Case timeline visualization

### 3.4 Regulatory Compliance Tracker

- [ ] Auto-monitor RBI circulars, SEBI orders, MCA notifications, TRAI regulations
- [ ] Alert subscribed users on relevant changes
- [ ] Summarize regulatory updates with AI
- [ ] Impact analysis: "How does this circular affect your practice area?"

### 3.5 API Marketplace

- [ ] Expose legal AI as REST APIs for third-party integrations
- [ ] SDKs for Python, JavaScript, Go
- [ ] Usage-based API pricing
- [ ] Integration guides for case management and practice management tools

### Milestone: Full-stack legal AI platform with agents, integrations, compliance, and enterprise deployment options.

---

## Phase 4 — Scale (Months 12-18)

**Goal:** Capture the Indian legal market.

### 4.1 Pricing & Launch

- [ ] **Free tier:** Constitution + BNS basic search (capture individual advocates, no credit card)
- [ ] **Pro tier** (~Rs 999-1999/month): Full statute corpus + case law search + document drafting + priority support
- [ ] **Enterprise tier** (~Rs 25,000-50,000/month per 10 seats): Custom workflows + API access + dedicated support + optional on-premise deployment
- [ ] Launch on Product Hunt, Hacker News, Indian legal forums
- [ ] **Monetization:** Primary revenue from SaaS subscriptions; API usage fees for integrations; premium enterprise on-prem option

### 4.2 Law College Partnerships

- [ ] Free access for law students (1700+ law colleges in India = massive funnel)
- [ ] Moot court preparation tools
- [ ] Legal research assignment helper
- [ ] Campus ambassador program

### 4.3 Bar Association & Distribution

- [ ] Partner with state bar councils for distribution and endorsement
- [ ] District court kiosk deployments
- [ ] Legal aid integration (free access for pro-bono cases)
- [ ] CLE (Continuing Legal Education) credit integration

### 4.4 Cloud Infrastructure & Scale

- [ ] Deploy on AWS / GCP with India (Mumbai) as primary region for data residency
- [ ] Managed Qdrant Cloud for vector DB (auto-scaling, backups, replication)
- [ ] Claude API / GPT-4 as core LLM (proven quality for legal reasoning)
- [ ] SOC 2 Type II and ISO 27001 compliance for enterprise trust
- [ ] Global CDN for fast response times (critical for mobile users)
- [ ] Cost optimization: usage-based billing aligns infrastructure cost with revenue

### 4.5 Mobile App

- [ ] Android-first (India market reality — 95%+ smartphone share)
- [ ] Offline mode with cached statutes and local embeddings
- [ ] Voice-first interface for courtroom use
- [ ] Push notifications for case updates and regulatory alerts
- [ ] iOS follow-up

### Milestone: Market-ready product with tiered pricing, institutional partnerships, mobile presence, and cloud + local deployment.

---

## Tech Stack Evolution

**Strategy:** POC on local stack (Ollama) to validate RAG architecture. Phase 1+ migrates to cloud-first SaaS architecture.

| Layer | POC (Now) | Phase 1-2 (Transition) | Phase 3+ (Production) |
|---|---|---|---|
| **LLM** | Ollama llama3.2 (local) | Claude API / GPT-4 (primary) + Ollama fallback | Claude API / GPT-4 (optimized for legal reasoning) |
| **Embeddings** | nomic-embed-text (768d) | multilingual-e5-large (cloud-hosted) | Fine-tuned legal embeddings (Qdrant or similar) |
| **Vector DB** | Qdrant Docker (local) | Qdrant Cloud managed service | Qdrant Cloud + multi-region replication |
| **Backend** | Go CLI (single-threaded) | Go HTTP API (Gin/Chi), cloud-ready | Go microservices (search, draft, agents) + event bus |
| **Frontend** | Terminal REPL | Next.js web app (Vercel) | Next.js + React Native mobile (self-hosted or Vercel) |
| **Ingest** | Python scripts (manual runs) | Python pipeline + scheduled jobs | Airflow / Temporal for orchestrated ingestion |
| **Auth** | None | JWT + basic roles | OAuth2 + RBAC + audit logging |
| **Infra** | Local Docker VM | Docker Compose / simple VPS | Kubernetes (GCP / AWS) + managed services |
| **Data Residency** | Local machine | India Cloud (AWS/GCP Mumbai region) | Multi-region with India primary |

**Key transition point:** End of Phase 1 (~Month 3) — move from Ollama to cloud LLMs to ensure production-grade quality and scale.

---

## Key Metrics to Track

| Metric | Phase 1 Target | Phase 4 Target |
|---|---|---|
| Registered users | 500 | 100,000 |
| Daily active users | 50 | 10,000 |
| Statutes ingested | 4 | 50+ |
| Judgments indexed | 0 | 500,000+ |
| Languages supported | 1 (English) | 7+ |
| Avg query latency | < 5s | < 2s |
| Monthly recurring revenue | Rs 0 | Rs 50L+ |
| Paid conversion rate | - | 5-8% |

---

## Immediate Next Steps (This Week)

1. Convert Go CLI to HTTP API — wrap existing REPL logic in Gin/Chi HTTP handlers (keep Ollama for now)
2. Ingest BNS PDF — use existing pipeline with `--collection bns`
3. Build multi-collection search — modify `rag.go` to fan out queries across collections
4. Set up Claude API integration — add as secondary LLM backend (parallel to Ollama)
5. Spin up basic Next.js frontend — chat interface calling Go API
6. Plan cloud LLM migration — identify cost vs. quality trade-offs for Claude vs. GPT-4 by end of Phase 1

---

*Last updated: 2026-04-01*
