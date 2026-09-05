DO $$ BEGIN RAISE NOTICE '[Migration 000091 down] Dropping tenants.default_agent_id'; END $$;

ALTER TABLE tenants DROP COLUMN IF EXISTS default_agent_id;
