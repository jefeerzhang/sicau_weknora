-- sicau-v1 notes — per-user Markdown notes and note images (sqlite mirror of 000097).

CREATE TABLE IF NOT EXISTS tenant_notes (
    id         TEXT PRIMARY KEY,
    tenant_id  INTEGER NOT NULL,
    user_id    TEXT NOT NULL,
    content    TEXT NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tenant_notes_user
    ON tenant_notes (tenant_id, user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS tenant_note_images (
    id         TEXT PRIMARY KEY,
    tenant_id  INTEGER NOT NULL,
    user_id    TEXT NOT NULL,
    mime       TEXT NOT NULL,
    bytes      BLOB NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_tenant_note_images_user
    ON tenant_note_images (tenant_id, user_id);
