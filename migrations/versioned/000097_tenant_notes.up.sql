-- Description: sicau-v1 notes — per-user Markdown notes and note images.
-- Notes are the student's only writable data surface in the course
-- deployment. Isolation is structural: every query filters on
-- (tenant_id, user_id) and no endpoint lists notes across users, so
-- teachers cannot see student notes even in principle (framework doc
-- ADR-012 / notes design N-1).
--
-- Images are stored as BYTEA in PG on purpose (v1 deviation from the
-- FileService abstraction — see notes design §7 for the rationale and
-- the re-evaluation trigger). Access is owner-checked; the row id is
-- merely the capability token in the URL.
DO $$ BEGIN RAISE NOTICE '[Migration 000097] Creating tenant_notes / tenant_note_images'; END $$;

CREATE TABLE IF NOT EXISTS tenant_notes (
    id         VARCHAR(36)  PRIMARY KEY,
    tenant_id  BIGINT       NOT NULL,
    user_id    VARCHAR(512) NOT NULL,
    content    TEXT         NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE tenant_notes IS
    'Per-user Markdown notes. Owner-scoped: every read/write filters on (tenant_id, user_id) taken from the auth context; there is deliberately no endpoint that lists notes across users.';

CREATE INDEX IF NOT EXISTS idx_tenant_notes_user
    ON tenant_notes (tenant_id, user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS tenant_note_images (
    id         VARCHAR(36)  PRIMARY KEY,
    tenant_id  BIGINT       NOT NULL,
    user_id    VARCHAR(512) NOT NULL,
    mime       VARCHAR(100) NOT NULL,
    bytes      BYTEA        NOT NULL,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

COMMENT ON TABLE tenant_note_images IS
    'Note images (paste/screenshot uploads), max 2MB each, 200 per user. Served only to the owning user; the UUID in the URL is the capability token but ownership is still enforced server-side.';

CREATE INDEX IF NOT EXISTS idx_tenant_note_images_user
    ON tenant_note_images (tenant_id, user_id);
