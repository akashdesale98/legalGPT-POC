-- +goose Up
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

ALTER TABLE sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE consent_records ENABLE ROW LEVEL SECURITY;

CREATE POLICY tenant_isolation_sessions ON sessions
    USING (tenant_id = current_setting('app.current_tenant_id')::uuid);

CREATE POLICY tenant_isolation_consent ON consent_records
    USING (tenant_id = current_setting('app.current_tenant_id')::uuid);

-- +goose Down
DROP POLICY IF EXISTS tenant_isolation_consent ON consent_records;
DROP POLICY IF EXISTS tenant_isolation_sessions ON sessions;
DROP TABLE IF EXISTS consent_records;
DROP TABLE IF EXISTS sessions;
