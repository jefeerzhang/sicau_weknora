package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func seedLegacyTeachingSQLite(t *testing.T, root, dbPath string, version int) *sql.DB {
	t.Helper()
	legacy := copySQLiteMigrationsThrough(t, root, 12)
	chdirAndRestore(t, legacy)
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: dbPath}))
	db := openSQLiteDB(t, dbPath)
	// Same teaching SQL occupied 13–18 before the v0.8.2 re-numbering.
	for offset, topic := range []string{
		"000034_platform_identity_flags", "000035_workspace_ownership_anomalies",
		"000036_single_active_share_link", "000031_tenant_default_agent",
		"000032_tenant_notes", "000033_announcements",
	} {
		if 13+offset > version {
			break
		}
		query, err := os.ReadFile(filepath.Join(root, "migrations", "sqlite", topic+".up.sql"))
		require.NoError(t, err)
		_, err = db.Exec(string(query))
		require.NoError(t, err)
	}
	_, err := db.Exec("UPDATE schema_migrations SET version = ?, dirty = false", version)
	require.NoError(t, err)
	return db
}

func TestSQLiteMigrationsUpgradeLegacyTeachingSlots(t *testing.T) {
	root := sqliteRepoRoot(t)
	for version := 13; version <= 18; version++ {
		t.Run(fmt.Sprintf("version%d", version), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "legacy teaching.db")
			db := seedLegacyTeachingSQLite(t, root, path, version)
			_, err := db.Exec("INSERT INTO tenants (name, business) VALUES ('course', 'teaching-sentinel')")
			require.NoError(t, err)
			if version >= 16 {
				_, err = db.Exec("UPDATE tenants SET default_agent_id = 'course-agent'")
				require.NoError(t, err)
			}
			if version >= 17 {
				_, err = db.Exec("INSERT INTO tenant_notes (id,tenant_id,user_id,content) VALUES ('note',1,'student','keep-note')")
				require.NoError(t, err)
			}
			if version >= 18 {
				_, err = db.Exec("INSERT INTO announcements (id,tenant_id,user_id,title,content) VALUES ('notice',1,'teacher','title','keep-notice')")
				require.NoError(t, err)
			}
			chdirAndRestore(t, root)
			for attempt := 0; attempt < 2; attempt++ {
				require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}))
				got, dirty := sqliteMigrationState(t, db)
				require.Equal(t, expectedSQLiteMigrationVersion, got)
				require.False(t, dirty)
				for _, table := range versionedSQLiteTables {
					require.True(t, sqliteTableExists(t, db, table), table)
				}
				for table, columns := range versionedSQLiteColumns {
					for _, column := range columns {
						require.True(t, sqliteColumnExists(t, db, table, column), table+"."+column)
					}
				}
			}
			var agent, content string
			if version >= 16 {
				require.NoError(t, db.QueryRow("SELECT default_agent_id FROM tenants WHERE business='teaching-sentinel'").Scan(&agent))
				require.Equal(t, "course-agent", agent)
			}
			if version >= 17 {
				require.NoError(t, db.QueryRow("SELECT content FROM tenant_notes WHERE id='note'").Scan(&content))
				require.Equal(t, "keep-note", content)
			}
			if version >= 18 {
				require.NoError(t, db.QueryRow("SELECT content FROM announcements WHERE id='notice'").Scan(&content))
				require.Equal(t, "keep-notice", content)
			}
		})
	}
}

func TestSQLiteTeachingBridgeFailureRollsBackAndRetries(t *testing.T) {
	root := sqliteRepoRoot(t)
	path := filepath.Join(t.TempDir(), "legacy.db")
	db := seedLegacyTeachingSQLite(t, root, path, 18)
	brokenRoot := copySQLiteMigrationsThrough(t, root, 36)
	brokenFile := filepath.Join(brokenRoot, "migrations", "sqlite", "000014_browser_authorization.up.sql")
	original, err := os.ReadFile(brokenFile)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(brokenFile, []byte("CREATE TABLE broken (;"), 0o600))
	chdirAndRestore(t, brokenRoot)
	require.ErrorContains(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path}), "teaching bridge 000014")
	require.False(t, sqliteColumnExists(t, db, "mcp_tool_approvals", "enabled"))
	version, dirty := sqliteMigrationState(t, db)
	require.Equal(t, 18, version)
	require.False(t, dirty)
	require.NotEmpty(t, CachedMigrationError())
	require.NoError(t, os.WriteFile(brokenFile, original, 0o600))
	require.NoError(t, RunMigrations("sqlite3://"+path))
	version, dirty = sqliteMigrationState(t, db)
	require.Equal(t, expectedSQLiteMigrationVersion, version)
	require.False(t, dirty)
	require.Empty(t, CachedMigrationError())
}

func TestSQLiteMigrationsRecoverLegacyTeachingDirty31(t *testing.T) {
	root := sqliteRepoRoot(t)
	path := filepath.Join(t.TempDir(), "failed-upgrade.db")
	db := seedLegacyTeachingSQLite(t, root, path, 18)
	// Model a deployment that already applied 19–30 and failed on the
	// duplicate teaching column at 31 before this fix was installed.
	for version := 19; version <= 30; version++ {
		paths, err := filepath.Glob(filepath.Join(root, "migrations", "sqlite", fmt.Sprintf("%06d_*.up.sql", version)))
		require.NoError(t, err)
		require.Len(t, paths, 1)
		contents, err := os.ReadFile(paths[0])
		require.NoError(t, err)
		_, err = db.Exec(string(contents))
		require.NoError(t, err)
	}
	_, err := db.Exec("UPDATE schema_migrations SET version=31, dirty=true")
	require.NoError(t, err)
	chdirAndRestore(t, root)
	require.NoError(t, RunMigrationsWithOptions("sqlite3://unused", MigrationOptions{SQLiteDBPath: path, AutoRecoverDirty: true}))
	version, dirty := sqliteMigrationState(t, db)
	require.Equal(t, expectedSQLiteMigrationVersion, version)
	require.False(t, dirty)
	require.True(t, sqliteColumnExists(t, db, "mcp_tool_approvals", "enabled"))
	require.True(t, sqliteColumnExists(t, db, "sessions", "parent_session_id"))
}
