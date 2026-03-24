-- Indian Legal GPT — Canonical Database Schema
-- Driver: PostgreSQL 16+ with Row-Level Security
-- ORM: sqlc (generates Go query layer from this schema)

-- =============================================================================
-- Users & Tenants
-- =============================================================================

CREATE TABLE tenants (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    tier        TEXT NOT NULL DEFAULT 'free' CHECK (tier IN ('free', 'pro', 'enterprise')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE users (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    email       TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    role        TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

CREATE INDEX idx_users_tenant ON users(tenant_id);
CREATE INDEX idx_users_email ON users(email);

-- =============================================================================
-- Sessions & Consent
-- =============================================================================

CREATE TABLE sessions (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at  TIMESTAMPTZ NOT NULL
);

CREATE TABLE consent_records (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    purpose     TEXT NOT NULL CHECK (purpose IN ('core_service', 'model_improvement', 'marketing')),
    granted     BOOLEAN NOT NULL,
    granted_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at  TIMESTAMPTZ,
    ip_hash     CHAR(64)
);

CREATE INDEX idx_consent_user ON consent_records(user_id);

-- =============================================================================
-- Query History
-- =============================================================================

CREATE TABLE query_history (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(id),
    tenant_id        UUID NOT NULL REFERENCES tenants(id),
    query_text_hash  CHAR(64) NOT NULL,
    response_hash    CHAR(64) NOT NULL,
    confidence_score NUMERIC(5,4),
    abstained        BOOLEAN NOT NULL DEFAULT FALSE,
    collections      TEXT[] NOT NULL DEFAULT '{}',
    provider         TEXT NOT NULL,
    tokens_in        INT NOT NULL DEFAULT 0,
    tokens_out       INT NOT NULL DEFAULT 0,
    latency_ms       INT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_query_history_tenant ON query_history(tenant_id);
CREATE INDEX idx_query_history_user ON query_history(user_id);

-- =============================================================================
-- Saved Research
-- =============================================================================

CREATE TABLE saved_research (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id),
    tenant_id   UUID NOT NULL REFERENCES tenants(id),
    title       TEXT NOT NULL,
    query       TEXT NOT NULL,
    response    TEXT NOT NULL,
    citations   JSONB NOT NULL DEFAULT '[]',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_saved_research_tenant ON saved_research(tenant_id);

-- =============================================================================
-- Ingestion Tracking
-- =============================================================================

CREATE TABLE ingest_jobs (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source      TEXT NOT NULL,
    collection  TEXT NOT NULL,
    status      TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'running', 'completed', 'failed')),
    chunk_count INT,
    error       TEXT,
    started_at  TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE ingest_docs (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id       UUID REFERENCES ingest_jobs(id),
    doc_id       TEXT NOT NULL,
    content_hash CHAR(64) NOT NULL,
    chunk_count  INT NOT NULL DEFAULT 0,
    status       TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'indexed', 'failed', 'superseded')),
    indexed_at   TIMESTAMPTZ,
    superseded_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX idx_ingest_docs_hash ON ingest_docs(content_hash);
CREATE INDEX idx_ingest_docs_job ON ingest_docs(job_id);

-- =============================================================================
-- IPC → BNS Mappings
-- =============================================================================

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

-- =============================================================================
-- Audit Log (append-only)
-- =============================================================================

CREATE TABLE audit_log (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID,
    tenant_id   UUID,
    action      TEXT NOT NULL,
    resource    TEXT NOT NULL,
    detail      JSONB,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_audit_log_tenant ON audit_log(tenant_id);
CREATE INDEX idx_audit_log_created ON audit_log(created_at);

-- Append-only: prevent UPDATE and DELETE on audit_log
CREATE OR REPLACE FUNCTION prevent_audit_mutation() RETURNS TRIGGER AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only: UPDATE and DELETE are not permitted';
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER audit_log_no_update
    BEFORE UPDATE OR DELETE ON audit_log
    FOR EACH ROW EXECUTE FUNCTION prevent_audit_mutation();

-- =============================================================================
-- Row-Level Security
-- =============================================================================

ALTER TABLE query_history ENABLE ROW LEVEL SECURITY;
ALTER TABLE saved_research ENABLE ROW LEVEL SECURITY;
ALTER TABLE consent_records ENABLE ROW LEVEL SECURITY;
ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_query_history ON query_history
    USING (tenant_id = current_setting('app.current_tenant_id')::uuid);

CREATE POLICY tenant_isolation_saved_research ON saved_research
    USING (tenant_id = current_setting('app.current_tenant_id')::uuid);

CREATE POLICY tenant_isolation_consent ON consent_records
    USING (tenant_id = current_setting('app.current_tenant_id')::uuid);

CREATE POLICY tenant_isolation_sessions ON sessions
    USING (tenant_id = current_setting('app.current_tenant_id')::uuid);
