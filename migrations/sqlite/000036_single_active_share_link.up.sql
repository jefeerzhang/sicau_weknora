-- SQLite mirror of versioned migration 000096_single_active_share_link.

UPDATE tenant_invitations
SET status = 'revoked',
    responded_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE status = 'pending'
  AND deleted_at IS NULL
  AND invitee_user_id = ''
  AND token <> ''
  AND id NOT IN (
      SELECT MAX(current_link.id)
      FROM tenant_invitations AS current_link
      WHERE current_link.status = 'pending'
        AND current_link.deleted_at IS NULL
        AND current_link.invitee_user_id = ''
        AND current_link.token <> ''
      GROUP BY current_link.tenant_id
  );

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_invitations_unique_pending_share_link
    ON tenant_invitations(tenant_id)
    WHERE status = 'pending'
      AND deleted_at IS NULL
      AND invitee_user_id = ''
      AND token <> '';
