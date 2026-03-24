-- +goose Up
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

ALTER TABLE query_history ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_query_history ON query_history
    USING (tenant_id = current_setting('app.current_tenant_id')::uuid);

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

ALTER TABLE saved_research ENABLE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation_saved_research ON saved_research
    USING (tenant_id = current_setting('app.current_tenant_id')::uuid);

-- +goose Down
DROP POLICY IF EXISTS tenant_isolation_saved_research ON saved_research;
DROP POLICY IF EXISTS tenant_isolation_query_history ON query_history;
DROP TABLE IF EXISTS saved_research;
DROP TABLE IF EXISTS query_history;
