# Indian Legal GPT — Infrastructure Cost Model

> Version: 1.0 | Date: 2026-04-02  
> Region: AWS ap-south-1 (Mumbai) — primary  
> Currency: USD/month (infrastructure); INR where user-facing  
> Pricing basis: AWS public pricing + Qdrant Cloud pricing + LLM API public pricing as of Q1 2026  
> Note: Figures are estimates. Verify against current pricing pages before committing budgets.

---

## Assumptions by Phase

| Phase | DAU | Queries/day | Avg query tokens (in) | Avg output tokens | Embedding calls/day |
|-------|-----|------------|----------------------|-------------------|---------------------|
| Phase 1 | 50 | 150 | 2,000 | 800 | 150 (1 embed/query) |
| Phase 2 | 500 | 1,500 | 2,500 | 900 | 1,500 |
| Phase 4/5 | 10,000 | 30,000 | 3,000 | 1,000 | 30,000 |

**LLM routing split (Phase 2+):** 70% GPT-4o Mini (standard queries), 30% Claude Sonnet (complex/drafting).

---

## Cost Model Table (USD/month)

| Component | Phase 1 (~50 DAU) | Phase 2 (~500 DAU) | Phase 4/5 (~10K DAU) | Notes |
|-----------|:-----------------:|:------------------:|:--------------------:|-------|
| **Compute — AWS ECS Fargate Graviton3** | $40 | $120 | $450 | Phase 1: 2× 0.5 vCPU/1GB tasks (query + ingest). Phase 2: 4 services × 1 vCPU/2GB. Phase 4: auto-scaling pool across 6 services. Graviton3 (arm64) is 25-34% cheaper than x86 for Go workloads. |
| **Vector DB — Qdrant Cloud** | $0 | $25 | $200 | Phase 1: Free tier (1 node, 1GB RAM, ~1M vectors). Phase 2: Starter tier ($25/mo). Phase 4: Standard dedicated cluster with auto-scaling. |
| **Relational DB — Amazon RDS PostgreSQL** | $15 | $30 | $200 | Phase 1: db.t4g.micro Single-AZ (~$15/mo). Phase 2: db.t4g.small Multi-AZ (~$30/mo). Phase 4: db.m6g.large Multi-AZ + 1 read replica (~$200/mo). |
| **LLM — GPT-4o Mini (70% of queries)** | $1 | $8 | $150 | $0.15/1M input + $0.60/1M output tokens. Phase 1: 105 queries/day × 2,000 in + 800 out tokens × 30 days. Phase 4: 21,000 queries/day. |
| **LLM — Claude Sonnet 4.6 (30% of queries)** | $2 | $18 | $360 | $3.00/1M input + $15.00/1M output tokens. Phase 1: 45 queries/day at 3,500 in + 1,200 out tokens. Phase 4: 9,000 queries/day (drafting, complex reasoning). |
| **Embeddings — Cohere Embed v3** | $1 | $5 | $90 | $0.10/1M tokens. Phase 1: 150 queries/day × 512 tokens avg × 30 days = ~2.3M tokens/mo. Phase 4: 30,000 queries × 512 tokens × 30 days. |
| **Redis — ElastiCache Serverless** | $5 | $12 | $80 | Phase 1: cache.t4g.micro (~$5/mo). Phase 2: Serverless up to 1GB ($12/mo). Phase 4: Serverless 5GB auto-scaling. |
| **S3 — Legal corpus + exports** | $3 | $8 | $40 | ap-south-1: $0.025/GB-month. Phase 1: ~100GB corpus. Phase 4: ~1TB corpus + user exports. |
| **AWS Data Transfer Out (DTO)** | $5 | $20 | $120 | DTO from ap-south-1: $0.109/GB after 100GB free. Keep EC2, RDS, and Redis in same AZ to minimize cross-AZ charges. LLM proxy calls routed through single NAT Gateway. |
| **CloudFront CDN** | $2 | $8 | $60 | Static assets (Next.js build), cached statute PDFs. 100GB free/month; $0.014/GB (ap-south-1) above free tier. |
| **Load Balancer (AWS ALB)** | $5 | $5 | $20 | ALB: $0.008/LCU-hour + $16/month base. Fixed cost in early phases. |
| **Temporal Cloud (Workflow Orchestration)** | $0 | $25 | $100 | Phase 1: Self-hosted Temporal on Fargate (no additional cost). Phase 2: Migrate to Temporal Cloud when DAG complexity grows. Temporal Cloud: ~$25/month at low action volume. |
| **Monitoring — Grafana Cloud** | $0 | $0 | $50 | Free tier: up to 10K metrics, 50GB logs. Phase 4: Grafana Cloud Pro at ~$50/month for full retention + alerting. |
| **Ory Hydra (Auth) — self-hosted** | $0 | $0 | $10 | Runs as sidecar on ECS Fargate; minimal dedicated compute. Cost absorbed in compute line above. |
| **NAT Gateway** | $5 | $5 | $15 | Required for ECS Fargate tasks in private subnets to reach LLM APIs. $0.045/hour + $0.045/GB. Phase 1: minimal traffic; Phase 4: higher data volume. |
| **Secrets Manager + CloudTrail** | $2 | $3 | $5 | API key rotation, DPDP audit logging. CloudTrail: free for management events; ~$2/month for data events. |
| **Total Estimated Monthly** | **~$86** | **~$292** | **~$1,950** | |

---

## LLM Cost Sensitivity Analysis

**What if all Phase 4 queries use Claude Sonnet (no routing)?**

| Scenario | Monthly LLM Cost | Total Monthly |
|----------|-----------------|---------------|
| Routing (70% Mini + 30% Sonnet) | $510 | ~$1,950 |
| All Claude Sonnet | ~$5,400 | ~$7,800 |
| All GPT-4o Mini | ~$200 | ~$1,640 |

**Conclusion:** LLM routing is not optional — it reduces LLM costs by ~94% vs. all-Sonnet, enabling the Pro tier to be profitable at ₹999/month.

---

## Break-Even: Self-Hosted vs. Qdrant Cloud

| Metric | Self-Hosted Qdrant (EC2) | Qdrant Cloud Standard |
|--------|--------------------------|-----------------------|
| **Setup cost** | Engineering time (~40 hours) | Zero |
| **Ops burden** | Backup scripts, sharding, upgrades, monitoring | Managed by Qdrant |
| **EC2 cost (m7g.large, ap-south-1)** | ~$55/month | — |
| **Qdrant Cloud Standard** | — | ~$50/month (1 node) |
| **Break-even point** | Self-hosted saves ~$5/month vs. Cloud Standard | — |
| **Recommendation** | Use Qdrant Cloud until > 10M vectors OR > 3 dedicated shards needed | — |

**At 10M+ vectors (Phase 4 at scale):** Qdrant Cloud Enterprise or self-hosted on Graviton3 EC2 becomes economically meaningful. Evaluate at Phase 4 with actual usage data.

---

## Unit Economics

### Cost Per Query

| Phase | Total Monthly Cost | Queries/Month | Cost Per Query (USD) | Cost Per Query (INR) |
|-------|:-----------------:|:-------------:|:--------------------:|:-------------------:|
| Phase 1 | $86 | 4,500 | $0.019 | ~₹1.60 |
| Phase 2 | $292 | 45,000 | $0.0065 | ~₹0.54 |
| Phase 4/5 | $1,950 | 900,000 | $0.0022 | ~₹0.18 |

### Revenue Per User (ARPU) for Profitability

**Pro tier price:** ₹999/month = ~$12/month

**Assumptions:**
- Free tier users: 80% of registered users; 15 queries/month; GPT-4o Mini only
- Pro tier users: 20% of registered users; 500 queries/month; routing enabled
- Infrastructure cost modeled at Phase 2 scale (500 DAU)

| Scenario | Monthly Cost | Required Pro Users | Required ARPU |
|----------|:------------:|:-----------------:|:-------------:|
| Break-even at Phase 2 ($292/mo) | $292 | 25 pro users paying ₹999 (~$12) | ~$11.70/user |
| Break-even at Phase 4 ($1,950/mo) | $1,950 | 163 pro users at ₹999 | $11.96/user |
| Target: 30% gross margin | $1,950 cost → need $2,786 revenue | 232 pro users | $12/user |

**Minimum viable subscription count for profitability at Phase 4 scale:** ~230 Pro subscribers.

At 10,000 DAU with 5% paid conversion = 500 Pro subscribers × ₹999 = ₹4.99L/month (~$5,988) vs. ~$1,950 infrastructure cost. **Gross margin: ~67%.** This is the target state.

### Free Tier Cost

Free tier (15 queries/month × GPT-4o Mini):
- Cost: 15 × 2,000 tokens in × $0.00000015 + 15 × 800 tokens out × $0.0000006 ≈ $0.013/user/month
- At 10,000 free users: ~$130/month in LLM costs for free tier
- This is manageable — free tier is genuinely free from a unit economics perspective with the hard cap in place

**Without the hard cap** (unlimited free queries): at 10,000 DAU × 30 queries/day average = 300,000 queries/day = $33/day = ~$990/month in LLM alone for free users. **The 15-query hard cap is financially essential.**

---

## Phase Transition Triggers

| Trigger | Action |
|---------|--------|
| Monthly LLM cost > 40% of total infra cost | Audit query routing; consider tightening free tier cap |
| Qdrant vector count > 5M | Evaluate migration to Standard+ tier or dedicated shard |
| ECS task CPU > 70% sustained | Scale out (add task instances) |
| RDS IOPS > 80% | Add read replica or upgrade instance class |
| Monthly total cost > $500 | Conduct cost review; validate against MRR |
| Monthly MRR > $3,000 | Scale infrastructure to Phase 4 configuration |

---

*Cost Model v1.0 — Validate against actual AWS/Qdrant/LLM invoices at end of Phase 2 and adjust.*
