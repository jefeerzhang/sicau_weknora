DO $$ BEGIN RAISE NOTICE '[Migration 000113 down] Dropping announcement_comments / announcements'; END $$;

DROP TABLE IF EXISTS announcement_comments;
DROP TABLE IF EXISTS announcements;
