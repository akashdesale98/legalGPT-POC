-- +goose Up
CREATE TABLE ingest_jobs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source       TEXT NOT NULL,
    collection   TEXT NOT NULL,
    status       TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    chunk_count  INT,
    error        TEXT,
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ingest_docs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id        UUID REFERENCES ingest_jobs(id),
    doc_id        TEXT NOT NULL,
    content_hash  CHAR(64) NOT NULL,
    chunk_count   INT NOT NULL DEFAULT 0,
    status        TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'indexed', 'failed', 'superseded')),
    indexed_at    TIMESTAMPTZ,
    superseded_at TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_ingest_docs_hash ON ingest_docs(content_hash);
CREATE INDEX idx_ingest_docs_job ON ingest_docs(job_id);

CREATE TABLE ipc_bns_mappings (
    id           SERIAL PRIMARY KEY,
    old_code     TEXT NOT NULL,
    old_section  TEXT NOT NULL,
    old_title    TEXT NOT NULL,
    new_code     TEXT NOT NULL,
    new_section  TEXT NOT NULL,
    new_title    TEXT NOT NULL,
    mapping_type TEXT NOT NULL CHECK (mapping_type IN ('equivalent', 'partial', 'split', 'merged', 'dropped')),
    notes        TEXT,
    source_ref   TEXT NOT NULL DEFAULT 'BPRD Comparative Table 2023',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_ipc_bns_old ON ipc_bns_mappings(old_code, old_section);
CREATE INDEX idx_ipc_bns_new ON ipc_bns_mappings(new_code, new_section);

-- +goose Down
DROP TABLE IF EXISTS ipc_bns_mappings;
DROP TABLE IF EXISTS ingest_docs;
DROP TABLE IF EXISTS ingest_jobs;
