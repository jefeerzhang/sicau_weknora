-- sicau-v1 ticket 04: workspace default agent id
ALTER TABLE tenants ADD COLUMN default_agent_id TEXT NOT NULL DEFAULT '';
