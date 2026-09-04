package container

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/database"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Startup-phase coverage for the teaching data migration (#22): the phase
// must run in every deployment mode (AUTO_MIGRATE on/off, externally
// applied schema), verify its required tables before touching data, surface
// failure as a blocked state via the cached teaching-migration error, and
// retry cleanly on the next startup.

// runInitDatabase drives the real initDatabase startup path against a temp
// SQLite file. Returns the opened handle; the caller must close it.
func runInitDatabase(t *testing.T, autoMigrate string) *gorm.DB {
	t.Helper()
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_PATH", filepath.Join(t.TempDir(), "teaching_startup.db"))
	t.Setenv("AUTO_MIGRATE", autoMigrate)
	db, err := initDatabase(&config.Config{})
	if err != nil {
		t.Fatalf("initDatabase: %v", err)
	}
	return db
}

// seedLegacyTeachingRows creates the schema (as an external migration tool
// would) plus one legacy admin membership and one pending admin invitation.
func seedLegacyTeachingRows(t *testing.T, dbPath string) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open external schema: %v", err)
	}
	if err := db.AutoMigrate(
		&types.Tenant{}, &types.TenantMember{}, &types.TenantInvitation{},
		&types.WorkspaceOwnershipAnomaly{}, &types.AuditLog{},
	); err != nil {
		t.Fatalf("external schema: %v", err)
	}
	now := time.Now()
	if err := db.Create(&types.Tenant{ID: 1, Name: "course", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if err := db.Create(&types.TenantMember{
		UserID: "u-lead", TenantID: 1, Role: types.TenantRoleOwner,
		Status: types.TenantMemberStatusActive, JoinedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	if err := db.Create(&types.TenantMember{
		UserID: "u-admin", TenantID: 1, Role: types.TenantRoleAdmin,
		Status: types.TenantMemberStatusActive, JoinedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed legacy admin: %v", err)
	}
	if err := db.Create(&types.TenantInvitation{
		TenantID:      1,
		InviteeUserID: "u-new",
		Role:          types.TenantRoleContributor,
		Status:        types.TenantInvitationStatusPending,
		ExpiresAt:     now.Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed legacy invitation: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("underlying sql db: %v", err)
	}
	_ = sqlDB.Close()
}

func readStartupMemberRole(t *testing.T, db *gorm.DB, userID string) types.TenantRole {
	t.Helper()
	var m types.TenantMember
	if err := db.Where("user_id = ? AND tenant_id = ?", userID, 1).First(&m).Error; err != nil {
		t.Fatalf("read member %s: %v", userID, err)
	}
	return m.Role
}

func readStartupInvitationRole(t *testing.T, db *gorm.DB) types.TenantRole {
	t.Helper()
	var inv types.TenantInvitation
	if err := db.Where("tenant_id = ? AND invitee_user_id = ?", 1, "u-new").First(&inv).Error; err != nil {
		t.Fatalf("read invitation: %v", err)
	}
	return inv.Role
}

func TestEnsureTeachingMigrationState_ExternalSchemaRunsDespiteAutoMigrateFalse(t *testing.T) {
	// Simulate the external-schema deployment: AUTO_MIGRATE=false and the
	// schema applied by an external tool before startup. The teaching data
	// migration must still normalize legacy rows.
	dbPath := filepath.Join(t.TempDir(), "external_schema.db")
	seedLegacyTeachingRows(t, dbPath)
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_PATH", dbPath)
	t.Setenv("AUTO_MIGRATE", "false")

	db, err := initDatabase(&config.Config{})
	if err != nil {
		t.Fatalf("initDatabase: %v", err)
	}
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	if got := readStartupMemberRole(t, db, "u-admin"); got != types.TenantRoleViewer {
		t.Fatalf("external-schema startup must demote legacy admin, got %s", got)
	}
	if got := readStartupMemberRole(t, db, "u-lead"); got != types.TenantRoleOwner {
		t.Fatalf("owner must stay owner, got %s", got)
	}
	if got := readStartupInvitationRole(t, db); got != types.TenantRoleViewer {
		t.Fatalf("external-schema startup must normalize pending elevated invitation, got %s", got)
	}
	if msg := database.CachedTeachingMigrationError(); msg != "" {
		t.Fatalf("successful startup must clear the blocked state, got %q", msg)
	}
	assertNormalizationAudits(t, db)
}

// assertNormalizationAudits checks one member-role audit and one
// invitation-role audit carry the downgrade trail for the seeded rows.
func assertNormalizationAudits(t *testing.T, db *gorm.DB) {
	t.Helper()
	var memberAudits, invitationAudits int64
	if err := db.Model(&types.AuditLog{}).
		Where("action = ? AND target_type = ?", types.AuditActionMemberRoleChanged, "tenant_member").
		Count(&memberAudits).Error; err != nil {
		t.Fatalf("count member audits: %v", err)
	}
	if err := db.Model(&types.AuditLog{}).
		Where("action = ? AND target_type = ?", types.AuditActionMemberRoleChanged, "tenant_invitation").
		Count(&invitationAudits).Error; err != nil {
		t.Fatalf("count invitation audits: %v", err)
	}
	if memberAudits != 1 || invitationAudits != 1 {
		t.Fatalf("want one success audit per normalized row, got member=%d invitation=%d", memberAudits, invitationAudits)
	}
}

func TestEnsureTeachingMigrationState_MissingTablesBlockedThenRetry(t *testing.T) {
	// Fresh empty database with AUTO_MIGRATE=false: the "external tool has
	// not applied the schema yet" state. The phase must fail closed with a
	// blocked state instead of crashing or silently succeeding.
	dbPath := filepath.Join(t.TempDir(), "blocked.db")
	db := runInitDatabaseWithDBPath(t, dbPath, "false")
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	msg := database.CachedTeachingMigrationError()
	if msg == "" {
		t.Fatalf("missing tables must record a blocked state")
	}
	if strings.Contains(msg, "token") {
		t.Fatalf("blocked state must not leak invitation tokens: %q", msg)
	}

	// Next startup after the external schema landed: retry completes and
	// clears the blocked state. Seed legacy rows with the schema to prove
	// the retry acts on them.
	seedLegacyTeachingRows(t, dbPath)
	db2 := runInitDatabaseWithDBPath(t, dbPath, "false")
	defer func() {
		if sqlDB, err := db2.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	if got := readStartupMemberRole(t, db2, "u-admin"); got != types.TenantRoleViewer {
		t.Fatalf("retry startup must demote legacy admin, got %s", got)
	}
	if got := readStartupInvitationRole(t, db2); got != types.TenantRoleViewer {
		t.Fatalf("retry startup must normalize pending invitation, got %s", got)
	}
	if msg := database.CachedTeachingMigrationError(); msg != "" {
		t.Fatalf("retry must clear the blocked state, got %q", msg)
	}
}

// runInitDatabaseWithDBPath is runInitDatabase with an explicit pre-existing
// database file path.
func runInitDatabaseWithDBPath(t *testing.T, dbPath, autoMigrate string) *gorm.DB {
	t.Helper()
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_PATH", dbPath)
	t.Setenv("AUTO_MIGRATE", autoMigrate)
	db, err := initDatabase(&config.Config{})
	if err != nil {
		t.Fatalf("initDatabase: %v", err)
	}
	return db
}

func TestEnsureTeachingMigrationState_AuditFailureBlocksStartup(t *testing.T) {
	// audit_logs exists but lacks a column the audit insert writes, so the
	// per-item downgrade transaction fails and must roll back: the member
	// stays elevated and the blocked state is recorded (#21/#22).
	dbPath := filepath.Join(t.TempDir(), "audit_failure.db")
	db, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	// Real schema for the teaching tables (same shapes the migrations lay
	// down); only audit_logs is deliberately broken.
	if err := db.AutoMigrate(
		&types.Tenant{}, &types.TenantMember{}, &types.TenantInvitation{},
		&types.WorkspaceOwnershipAnomaly{},
	); err != nil {
		t.Fatalf("migrate teaching tables: %v", err)
	}
	// Deliberately missing the outcome column the audit write needs.
	if err := db.Exec(`CREATE TABLE audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT, tenant_id INTEGER NOT NULL DEFAULT 0,
		actor_user_id TEXT DEFAULT '', actor_role TEXT DEFAULT '', action TEXT NOT NULL,
		scope_type TEXT DEFAULT '', scope_id TEXT DEFAULT '', target_type TEXT DEFAULT '',
		target_id TEXT DEFAULT '', target_user_id TEXT DEFAULT '',
		request_path TEXT DEFAULT '', request_method TEXT DEFAULT '',
		details TEXT DEFAULT '{}', created_at DATETIME)`).Error; err != nil {
		t.Fatalf("create broken audit_logs: %v", err)
	}
	now := time.Now()
	if err := db.Create(&types.Tenant{ID: 1, Name: "course", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	if err := db.Create(&types.TenantMember{
		UserID: "u-lead", TenantID: 1, Role: types.TenantRoleOwner,
		Status: types.TenantMemberStatusActive, JoinedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	if err := db.Create(&types.TenantMember{
		UserID: "u-admin", TenantID: 1, Role: types.TenantRoleAdmin,
		Status: types.TenantMemberStatusActive, JoinedAt: now,
	}).Error; err != nil {
		t.Fatalf("seed legacy admin: %v", err)
	}
	if err := db.Create(&types.TenantInvitation{
		TenantID:      1,
		InviteeUserID: "u-new",
		Role:          types.TenantRoleContributor,
		Status:        types.TenantInvitationStatusPending,
		ExpiresAt:     now.Add(time.Hour),
	}).Error; err != nil {
		t.Fatalf("seed legacy invitation: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}

	db = runInitDatabaseWithDBPath(t, dbPath, "false")
	defer func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()

	if msg := database.CachedTeachingMigrationError(); msg == "" {
		t.Fatalf("mid-run audit failure must record a blocked state")
	}
	// The downgrade rolled back with its audit: permissions and audit
	// facts stay consistent even on the startup path.
	if got := readStartupMemberRole(t, db, "u-admin"); got != types.TenantRoleAdmin {
		t.Fatalf("failed downgrade must roll back to admin, got %s", got)
	}

	// Operator repairs the audit table; the next startup retry completes
	// the downgrade and clears the blocked state.
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	repair, err := gorm.Open(sqlite.Open(dbPath+"?_journal_mode=WAL"), &gorm.Config{})
	if err != nil {
		t.Fatalf("reopen for repair: %v", err)
	}
	if err := repair.Migrator().DropTable("audit_logs"); err != nil {
		t.Fatalf("drop broken audit_logs: %v", err)
	}
	if err := repair.AutoMigrate(&types.AuditLog{}); err != nil {
		t.Fatalf("repair audit_logs: %v", err)
	}
	if sqlDB, err := repair.DB(); err == nil {
		_ = sqlDB.Close()
	}

	db2 := runInitDatabaseWithDBPath(t, dbPath, "false")
	defer func() {
		if sqlDB, err := db2.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()
	if msg := database.CachedTeachingMigrationError(); msg != "" {
		t.Fatalf("retry after repair must clear the blocked state, got %q", msg)
	}
	if got := readStartupMemberRole(t, db2, "u-admin"); got != types.TenantRoleViewer {
		t.Fatalf("retry after repair must demote the legacy admin, got %s", got)
	}
	assertNormalizationAudits(t, db2)
}

func TestRunTeachingDataMigration_RunnerOutcomes(t *testing.T) {
	ctx := context.Background()
	db := newRunnerFixtureDB(t)

	// Runner reports failure → phase fails closed.
	if err := runTeachingDataMigration(ctx, db, &stubTeachingRunner{failed: 3}); err == nil {
		t.Fatalf("runner failure must propagate")
	}
	// Runner errors → phase fails closed.
	if err := runTeachingDataMigration(ctx, db, &stubTeachingRunner{runErr: true}); err == nil {
		t.Fatalf("runner error must propagate")
	}
	// Runner succeeds → phase succeeds (missing-table check passes on the
	// fully migrated fixture).
	if err := runTeachingDataMigration(ctx, db, &stubTeachingRunner{}); err != nil {
		t.Fatalf("runner success must pass: %v", err)
	}
	if msg := database.CachedTeachingMigrationError(); msg != "" {
		// runTeachingDataMigration itself does not cache; only
		// ensureTeachingMigrationState does. The global state from earlier
		// tests must not be confused with this success path.
		t.Logf("cached state after direct run (informational): %q", msg)
	}
}

// stubTeachingRunner injects migrator outcomes without a database.
type stubTeachingRunner struct {
	failed int
	runErr bool
}

func (s *stubTeachingRunner) Run(ctx context.Context) (*types.TeachingRoleMigrationReport, error) {
	if s.runErr {
		return nil, os.ErrDeadlineExceeded
	}
	return &types.TeachingRoleMigrationReport{
		Downgraded:         0,
		Skipped:            0,
		Failed:             s.failed,
		AnomalyTenantIDs:   []uint64{},
		InvitationsDowngraded: 0,
	}, nil
}

// newRunnerFixtureDB provides a schema-complete sqlite handle for
// verifyTeachingMigrationTables.
func newRunnerFixtureDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:runner_fixture_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&types.Tenant{}, &types.TenantMember{}, &types.TenantInvitation{},
		&types.WorkspaceOwnershipAnomaly{}, &types.AuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestVerifyTeachingMigrationTables_MissingTableFailsClosed(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:verify_tables_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&types.Tenant{}, &types.TenantMember{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	err = verifyTeachingMigrationTables(db)
	if err == nil {
		t.Fatalf("missing tables must fail closed")
	}
	if !strings.Contains(err.Error(), "tenant_invitations") {
		t.Fatalf("error must name the missing table, got %v", err)
	}
}

// TestInitDatabase_TeachingPhaseAfterAutoMigration drives the full
// AUTO_MIGRATE=true startup: the first boot runs the real versioned SQLite
// migrations from the repo, the second boot (with legacy rows seeded in
// between, the way an upgrade would find them) must still normalize them
// (#22 Testing Decisions: both AUTO_MIGRATE modes).
func TestInitDatabase_TeachingPhaseAfterAutoMigration(t *testing.T) {
	if testing.Short() {
		t.Skip("startup integration test")
	}
	// golang-migrate resolves "file://migrations/sqlite" relative to the
	// process working directory; point it at the repo root for the run.
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir("../../"); err != nil {
		t.Fatalf("chdir to repo root: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	dbPath := filepath.Join(t.TempDir(), "auto_migrate_true.db")
	t.Setenv("DB_DRIVER", "sqlite")
	t.Setenv("DB_PATH", dbPath)
	t.Setenv("AUTO_MIGRATE", "true")

	// First boot: schema migrations run for real; the teaching phase
	// completes on an empty dataset.
	database.CacheTeachingMigrationError("")
	db, err := initDatabase(&config.Config{})
	if err != nil {
		t.Fatalf("first initDatabase: %v", err)
	}
	if msg := database.CachedTeachingMigrationError(); msg != "" {
		t.Fatalf("first boot must leave the teaching phase healthy, got %q", msg)
	}

	// Seed legacy rows as they would exist before an upgrade. The tenant
	// row uses raw SQL because the versioned migrations lag behind the
	// gorm model on non-essential tenant columns; the migrator only reads
	// tenants.id.
	if err := db.Exec(`INSERT INTO tenants (id, name, business, retriever_engines, storage_quota, storage_used, created_at, updated_at) VALUES (1, 'course', '', '[]', 10737418240, 0, datetime('now'), datetime('now'))`).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
	now := time.Now()
	if err := db.Create(&types.TenantMember{UserID: "u-lead", TenantID: 1, Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive, JoinedAt: now}).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	if err := db.Create(&types.TenantMember{UserID: "u-admin", TenantID: 1, Role: types.TenantRoleAdmin, Status: types.TenantMemberStatusActive, JoinedAt: now}).Error; err != nil {
		t.Fatalf("seed legacy admin: %v", err)
	}
	if err := db.Create(&types.TenantInvitation{TenantID: 1, InviteeUserID: "u-new", Role: types.TenantRoleContributor, Status: types.TenantInvitationStatusPending, ExpiresAt: now.Add(time.Hour)}).Error; err != nil {
		t.Fatalf("seed legacy invitation: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}

	// Second boot: migrations are a no-op at the recorded version; the
	// teaching phase still normalizes the legacy rows.
	db2 := runInitDatabaseWithDBPath(t, dbPath, "true")
	defer func() {
		if sqlDB, err := db2.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}()
	if msg := database.CachedTeachingMigrationError(); msg != "" {
		t.Fatalf("second boot must keep the teaching phase healthy, got %q", msg)
	}
	if got := readStartupMemberRole(t, db2, "u-admin"); got != types.TenantRoleViewer {
		t.Fatalf("AUTO_MIGRATE=true boot must demote the legacy admin, got %s", got)
	}
	if got := readStartupInvitationRole(t, db2); got != types.TenantRoleViewer {
		t.Fatalf("AUTO_MIGRATE=true boot must normalize the pending invitation, got %s", got)
	}
	if got := readStartupMemberRole(t, db2, "u-lead"); got != types.TenantRoleOwner {
		t.Fatalf("owner must stay owner, got %s", got)
	}
}
