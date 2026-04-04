-- 002_rls.sql
-- Row-level security for multi-tenant isolation.
-- Idempotent: uses DO $$ blocks to guard policy creation.

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE query_history ENABLE ROW LEVEL SECURITY;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE tablename = 'users' AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON users
            USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
    END IF;
END;
$$;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_policies
        WHERE tablename = 'query_history' AND policyname = 'tenant_isolation'
    ) THEN
        CREATE POLICY tenant_isolation ON query_history
            USING (tenant_id = current_setting('app.tenant_id', true)::uuid);
    END IF;
END;
$$;

CREATE INDEX IF NOT EXISTS idx_users_tenant_email ON users(tenant_id, email);
CREATE INDEX IF NOT EXISTS idx_qh_tenant_time     ON query_history(tenant_id, created_at DESC);
