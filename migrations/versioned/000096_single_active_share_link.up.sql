-- Migration: 000096_single_active_share_link
-- Keep one reusable pending share link per workspace. Existing databases may
-- contain several links because migration 000054 intentionally allowed that;
-- retain the newest and revoke the older rows before adding the constraint.

DO $$ BEGIN RAISE NOTICE '[Migration 000096] Enforcing one active share link per tenant'; END $$;

UPDATE tenant_invitations AS invitation
SET status = 'revoked',
    responded_at = NOW(),
    updated_at = NOW()
WHERE invitation.status = 'pending'
  AND invitation.deleted_at IS NULL
  AND invitation.invitee_user_id = ''
  AND invitation.token <> ''
  AND invitation.id NOT IN (
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

DO $$ BEGIN RAISE NOTICE '[Migration 000096] Single active share link invariant ready'; END $$;
