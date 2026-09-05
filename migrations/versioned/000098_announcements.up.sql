-- Description: sicau-v1 announcements — teacher-posted course notices
-- (homework distribution) with student comments. Posting is
-- Contributor+; every authenticated member reads and comments. Comments
-- are flat (no threading) plain text.
--
-- Attachments live in the workspace's FileService (NOT in PG) — files up
-- to 50MB are the wrong shape for BYTEA. The announcement row keeps only
-- metadata [{name, path, size}] in a jsonb column; the files themselves
-- are deleted via FileService when the announcement is deleted.
DO $$ BEGIN RAISE NOTICE '[Migration 000098] Creating announcements / announcement_comments'; END $$;

CREATE TABLE IF NOT EXISTS announcements (
    id         VARCHAR(36)  PRIMARY KEY,
    tenant_id  BIGINT       NOT NULL,
    user_id    VARCHAR(512) NOT NULL,
    title      VARCHAR(200) NOT NULL,
    content    TEXT         NOT NULL DEFAULT '',
    attachments JSONB       NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE announcements IS
    'Course announcements posted by Contributor+ (homework distribution). Readable by every member; deletion by author or admin cascades attachments (via FileService) and comments.';

CREATE INDEX IF NOT EXISTS idx_announcements_tenant
    ON announcements (tenant_id, created_at DESC);

CREATE TABLE IF NOT EXISTS announcement_comments (
    id              VARCHAR(36)  PRIMARY KEY,
    tenant_id       BIGINT       NOT NULL,
    announcement_id VARCHAR(36)  NOT NULL,
    user_id         VARCHAR(512) NOT NULL,
    content         TEXT         NOT NULL,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE announcement_comments IS
    'Flat per-announcement comments (plain text, no threading). Deletion: comment author or workspace admin.';

CREATE INDEX IF NOT EXISTS idx_announcement_comments
    ON announcement_comments (announcement_id, created_at);
