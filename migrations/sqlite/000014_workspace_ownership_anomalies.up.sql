-- SQLite: workspace ownership anomalies for teaching migration (#18/#19)

CREATE TABLE IF NOT EXISTS workspace_ownership_anomalies (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    tenant_id INTEGER NOT NULL,
    kind TEXT NOT NULL,
    owner_count INTEGER NOT NULL DEFAULT 0,
    owner_user_ids TEXT NOT NULL DEFAULT '[]',
    status TEXT NOT NULL DEFAULT 'open',
    resolved_by TEXT,
    resolved_at DATETIME,
    resolution_note TEXT,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_workspace_ownership_anomalies_tenant
    ON workspace_ownership_anomalies (tenant_id);

CREATE INDEX IF NOT EXISTS idx_workspace_ownership_anomalies_status
    ON workspace_ownership_anomalies (status);
