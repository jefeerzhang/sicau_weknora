-- Migration: 000094_platform_identity_flags
-- Teaching permission architecture (#8/#9):
--   must_change_password — bootstrap SuperAdmin must rotate initial credential
--   is_teacher           — platform Teacher appointment (independent of tenant roles)

DO $$ BEGIN RAISE NOTICE '[Migration 000094] Adding users.must_change_password and users.is_teacher...'; END $$;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS must_change_password BOOLEAN NOT NULL DEFAULT FALSE;

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS is_teacher BOOLEAN NOT NULL DEFAULT FALSE;

CREATE INDEX IF NOT EXISTS idx_users_is_teacher ON users (is_teacher);

COMMENT ON COLUMN users.must_change_password IS 'When true, authenticated user may only change password until cleared';
COMMENT ON COLUMN users.is_teacher IS 'Platform teacher identity appointed by SuperAdmin; prerequisite for creating workspaces';
