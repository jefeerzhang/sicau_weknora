DO $$ BEGIN RAISE NOTICE '[Migration 000112 down] Dropping tenant_note_images / tenant_notes'; END $$;

DROP TABLE IF EXISTS tenant_note_images;
DROP TABLE IF EXISTS tenant_notes;
