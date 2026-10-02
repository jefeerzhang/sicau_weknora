-- Description: student personal models (ADR-0001) — per-user chat model
-- rows plus workspace switch column on tenants.
DO $$ BEGIN RAISE NOTICE '[Migration 000118] Creating tenant_personal_models + tenants.student_personal_models'; END $$;

ALTER TABLE tenants
    ADD COLUMN IF NOT EXISTS student_personal_models JSONB;

CREATE TABLE IF NOT EXISTS tenant_personal_models (
    id          VARCHAR(36)  PRIMARY KEY,
    tenant_id   BIGINT       NOT NULL,
    user_id     VARCHAR(512) NOT NULL,
    name        VARCHAR(255) NOT NULL DEFAULT '',
    model_name  VARCHAR(255) NOT NULL,
    base_url    VARCHAR(1024) NOT NULL,
    provider    VARCHAR(64)  NOT NULL DEFAULT '',
    api_key_enc TEXT         NOT NULL DEFAULT '',
    enabled     BOOLEAN      NOT NULL DEFAULT TRUE,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at  TIMESTAMPTZ
);

COMMENT ON TABLE tenant_personal_models IS
    'Student-owned chat model entries (ADR-0001). Queries filter on (tenant_id, user_id); credentials encrypted at rest.';

CREATE INDEX IF NOT EXISTS idx_tenant_personal_models_user
    ON tenant_personal_models (tenant_id, user_id) WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_tenant_personal_models_deleted
    ON tenant_personal_models (deleted_at);
