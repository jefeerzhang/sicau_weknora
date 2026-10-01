-- Revoked duplicate links remain revoked on rollback.

DROP INDEX IF EXISTS idx_tenant_invitations_unique_pending_share_link;
