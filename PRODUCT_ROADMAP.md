# Indian Legal GPT — Product Roadmap

> **Vision:** Build India's most comprehensive, privacy-first legal AI platform that beats global players on India-specific depth and local firms on AI quality.

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
| On-premise / local deployment | No | No | No | **Yes** |
| Regional language support | No | No | Hindi only | **Yes** |
| WhatsApp / Telegram bot | No | No | Yes | **Yes** |
| Workflow agents | Yes | No | No | **Yes** |
| Affordable for individual lawyers | No | No | Yes | **Yes** |
| e-Courts integration | No | No | Yes | **Yes** |

---

## Unfair Advantages

| Advantage | Why It Matters |
|---|---|
| **Privacy-first architecture** | Qdrant + Ollama runs 100% local. No Indian law firm wants client data on US servers. Harvey can never offer this. |
| **Go + Python stack** | Go for performance-critical API, Python for ML pipeline — the right stack for scale. |
| **India-native understanding** | Built from India, for India. Harvey is a US company opening a dev office. LawCentral is early. NeuroLaw is generic. |
| **New criminal code timing** | BNS/BNSS/BSA replaced IPC/CrPC/Evidence Act in 2024. Most lawyers are confused. An AI that deeply understands old-to-new mapping is immediately valuable. |
| **Existing RAG pipeline** | Article-level chunking, vector embeddings, and cited source retrieval already working — ahead of LawCentral's basic search. |

---

## Phase 1 — Foundation (Months 1-3)

**Goal:** Turn the CLI into a web-accessible product with expanded legal corpus.

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

### 3.1 On-Premise / Hybrid Deployment

- [ ] Productize local deployment (Qdrant + Ollama) as a one-click Docker Compose setup
- [ ] "Law Firm Edition" — runs entirely on firm's own infrastructure
- [ ] Hybrid mode: local vector DB + cloud LLM (for firms that want better answers but local data)
- [ ] Data residency compliance documentation (India DPDP Act 2023)

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

- [ ] **Free tier:** Constitution + BNS basic search (capture individual advocates)
- [ ] **Pro tier:** Full corpus + drafting + agents (~Rs 999-1999/month)
- [ ] **Enterprise tier:** On-premise + custom agents + SLA (~Rs 25,000+/month per seat)
- [ ] Launch on Product Hunt, Hacker News, Indian legal forums

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

### 4.4 Cloud-Hosted SaaS

- [ ] Managed cloud deployment for users who don't want to self-host
- [ ] Upgrade to cloud LLMs (Claude API / GPT-4) for higher-quality answers
- [ ] Keep local/on-premise as a parallel offering
- [ ] SOC 2 and ISO 27001 compliance

### 4.5 Mobile App

- [ ] Android-first (India market reality — 95%+ smartphone share)
- [ ] Offline mode with cached statutes and local embeddings
- [ ] Voice-first interface for courtroom use
- [ ] Push notifications for case updates and regulatory alerts
- [ ] iOS follow-up

### Milestone: Market-ready product with tiered pricing, institutional partnerships, mobile presence, and cloud + local deployment.

---

## Tech Stack Evolution

| Layer | POC (Now) | Phase 1-2 | Phase 3-4 |
|---|---|---|---|
| **LLM** | Ollama (llama3.2, local) | Ollama + cloud fallback | Cloud LLMs (Claude/GPT-4) + local option |
| **Embeddings** | nomic-embed-text (768d) | multilingual-e5-large | Fine-tuned Indian legal embeddings |
| **Vector DB** | Qdrant (Docker) | Qdrant (managed) | Qdrant Cloud + local option |
| **Backend** | Go CLI | Go HTTP API (Gin/Chi) | Go microservices + event bus |
| **Frontend** | Terminal REPL | Next.js web app | Next.js + React Native mobile |
| **Ingest** | Python scripts | Python pipeline + scrapers | Airflow/Temporal orchestrated pipelines |
| **Auth** | None | JWT | OAuth2 + RBAC |
| **Infra** | Local Docker | Docker Compose | Kubernetes (cloud) + Docker (on-prem) |

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

1. Convert Go CLI to HTTP API — wrap existing REPL logic in Gin/Chi HTTP handlers
2. Ingest BNS PDF — use existing pipeline with `--collection bns`
3. Build multi-collection search — modify `rag.go` to fan out queries
4. Spin up basic Next.js frontend — chat interface calling Go API

---

*Last updated: 2026-04-01*
