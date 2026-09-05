-- Description: sicau-v1 ticket 04 — the workspace default agent. New
-- conversations in a course workspace auto-select this agent (and its
-- pre-bound knowledge bases) unless the member explicitly picked another
-- one. Empty string means no default; clearing it is a supported op, so
-- the column is a plain value updated via a dedicated query rather than
-- the struct-based Updates() path that skips zero values.
DO $$ BEGIN RAISE NOTICE '[Migration 000091] Adding tenants.default_agent_id'; END $$;

ALTER TABLE tenants ADD COLUMN IF NOT EXISTS default_agent_id VARCHAR(36) NOT NULL DEFAULT '';

COMMENT ON COLUMN tenants.default_agent_id IS
    'Agent auto-selected for new conversations in this workspace; empty means none. Set/cleared by workspace Admin+ via PUT /tenants/kv/default-agent-id.';
