-- SQLite: student personal models (ADR-0001)
ALTER TABLE tenants ADD COLUMN student_personal_models TEXT;

CREATE TABLE IF NOT EXISTS tenant_personal_models (
    id          TEXT PRIMARY KEY,
    tenant_id   INTEGER NOT NULL,
    user_id     TEXT NOT NULL,
    name        TEXT NOT NULL DEFAULT '',
    model_name  TEXT NOT NULL,
    base_url    TEXT NOT NULL,
    provider    TEXT NOT NULL DEFAULT '',
    api_key_enc TEXT NOT NULL DEFAULT '',
    enabled     INTEGER NOT NULL DEFAULT 1,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at  DATETIME
);

CREATE INDEX IF NOT EXISTS idx_tenant_personal_models_user
    ON tenant_personal_models (tenant_id, user_id);
