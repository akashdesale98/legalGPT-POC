# Indian Legal GPT — Risk Register

> Version: 1.0 | Date: 2026-04-02  
> Review cadence: Monthly during Phases 1–2; quarterly thereafter  
> Likelihood: H = High (> 50% in 12 months), M = Medium (20–50%), L = Low (< 20%)  
> Impact: H = High (threatens viability), M = Medium (significant setback), L = Low (manageable)

---

| # | Risk | Category | Likelihood | Impact | Mitigation | Owner | Phase |
|---|------|----------|:----------:|:------:|-----------|-------|-------|
| 1 | **LLM hallucination causing legal harm** — AI generates incorrect statute citation or fabricates case law; advocate relies on it; court or regulator sanctions advocate | Legal/Compliance | M | H | Guardrails AI provenance validation on every response; confidence scoring with mandatory abstention below threshold; BCI-compliant review checkbox before document export; digital watermark on all AI drafts; RAGAS hallucination rate monitored in CI (alert if > 5%); standard disclaimer injected on all responses | Engineering | Phase 1 |
| 2 | **eCourts API access revocation** — Attestr CDN withdraws API service or eCourts changes access terms, blocking compliant judgment ingestion | Technical | M | H | Maintain local snapshots of SC judgment corpus in S3 (rolling 6-month archive); negotiate multi-year Attestr contract with SLA; build acquisition adapter layer so alternative providers can be swapped in without pipeline changes; maintain Indian Kanoon as parallel source for redundancy | Engineering | Phase 3 |
| 3 | **BCI regulation restricting AI legal advice** — Bar Council of India issues binding regulations prohibiting commercial AI legal research products or requiring registration | Legal/Compliance | M | H | Proactive engagement with BCI (Phase 3): submit product brief, request advisory opinion; position product as "AI research assistant" not "legal advisor" from Day 1; implement all current BCI guidelines preemptively; maintain compliance counsel on retainer from Phase 3 | Founders + Legal | Phase 3 |
| 4 | **Unauthorized scraping lawsuit (IT Act Section 43)** — Indian Kanoon or eCourts files complaint for unauthorized data access before compliant API migration is complete | Legal/Compliance | H | H | Do NOT initiate any scraping of Indian Kanoon or eCourts. Use only: Gazette of India (public domain), Attestr API (licensed), Indian Kanoon API (licensed). Current `ingest/` code must be reviewed against this policy before any production run. Engage legal counsel to review data acquisition plan before Phase 2 ingestion. | Engineering + Legal | Phase 2 |
| 5 | **Cross-tenant data leakage** — software bug or Qdrant misconfiguration exposes one law firm's research or uploaded documents to another tenant | Security | L | H | Qdrant payload-based partitioning (never collection-per-tenant); tenant_id filter mandatory in all Qdrant queries (enforced in Go code, not configurable by API); PostgreSQL Row-Level Security enforced at DB level (not just application); penetration test before enterprise client onboarding; DPDP breach notification SLA active | Engineering | Phase 1 |
| 6 | **Competitor (Harvey) aggressive India pricing** — Harvey announces India-specific pricing competitive with Pro tier to cut off addressable market | Market | M | M | Accelerate moat-building: IPC↔BNS mapping engine, eCourts integration, and Hindi/regional language support are India-specific and take Harvey months to replicate; differentiate on local workflow integrations (WhatsApp, district court kiosk); focus on tier-2/3 city advocate market Harvey's enterprise sales motion cannot efficiently serve; law college PLG generates loyalty before Harvey reaches students | Founders | Phase 3 |
| 7 | **Key person dependency** — 2-3 person team; one critical engineer leaving halts development | Operational | M | H | Comprehensive code documentation and ADRs (Architecture Decision Records); all services designed for 12-factor app standards for quick onboarding; no tribal knowledge in production configurations (all in code/config); recruit 4th engineer before Phase 3 agent development; cross-train each engineer on one additional service | Founders | Phase 1 |
| 8 | **Statute amendment lag (knowledge staleness)** — BNS or another key statute is amended; users receive answers citing superseded text for days/weeks before re-ingestion | Technical | H | M | Temporal cron: daily gazette scraper at 02:00 IST; staleness detection via `effective_date`/`superseded_by` chunk metadata; Redis cache invalidation on ingestion; UI staleness warning for affected responses; PagerDuty alert if ingestion cron fails for > 24 hours | Engineering | Phase 2 |
| 9 | **WhatsApp Business API policy change** — Meta changes WhatsApp Business API pricing, template requirements, or terms of service; WhatsApp bot becomes non-viable as GTM channel | Market | M | M | Do not build core product functionality exclusively on WhatsApp; treat WhatsApp as distribution channel, not feature. Maintain Telegram bot as parallel channel. Design WhatsApp adapter as swappable module in notification-svc. Monitor Meta policy announcements quarterly. | Engineering | Phase 2 |
| 10 | **LLM API rate limit exhaustion** — viral traffic spike or free tier abuse exhausts GPT-4o Mini or Claude Sonnet API quota; paying users experience degraded service | Technical | H | M | Bounded concurrency via `conc.ContextPool` (max 50 concurrent RAG queries per pod); Redis sliding window rate limiter per tenant tier enforced at API gateway; circuit breaker on LLM client (fail fast on 429); free tier hard cap (15 queries/month) prevents runaway costs; multi-provider routing — if one provider rate-limits, router can fail over to alternate | Engineering | Phase 1 |
| 11 | **Goroutine memory leaks under load** — unbounded goroutine spawning from concurrent RAG queries causes OOM crash or severe latency degradation | Technical | H | M | `sourcegraph/conc` pool.ContextPool with explicit goroutine ceiling from Day 1; no bare `go func()` in production code; context cancellation propagated on client disconnect; load test to 5× expected peak before Phase 2 launch; memory profiling (pprof) in staging environment | Engineering | Phase 1 |
| 12 | **Prompt injection jailbreaks** — malicious users override system prompt to extract proprietary data, generate fraudulent legal documents, or bypass BCI compliance controls | Security | M | H | System prompt injected via separate `system` parameter — never string concatenation; input sanitizer with regex + semantic classifier for known injection signatures; agent service: least-privilege tool access (read-only); all agent outputs to staging (no direct external actions); rate limiting on repeated failed injection attempts (auto-block after 5 attempts); security review of all prompt templates before production | Engineering | Phase 1 |
| 13 | **AWS cost overrun from DTO fees** — unexpected spike in Data Transfer Out costs from LLM API calls, CDN traffic, or cross-AZ database replication | Operational | H | M | Keep ECS tasks, RDS, Redis, and NAT Gateway within same AZ (ap-south-1a); route all LLM API calls through single NAT Gateway; use CloudFront for static assets and cached statute responses; set AWS Budget alert at $150/month in Phase 1 with auto-notification to founders; review DTO line item weekly during Phase 2 | Engineering | Phase 1 |
| 14 | **Embedding model quality regression** — switching from nomic-embed-text to Vyakyarth-1-Indic or Cohere causes unexpected drop in retrieval quality for English legal queries | Technical | M | M | A/B test new embedding model against golden test set before production rollout; RAGAS Context Precision must be ≥ baseline (within 2%) on English subset before switching; gradual rollout: new embedding model on new ingestion only; maintain old embedding vectors for 30-day rollback window; never switch embedding model and chunking strategy simultaneously | Engineering | Phase 3 |
| 15 | **DPDP Act non-compliance** — platform goes live without adequate consent management or data deletion mechanisms; Data Protection Board issues notice or fine | Legal/Compliance | M | H | DPDP CMS implemented in Phase 2 (Month 3) before any marketing to non-beta users; cryptographic deletion workflow tested before public launch; privacy policy reviewed by legal counsel before Phase 2; data residency verification via CloudTrail before onboarding enterprise clients; breach notification runbook documented and tested before Phase 2 | Engineering + Legal | Phase 2 |

---

## Risk Matrix

```
         IMPACT
         L       M       H
    H    —      [8,10,  [4,12,
                 11,13]   1,7,
L                        14,15]
I
K   M    —      [6,9]   [2,3,
E                        5,14]
L
I   L    —       —      [5]
H
O
O
D
```

**Highest priority risks (H likelihood + H impact):**
- #4 Unauthorized scraping lawsuit — **immediate action required** (review ingest code now)
- #10 LLM rate limit exhaustion — mitigated by design in Phase 1
- #11 Goroutine leaks — mitigated by conc library in Phase 1
- #13 AWS DTO overrun — monitor from Day 1

---

## Residual Risk After Mitigation

| Risk # | Residual Likelihood | Residual Impact | Acceptable? |
|--------|:------------------:|:---------------:|:-----------:|
| 1 (Hallucination) | L | M | Yes — guardrails reduce impact |
| 2 (eCourts API revocation) | L | M | Yes — S3 archive provides continuity |
| 3 (BCI regulation) | L | M | Yes — proactive compliance |
| 4 (Scraping lawsuit) | L | L | Yes — if compliant APIs used from Day 1 |
| 5 (Data leakage) | L | M | Yes — RLS + Qdrant partitioning |
| 6 (Harvey pricing) | M | L | Yes — moat-building mitigates |
| 7 (Key person) | M | M | Monitor — address by Phase 3 |
| 8 (Staleness) | L | L | Yes — daily ingestion + warnings |
| 9 (WhatsApp policy) | L | L | Yes — Telegram fallback |
| 10 (Rate limits) | L | L | Yes — circuit breaker + caps |
| 11 (Goroutine leaks) | L | L | Yes — conc library |
| 12 (Prompt injection) | L | M | Yes — sanitizer + least privilege |
| 13 (DTO overrun) | L | L | Yes — same-AZ architecture |
| 14 (Embedding regression) | L | L | Yes — A/B testing protocol |
| 15 (DPDP non-compliance) | L | M | Yes — CMS before public launch |

---

*Risk Register v1.0 — Review at Phase 2 kickoff and after any significant product, market, or regulatory change.*
