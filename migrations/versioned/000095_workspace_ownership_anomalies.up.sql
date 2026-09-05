-- Migration: 000095_workspace_ownership_anomalies
-- Teaching #18: persist zero-owner / multi-owner workspaces for SuperAdmin
-- recovery (#19). Role demotion itself is a Go startup migrator.

DO $$ BEGIN RAISE NOTICE '[Migration 000095] Creating workspace_ownership_anomalies...'; END $$;

CREATE TABLE IF NOT EXISTS workspace_ownership_anomalies (
    id              BIGSERIAL PRIMARY KEY,
    tenant_id       BIGINT NOT NULL,
    kind            VARCHAR(32) NOT NULL,
    owner_count     INTEGER NOT NULL DEFAULT 0,
    owner_user_ids  JSONB NOT NULL DEFAULT '[]'::jsonb,
    status          VARCHAR(16) NOT NULL DEFAULT 'open',
    resolved_by     VARCHAR(36),
    resolved_at     TIMESTAMPTZ,
    resolution_note TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_workspace_ownership_anomalies_tenant UNIQUE (tenant_id)
);

CREATE INDEX IF NOT EXISTS idx_workspace_ownership_anomalies_status
    ON workspace_ownership_anomalies (status);

COMMENT ON TABLE workspace_ownership_anomalies IS
    'Teaching ownership anomalies (zero/multi owner); open rows await SuperAdmin resolve (#18/#19)';
