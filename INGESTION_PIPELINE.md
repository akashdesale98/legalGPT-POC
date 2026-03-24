# Indian Legal GPT — Data Ingestion Pipeline Specification

> Version: 1.0 | Date: 2026-04-02  
> Orchestration: Temporal (Go SDK) | Storage: S3 (ap-south-1) + Qdrant Cloud + RDS PostgreSQL

---

## 1. Document Acquisition

### 1.1 Source Registry

| Source | Content | Access Method | Rate Limit | Legal Risk | Mitigation |
|--------|---------|--------------|------------|-----------|-----------|
| **Gazette of India** (egazette.gov.in) | Acts, notifications, amendments | Public HTML — scraping permissible (government public domain) | None specified | Low — government publication | Prefer gazette for primary source statutes; cite gazette issue number in metadata |
| **eCourts** via Attestr CDN API | SC/HC judgments, orders | REST API (docs.attestr.com) | Per-plan quota (see Attestr pricing) | Low — official CDN, licensed access | Procure Attestr enterprise API subscription; async download jobs via ingest-service |
| **Indian Kanoon API** (api.indiankanoon.org) | Case law, fragment search, metadata | REST API — commercial license required | Per-subscription | Low with license; High without | Negotiate commercial license; attribution per IK terms; do NOT scrape IK website |
| **MCA Portal** (mca.gov.in) | Companies Act, rules, circulars | Public HTML (government) | None | Low | Scrape MCA only for publicly published Acts and rules; not private company data |
| **RBI** (rbi.org.in) | Monetary policy, Banking Regulation Act circulars | Public HTML (government) | None | Low | Statutory circulars and master directions only; not market-sensitive data |
| **SEBI** (sebi.gov.in) | Securities law circulars, SEBI Act | Public HTML (government) | None | Low | Published regulations and circulars only |
| **TRAI** (trai.gov.in) | Telecom regulations | Public HTML (government) | None | Low | Published regulations only |

**Critical Note:** Unauthorized scraping of Indian Kanoon, Manupatra, or SCC Online violates those platforms' Terms of Service and may attract liability under Section 43 of the IT Act, 2000 (unauthorized access to computer systems). All Indian Kanoon data must be obtained exclusively via the official API with a valid commercial license. Manupatra and SCC Online require a separate licensing negotiation — defer these sources to Phase 4 after legal counsel review.

### 1.2 Acquisition Implementation

**eCourts via Attestr:**
```go
// ingest-service/acquisition/attestr.go
type AttesteCDNClient struct {
    APIKey   string
    BaseURL  string  // https://api.attestr.com/v1
    RateLimit *rate.Limiter
}

func (c *AttestCDNClient) FetchJudgment(ctx context.Context, courtCode, caseNo, year string) ([]byte, error) {
    // GET /ecourts/orders?court_code={}&case_no={}&year={}
    // Returns: PDF binary or JSON with download URL
}
```

**Indian Kanoon API:**
```go
// ingest-service/acquisition/kanoon.go
type KanoonClient struct {
    APIKey  string
    BaseURL string  // https://api.indiankanoon.org
    // Per API terms: provide attribution, no bulk re-distribution
}

func (c *KanoonClient) SearchDocuments(ctx context.Context, query string, pageNum int) ([]KanoonDoc, error) {
    // POST /search/?formInput={query}&pagenum={pageNum}
}
func (c *KanoonClient) GetDocument(ctx context.Context, docID int) (KanoonDoc, error) {
    // POST /doc/{docID}/
}
```

**Gazette of India:**
```python
# ingest/acquisition/gazette_scraper.py
# Permitted: egazette.gov.in is a government public domain resource
# Fetch daily gazette notifications; parse Part II/III/IV for Acts and rules
```

---

## 2. Processing DAG (Temporal Workflow)

### 2.1 Primary Workflow: IngestDocumentWorkflow

```
IngestDocumentWorkflow(source, doc_id, raw_content, metadata)
│
├── Activity: Validate
│   ├── Verify file format (PDF/HTML/JSON)
│   ├── Check minimum content length (> 500 chars)
│   ├── Language detection (accept: en, hi, mr, ta, te, bn, kn)
│   └── On failure: move to quarantine bucket, notify admin
│
├── Activity: Deduplicate
│   ├── Compute SHA-256 of normalized text
│   ├── Query dedup_ledger table in PostgreSQL
│   ├── If hash exists AND effective_date unchanged: STOP (already indexed)
│   └── If hash exists AND effective_date changed: mark old chunks superseded, continue
│
├── Activity: Normalize
│   ├── PDF → text extraction (PyMuPDF; fallback to Tesseract OCR for scanned PDFs)
│   ├── OCR error rate check: if > 5% suspected OCR errors → human review queue
│   ├── Strip pagination artifacts (page numbers, running headers/footers)
│   ├── Remove section number formatting artifacts (e.g., "1 0 3" → "103")
│   ├── Separate footnotes into footnote_text field (do not mix into main body)
│   ├── Normalize whitespace and encoding (UTF-8)
│   └── Store normalized text to S3: s3://legal-corpus-{env}/normalized/{doc_id}.txt
│
├── Activity: PIIDetect
│   ├── Run Microsoft Presidio on normalized text
│   ├── Detect: phone numbers, Aadhaar numbers, PAN, email addresses, personal names in FIRs
│   ├── For statutes: expected to have zero PII — flag if PII detected (likely OCR error or wrong file)
│   ├── For judgments: redact party names in sensitive categories (POCSO, matrimonial) per SC guidelines
│   └── Store redacted text to S3: s3://legal-corpus-{env}/redacted/{doc_id}.txt
│
├── Activity: Chunk (SAC for statutes; sliding window for judgments)
│   └── [See Section 3 for chunking specification]
│
├── Activity: Embed
│   ├── Batch chunks (max 96 per API call to Cohere/Vyakyarth)
│   ├── input_type="search_document" (Cohere Embed v3)
│   ├── Rate limit: token bucket per embedding API quota
│   └── Store embeddings + metadata: Qdrant upsert batch
│
├── Activity: Upsert
│   ├── Qdrant batch upsert with payload metadata
│   ├── Set tenant_id = "public" for statutes; source-specific for proprietary
│   ├── Update dedup_ledger: mark doc_id as indexed, store chunk_count
│   └── Update ingestion_runs table with status, chunk_count, timestamp
│
├── Activity: Verify
│   ├── Random sample 5% of chunks: re-embed and verify cosine similarity ≥ 0.99 vs stored vector
│   ├── Verify chunk count matches expected statute section count (± 10%)
│   └── On failure: alert ingestion team, mark run as NEEDS_REVIEW
│
└── Activity: Notify
    ├── Publish event to Redis Streams: "new_content:{statute_name}:{effective_date}"
    ├── notification-svc consumes and fans out staleness alerts to subscribed users
    └── Invalidate Redis semantic cache entries for affected statute collections
```

### 2.2 Incremental Judgment Ingestion Workflow

```
IncrementalJudgmentIngestion(date: YYYY-MM-DD)
│
├── FetchDelta
│   ├── Query Attestr API: judgments published on {date} for SC + configured HCs
│   ├── Query Indian Kanoon API: new documents since last run (paginated)
│   └── Output: list of doc_ids to process
│
├── FilterNovel (parallel)
│   ├── For each doc_id: compute SHA-256 of normalized text
│   ├── Check dedup_ledger: skip if already indexed
│   └── Output: novel_doc_ids list
│
└── Fan-out IngestDocumentWorkflow
    └── Spawn one IngestDocumentWorkflow per novel doc (bounded: max 50 concurrent)
```

**Failure Handling:**
- Temporal's retry policy: max 5 retries with exponential backoff (2s, 4s, 8s, 16s, 32s)
- Flaky external APIs (Attestr, Indian Kanoon): retry on HTTP 429/503; dead-letter after max retries
- Heartbeat timeout: 30 minutes per activity (long-running PDF extraction)
- All failures stored in `ingestion_failures` table for manual review

---

## 3. Chunking Specification

### 3.1 Hierarchical SAC for Statutes

**Parser input:** Normalized statute text (BNS, BNSS, BSA, Constitution, Companies Act, etc.)

**Document tree structure:**
```
Act
└── Part (optional)
    └── Chapter
        └── Section
            └── Sub-section
                └── Clause
```

**Chunk generation algorithm:**

```python
def generate_sac_chunks(doc_tree: StatuteNode) -> list[Chunk]:
    chunks = []
    for section in doc_tree.iter_sections():
        # Generate parent summary (lightweight, ~100 tokens)
        parent_summary = f"""
Act: {section.act_name}
Part: {section.part or 'N/A'}
Chapter: {section.chapter}
Section {section.number}: {section.title}
Summary: {generate_brief_summary(section.text)}
"""
        # Create chunk with prepended parent context
        chunk_text = parent_summary + "\n\n" + section.text
        
        # If section is long (> 600 tokens), split at sub-section boundaries
        if token_count(chunk_text) > 600:
            for subsection in section.sub_sections:
                sub_chunk_text = parent_summary + "\n\n" + subsection.text
                chunks.append(Chunk(
                    text=sub_chunk_text,
                    metadata=build_metadata(subsection)
                ))
        else:
            chunks.append(Chunk(text=chunk_text, metadata=build_metadata(section)))
    
    return chunks
```

### 3.2 Sliding Window for Judgment Prose

```python
def chunk_judgment(judgment_text: str, metadata: JudgmentMetadata) -> list[Chunk]:
    tokens = tokenize(judgment_text)
    chunks = []
    window_size = 512
    overlap = 128
    
    for i in range(0, len(tokens), window_size - overlap):
        chunk_tokens = tokens[i : i + window_size]
        chunks.append(Chunk(
            text=detokenize(chunk_tokens),
            metadata={
                **metadata.to_dict(),
                "chunk_index": len(chunks),
                "is_judgment": True
            }
        ))
    
    return chunks
```

### 3.3 Metadata Schema per Chunk

```json
{
  "statute":         "Bharatiya Nyaya Sanhita, 2023",
  "part":            "Part I",
  "chapter":         "Chapter III — Offences Against the Human Body",
  "section":         "103",
  "sub_section":     null,
  "effective_date":  "2024-07-01",
  "superseded_by":   null,
  "source_url":      "https://egazette.gov.in/...",
  "content_hash":    "sha256:abc123...",
  "language":        "en",
  "tenant_id":       "public",
  "is_judgment":     false,
  "case_name":       null,
  "citation":        null,
  "court":           null,
  "judgment_date":   null,
  "statutes_cited":  []
}
```

For judgments, `is_judgment: true` and `case_name`, `citation`, `court`, `judgment_date`, `statutes_cited` are populated.

---

## 4. IPC → BNS Cross-Reference Data Model

### 4.1 PostgreSQL Mapping Table

```sql
CREATE TABLE ipc_bns_mapping (
    id              SERIAL PRIMARY KEY,
    old_code        TEXT NOT NULL,        -- 'IPC'
    old_section     TEXT NOT NULL,        -- '302'
    old_title       TEXT NOT NULL,        -- 'Murder'
    new_code        TEXT NOT NULL,        -- 'BNS'
    new_section     TEXT NOT NULL,        -- '103'
    new_title       TEXT NOT NULL,        -- 'Punishment for murder'
    mapping_type    TEXT NOT NULL,        -- 'equivalent' | 'partial' | 'split' | 'merged' | 'dropped'
    notes           TEXT,                 -- explanation for partial/split/merged cases
    source_ref      TEXT NOT NULL,        -- 'BPRD Comparative Table 2023'
    created_at      TIMESTAMPTZ DEFAULT NOW()
);

-- Same table covers CrPC → BNSS and Evidence Act → BSA mappings
CREATE INDEX ON ipc_bns_mapping (old_code, old_section);
CREATE INDEX ON ipc_bns_mapping (new_code, new_section);
```

**Seed data sources:**
- UP Police / BPRD BNS-IPC Correspondence Table (official government publication)
- Lawsikho IPC-to-BNS conversion table (cross-reference validation)
- Hugging Face dataset: `nandhakumarg/IPC_and_BNS_transformation`

**Mapping types:**
- `equivalent` — one-to-one correspondence with minor rewording
- `partial` — BNS section covers part of IPC section scope
- `split` — one IPC section split into multiple BNS sections
- `merged` — multiple IPC sections merged into one BNS section
- `dropped` — IPC section has no BNS equivalent (provision removed)

### 4.2 Query-Time Resolution Strategy

```go
// query-service/retrieval/intent_classifier.go

func DetectOldStatuteReference(query string) ([]StatuteRef, bool) {
    // Patterns: "IPC 302", "Section 302 IPC", "IPC section three hundred and two"
    // Also: "CrPC 161", "Section 27 Evidence Act"
    patterns := []regexp.Regexp{
        regexp.MustCompile(`\bIPC\s+(?:section\s+)?(\d+[A-Z]?)\b`),
        regexp.MustCompile(`\bSection\s+(\d+[A-Z]?)\s+(?:of\s+)?IPC\b`),
        regexp.MustCompile(`\bCrPC\s+(?:section\s+)?(\d+[A-Z]?)\b`),
        // ... more patterns
    }
    // Returns list of detected old statute references
}

func ResolveToNewCode(ctx context.Context, db *sql.DB, refs []StatuteRef) []MappingResult {
    // Query ipc_bns_mapping for each ref
    // Return exact BNS/BNSS/BSA equivalents
    // For 'split' or 'merged' types: return all relevant new sections
}
```

If old statute references detected → mapping results injected into system prompt:
```
IMPORTANT MAPPING CONSTRAINT: The user referenced IPC Section 302 which has been repealed. 
The equivalent provision under BNS 2023 is Section 103 (Punishment for murder). 
Your response MUST reference Section 103 BNS, not IPC 302.
```

### 4.3 UI Surface Pattern

In the frontend split-pane:
- Left panel: AI response uses new code (BNS 103)
- Right panel: Source document panel shows BNS 103 text
- Below source: Collapsible "Historical equivalent" section showing IPC 302 text with deprecation notice
- Badge: "Replaced IPC §302 — effective 01 July 2024"

---

## 5. Incremental Ingestion

### 5.1 Delta Detection

**Daily judgment feed (SC/HC):**
- Temporal cron workflow: runs daily at 02:00 IST
- Queries Attestr API for judgments published in last 24 hours
- Queries Indian Kanoon API: `/search/?formInput=date:[yesterday TO today]`
- SHA-256 hash of each normalized document text compared against `dedup_ledger`
- Only novel documents (new hash) enter the ingestion pipeline

**Statute amendments:**
- Weekly gazette scraper: scans egazette.gov.in for new notifications in relevant categories
- If amendment found for indexed statute: update `effective_date`, set `superseded_by` on old chunks, ingest new chunks

### 5.2 Re-Embedding Avoidance

```sql
-- dedup_ledger table
CREATE TABLE dedup_ledger (
    id              SERIAL PRIMARY KEY,
    doc_id          TEXT UNIQUE NOT NULL,
    content_hash    CHAR(64) NOT NULL,    -- SHA-256 of normalized text
    chunk_count     INT NOT NULL,
    indexed_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_date  DATE,
    superseded_at   TIMESTAMPTZ           -- set when new version replaces this
);
```

**Logic:** Before embedding, compute SHA-256 of normalized text. Check `dedup_ledger`:
- `Hash exists, superseded_at IS NULL` → skip (already current, no re-embedding needed)
- `Hash exists, superseded_at IS NOT NULL` → re-embed (superseded, new version incoming)
- `Hash not exists` → embed (new document)

This avoids re-embedding ~95% of documents in the daily incremental run.

---

## 6. Data Quality Gates

Every document must pass all gates before proceeding to chunking. Gate failures route to the appropriate queue.

| Gate | Check | Failure Action |
|------|-------|---------------|
| **File validity** | Valid PDF/HTML/JSON; minimum 500 chars after extraction | Quarantine queue |
| **OCR error rate** | < 5% suspected OCR errors (detected via regex: `l0ng`, `0rder`, lone consonants) | Human review queue |
| **Section number artifacts** | No section numbers with extra spaces (e.g., "1 0 3") — normalized to "103" | Auto-fix in normalization |
| **Footnote separation** | Footnotes identified and split into `footnote_text` field | Auto-fix |
| **Duplicate section detection** | No repeated section numbers within same act | Alert + manual review |
| **Language identification** | `langdetect` confirms language is one of: en, hi, mr, ta, te, bn, kn | Quarantine if unknown |
| **PII in public statute** | Zero PII expected in Acts; flag if detected (likely wrong file) | Alert + manual review |
| **Minimum chunk coherence** | Average chunk length > 100 tokens; no chunks with only whitespace | Re-normalize |
| **Embedding dimensionality** | Embedding vector is exactly 1024 dimensions (Cohere) or configured size | Reject + alert |

### Quality Metrics Dashboard

Tracked in `ingestion_runs` table and surfaced in Grafana:
- OCR error rate by document source
- Chunk coherence score (average token length per chunk)
- Dedup rate (% of incoming documents already indexed)
- Ingestion latency (time from acquire to Qdrant upsert)
- Verification pass rate

---

*Ingestion Pipeline v1.0 — Schedule full re-ingestion with SAC chunker after Phase 1 corpus is validated against RAGAS baseline.*
