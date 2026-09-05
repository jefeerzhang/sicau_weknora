-- Migration: 000094_platform_identity_flags (down)

DROP INDEX IF EXISTS idx_users_is_teacher;

ALTER TABLE users DROP COLUMN IF EXISTS is_teacher;
ALTER TABLE users DROP COLUMN IF EXISTS must_change_password;
