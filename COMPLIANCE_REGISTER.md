# Indian Legal GPT — Compliance Register

> Version: 1.0 | Date: 2026-04-02  
> Scope: India — DPDP Act 2023, IT Act 2000, Bar Council of India AI Guidelines  
> Owner: Founding team (legal counsel to be engaged by Phase 3)

---

## 1. DPDP Act 2023 Obligation Map

The Digital Personal Data Protection Act, 2023 imposes obligations on "Data Fiduciaries" (entities that determine purpose and means of processing personal data). Indian Legal GPT is a Data Fiduciary for all user personal data processed on the platform.

| Obligation | Section (DPDP Act) | Applicable Phase | Implementation | Owner | Status |
|-----------|-------------------|-----------------|---------------|-------|--------|
| **Explicit, informed, affirmative consent** before processing personal data | §6 | Phase 2 (Month 3) | Consent Management System (CMS): granular consent per processing activity (core service delivery vs. model improvement). No pre-ticked boxes. | Engineering | Planned |
| **Purpose limitation** — data used only for consented purpose | §6(2) | Phase 2 | CMS tracks purpose per data element. Model improvement processing disabled by default; opt-in only. | Engineering | Planned |
| **Data minimization** — collect only what is necessary | §8(3) | Phase 1 (Month 2) | Collect: query text, user ID, timestamp. Do NOT log: IP address (unless security incident), device fingerprint, or non-essential metadata. | Engineering | In Progress |
| **Data accuracy** — maintain accurate personal data | §8(4) | Phase 2 | User profile edit endpoints; ability to correct stored query history if attributed incorrectly. | Engineering | Planned |
| **Data retention limits** | §8(7) | Phase 2 | Query history: auto-delete after 2 years (configurable). Uploaded documents: delete 90 days after last access or on demand. Audit logs: 7 years (legal requirement). | Engineering | Planned |
| **Data principal right to access** | §11 | Phase 2 (Month 4) | `GET /api/v1/user/data-export` — returns all stored personal data as JSON/PDF. Fulfilled within 72 hours. | Engineering | Planned |
| **Data principal right to correction** | §12 | Phase 2 | `PATCH /api/v1/user/data` — user can update profile data. | Engineering | Planned |
| **Data principal right to erasure** | §12 | Phase 2 (Month 3) | `DELETE /api/v1/user/account` — triggers cryptographic deletion of all personal data within 24 hours. PostgreSQL RLS ensures scoped deletion. | Engineering | Planned |
| **Grievance redressal mechanism** | §13 | Phase 2 | Designated grievance officer (founding team member). Contact: grievance@[domain]. Response within 30 days. | Operations | Planned |
| **Breach notification to Data Protection Board** | §8(6) | Phase 2 | Incident response runbook: detect → contain → assess → notify DPB within 72 hours. PagerDuty alert on anomalous data access patterns. | Engineering + Operations | Planned |
| **Children's data protection** | §9 | Phase 2 | Age verification on registration (self-declared + legal disclaimer). No targeted profiling of users under 18. Law students may use platform with parental consent attestation. | Engineering + Legal | Planned |
| **Cross-border data transfer restrictions** | §16 | Phase 1 | All data stored exclusively in AWS ap-south-1 (Mumbai). LLM API calls to Anthropic/OpenAI transmit query text — user consent obtained for this processing. CloudTrail audit log for all data locations. | Engineering | In Progress |
| **Data fiduciary registration** with Data Protection Board | §§ (Section numbers pending final Rules notification by MeitY; Rules not yet gazetted as of April 2026) | Phase 3 (Month 7) | Engage legal counsel to assess registration threshold and file registration once Rules are notified. Monitor MeitY gazette for Rules publication. | Legal Counsel | Not Started |
| **Privacy Notice** — clear and plain language | §5 | Phase 1 (at launch) | Privacy policy drafted before first public user onboarding. Must explicitly mention: query text processed by third-party LLM APIs (OpenAI, Anthropic). | Legal + Engineering | Planned |

### Consent Management System (CMS) Architecture

```
User Registration / First Login:
  → Consent modal (non-dismissible without action)
  → Separate consent checkboxes for:
     [✓] Core service delivery (required — service unavailable without)
     [ ] Using my queries to improve the AI model (optional, opt-in)
     [ ] Receiving product updates and legal news (optional, opt-in)
  → Consent stored in PostgreSQL: {user_id, purpose, granted_at, ip_hash, revoked_at}

User Account Settings → Privacy:
  → View all active consents with grant dates
  → Revoke any optional consent with immediate effect
  → "Delete My Account" button → triggers cryptographic deletion workflow

Cryptographic Deletion Workflow (Temporal):
  → Soft-delete user record (set deleted_at timestamp)
  → Overwrite all personal data fields with random bytes
  → Delete Qdrant vectors tagged with user's tenant_id (uploaded docs)
  → Delete Redis cache entries for tenant
  → Log deletion event to immutable audit table
  → Complete within 24 hours of request
```

---

## 2. Bar Council of India (BCI) AI Guidelines

The Bar Council of India has issued guidelines governing the use of AI tools by advocates in India (2026). These guidelines impose constraints on how AI-generated legal content may be used professionally.

### 2.1 Applicable BCI Clauses and Constraints

| BCI Requirement | Constraint on Product | Implementation | Phase |
|-----------------|----------------------|---------------|-------|
| **Transparency about AI use** — users must know they are interacting with AI | All AI-generated content must be clearly labeled | Persistent "AI-assisted" label on every response; cannot be hidden by user CSS or settings | Phase 2 |
| **Human professional judgment is irreplaceable** — AI cannot make final legal decisions | Document export requires explicit advocate review and acceptance of responsibility | Mandatory review checkbox: "I confirm I have personally reviewed this AI-generated draft and take full professional responsibility for its contents" — enforced at backend API level | Phase 3 |
| **No AI-generated submissions without advocate review** | AI cannot directly file documents to eCourts or any tribunal | Agent-service outputs are staged; `review_acknowledged: true` required before any export or filing action. Backend enforces this — not a frontend-only check. | Phase 4 |
| **Citation accuracy and verifiability** | AI must not present unverifiable citations | Guardrails AI provenance validation on every response; all citations verified against corpus before display | Phase 3 |
| **Confidence disclosure** | Users must know when AI is uncertain | Abstention fallback at confidence < 0.65; explicit uncertainty disclosure in responses that are answered but have lower confidence (0.65–0.80 range) | Phase 3 |
| **Digital watermark on AI documents** | Exported documents must indicate AI assistance | PDF/DOCX exports include footer: "This document was prepared with AI assistance (Indian Legal GPT). It has been reviewed and approved by [Advocate Name, Enrollment No.]" | Phase 3 |
| **No independent litigation strategy decisions** | AI advises; advocate decides | System prompt engineering: AI role is "research assistant" not "legal advisor." All responses include disclaimer: "This is AI-assisted research, not legal advice. Consult a qualified advocate for legal decisions." | Phase 1 (from launch) |
| **No fabrication of case law** | AI must not hallucinate judgments | Citation grounding + abstention threshold. Hallucination rate monitored via RAGAS; alert if > 5%. | Phase 3 |

### 2.2 Required Disclaimers

**Standard response disclaimer (injected by system, non-removable):**
```
⚠ AI Research Assistant: This response is generated by AI and is intended for research assistance only. 
It is not legal advice. All cited statutes and judgments should be verified against primary sources. 
Consult a qualified advocate before taking any legal action.
```

**Abstention message (triggered when confidence < 0.65):**
```
Insufficient authoritative context is available in the database to answer this legal query accurately. 
The retrieved materials do not provide a definitive answer. Please consult a qualified advocate 
or refer to the primary statute/judgment directly.
```

**Low confidence advisory (triggered when confidence 0.65–0.80):**
```
Note: This response is based on limited matching context. Confidence score: [X]%. 
Please verify the cited provisions against the official gazette before relying on this information.
```

### 2.3 Abstention Decision Tree

```
Query received
    │
    ▼
Run RAG pipeline → compute Faithfulness score (RAGAS)
    │
    ├── Score < 0.65 → ABSTAIN
    │       └── Return abstention message (no generated answer)
    │
    ├── Score 0.65–0.80 → ANSWER WITH LOW-CONFIDENCE ADVISORY
    │       └── Return answer + confidence score + verification reminder
    │
    └── Score > 0.80 → ANSWER WITH STANDARD DISCLAIMER
            └── Return answer + standard disclaimer
            │
            └── Guardrails AI citation validation
                    ├── All citations grounded → Return response
                    └── Ungrounded citation detected → 
                            └── Remove ungrounded claim → Return response with warning:
                                "Note: One or more references could not be verified against 
                                 the indexed corpus and have been removed from this response."
```

---

## 3. Data IP Risk Register

### 3.1 Source-by-Source Licensing Risk

| Source | Licensing Status | Risk Level | Mitigation |
|--------|-----------------|-----------|-----------|
| **Gazette of India** (egazette.gov.in) | Government of India publication — public domain under Government of India's open data policy | **Low** | Preferred primary source for all Acts and rules. Cite gazette issue/notification number in metadata. |
| **eCourts Portal** (ecourts.gov.in) | Government public portal — court orders are public documents. Attestr CDN provides licensed API access. | **Low** (with Attestr subscription) | Use Attestr CDN API; do not scrape eCourts portal directly. |
| **Indian Kanoon** (indiankanoon.org) | Independent aggregator. Commercial API available at api.indiankanoon.org with explicit terms (attribution, no bulk redistribution). | **Medium** (scraping = high; API = low) | Use only official API with paid license. Comply with attribution requirements. Do NOT scrape website. |
| **Manupatra** | Commercial publisher. No public API. Strict licensing terms. | **High** | Do NOT access without a commercial licensing agreement. Defer to Phase 4 after legal counsel review. |
| **SCC Online** (Supreme Court Cases) | Commercial publisher (EBC). No public API. | **High** | Do NOT access without licensing agreement. Defer to Phase 4. |
| **MCA Portal** (mca.gov.in) | Government — public Acts and rules are public domain. | **Low** | Scrape only published Acts and Ministerial rules, not company filings. |
| **RBI / SEBI / TRAI** | Government regulators — published circulars are public documents. | **Low** | Published master directions, circulars, and regulations only. |

### 3.2 Fair Use Analysis

For content obtained via Indian Kanoon API:
- The Indian Kanoon Terms (api.indiankanoon.org/terms/) permit API usage for defined commercial purposes
- Attribution is required in the user interface where content is displayed
- Bulk redistribution of the raw text corpus is prohibited
- Our use case (display to authenticated paying users in a research interface) is within permitted scope
- Legal counsel review required before signing commercial license agreement

For government publications (gazette, MCA, RBI, SEBI, eCourts):
- These are public documents. Section 52(1)(q) of the Copyright Act, 1957 permits reproduction of government works.
- Acts, rules, and notifications enacted by Parliament/State Legislatures are in the public domain.
- Court judgments and orders are public documents (eCourts guidelines confirm public access).

---

## 4. Liability Architecture

### 4.1 Confidence Scoring → Abstention Pipeline

The platform's liability exposure is directly related to the quality and groundedness of its responses. The following architecture limits liability by ensuring the platform never presents uncertain information with false confidence.

**Confidence Score Computation:**
```python
# For each RAG response:
confidence_score = (
    0.40 * context_precision_score +    # Did we retrieve relevant chunks?
    0.40 * faithfulness_score +          # Is the answer grounded in context?
    0.20 * citation_verification_score   # Are all citations verifiable?
)
# Range: 0.0 (complete fabrication) to 1.0 (fully grounded)
```

**Decision Table:**

| Confidence Score | Action | User Communication |
|-----------------|--------|-------------------|
| < 0.65 | Mandatory abstention | Abstention message (see Section 2.2) |
| 0.65 – 0.80 | Answer with advisory | Low-confidence advisory injected |
| > 0.80 | Standard answer | Standard disclaimer injected |
| Any score, ungrounded citation detected | Remove ungrounded claim | Citation warning injected |

### 4.2 Audit Log

Every query, response, confidence score, and user action is logged to an immutable audit table:

```sql
CREATE TABLE query_audit_log (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL,
    tenant_id        UUID NOT NULL,
    query_text_hash  CHAR(64) NOT NULL,  -- SHA-256 of query (not stored in plaintext)
    response_hash    CHAR(64) NOT NULL,
    confidence_score NUMERIC(5,4),
    abstained        BOOLEAN NOT NULL DEFAULT FALSE,
    hallucination_flagged BOOLEAN NOT NULL DEFAULT FALSE,
    collections_searched TEXT[] NOT NULL,
    latency_ms       INT NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
-- Append-only: no UPDATE or DELETE permitted (enforced via trigger)
```

**Retention:** 7 years (standard for Indian legal records).

**Purpose:** In the event of a legal claim that the platform generated harmful advice, the audit log allows reconstruction of the exact query, confidence score, and whether the appropriate disclaimer was shown.

### 4.3 When the System Refuses to Answer

The system refuses (abstains) in the following cases:
1. Confidence score < 0.65
2. Query contains explicit request for strategic litigation advice ("What legal strategy should I use?")
3. Query asks for advice on an active criminal matter where identification of parties is evident
4. Query contains explicit personal data of third parties (DPDP protection)
5. Prompt injection detected by input sanitizer (for agent workflows)
6. User's free tier query limit exceeded (HTTP 402 with upgrade prompt)

---

*Compliance Register v1.0 — Legal counsel engagement required by Month 7 (Phase 3) for DPDP registration, Manupatra/SCC licensing, and Bar Council liaison.*
