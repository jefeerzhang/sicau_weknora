-- Reverse of 000096_single_active_share_link. Revoked duplicate links are not
-- reactivated because doing so would unexpectedly restore access.

DROP INDEX IF EXISTS idx_tenant_invitations_unique_pending_share_link;
