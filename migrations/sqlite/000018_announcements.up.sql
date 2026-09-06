-- sicau-v1 announcements (sqlite mirror of 000098). Attachments metadata is TEXT JSON.

CREATE TABLE IF NOT EXISTS announcements (
    id          TEXT PRIMARY KEY,
    tenant_id   INTEGER NOT NULL,
    user_id     TEXT NOT NULL,
    title       TEXT NOT NULL,
    content     TEXT NOT NULL DEFAULT '',
    attachments TEXT NOT NULL DEFAULT '[]',
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_announcements_tenant
    ON announcements (tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS announcement_comments (
    id              TEXT PRIMARY KEY,
    tenant_id       INTEGER NOT NULL,
    announcement_id TEXT NOT NULL,
    user_id         TEXT NOT NULL,
    content         TEXT NOT NULL,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_announcement_comments
    ON announcement_comments (announcement_id, created_at);
