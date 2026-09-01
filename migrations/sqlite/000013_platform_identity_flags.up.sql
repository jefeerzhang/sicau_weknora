-- SQLite: platform identity flags for SuperAdmin bootstrap + Teacher appointment

ALTER TABLE users ADD COLUMN must_change_password INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN is_teacher INTEGER NOT NULL DEFAULT 0;
