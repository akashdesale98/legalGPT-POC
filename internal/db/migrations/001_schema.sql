-- 001_schema.sql
-- Idempotent: all statements use IF NOT EXISTS.

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

CREATE TABLE IF NOT EXISTS tenants (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    name       TEXT        NOT NULL,
    slug       TEXT        NOT NULL UNIQUE,
    plan       TEXT        NOT NULL DEFAULT 'free',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS users (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id    UUID        NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    email        TEXT        NOT NULL,
    role         TEXT        NOT NULL DEFAULT 'free',
    hashed_token TEXT        NOT NULL,
    last_seen    TIMESTAMPTZ,
    created_at   TIMESTAMPTZ DEFAULT NOW(),
    updated_at   TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, email)
);

CREATE TABLE IF NOT EXISTS query_history (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id         UUID        NOT NULL REFERENCES tenants(id),
    user_id           UUID        REFERENCES users(id),
    query_hash        TEXT        NOT NULL,
    response_text     TEXT,
    provider_used     TEXT,
    tokens_in         INT,
    tokens_out        INT,
    latency_ms        INT,
    faithfulness_score FLOAT,
    created_at        TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS ipc_bns_map (
    ipc_section    TEXT PRIMARY KEY,
    bns_section    TEXT NOT NULL,
    description    TEXT,
    effective_date DATE
);
