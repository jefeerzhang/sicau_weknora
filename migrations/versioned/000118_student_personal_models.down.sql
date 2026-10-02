DO $$ BEGIN RAISE NOTICE '[Migration 000118 down] Dropping tenant_personal_models + tenants.student_personal_models'; END $$;

DROP TABLE IF EXISTS tenant_personal_models;
ALTER TABLE tenants DROP COLUMN IF EXISTS student_personal_models;
