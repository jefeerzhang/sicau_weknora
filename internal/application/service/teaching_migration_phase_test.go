package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/database"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Unit coverage for the shared teaching migration phase (#24/#25): schema
// verification fails closed, the success judgement turns unresolved items
// into failures, and RunAndRecord keeps the blocked-state cache consistent
// for both the startup path and the SuperAdmin manual retry.

func newPhaseFixtureDB(t *testing.T, dsn string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
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

func TestJudgeTeachingMigrationRun(t *testing.T) {
	ok := &types.TeachingRoleMigrationReport{Failed: 0}

	if report, err := judgeTeachingMigrationRun(ok, nil); err != nil || report != ok {
		t.Fatalf("clean run must pass through, got report=%v err=%v", report, err)
	}

	runErr := errors.New("db gone")
	if _, err := judgeTeachingMigrationRun(nil, runErr); !errors.Is(err, runErr) {
		t.Fatalf("run error must propagate, got %v", err)
	}

	partial := &types.TeachingRoleMigrationReport{Failed: 2, Downgraded: 3}
	report, err := judgeTeachingMigrationRun(partial, nil)
	if err == nil {
		t.Fatalf("unresolved items must fail the phase")
	}
	if report != partial {
		t.Fatalf("the partial report must still be returned for operators, got %v", report)
	}
	if !strings.Contains(err.Error(), "2 item(s) unresolved") {
		t.Fatalf("error must name the unresolved count, got %v", err)
	}
}

func TestTeachingMigrationPhase_VerifySchema(t *testing.T) {
	db := newPhaseFixtureDB(t, "file:phase_verify_"+t.Name()+"?mode=memory&cache=shared")
	phase := NewTeachingMigrationPhase(db, nil)
	if err := phase.VerifySchema(); err != nil {
		t.Fatalf("complete schema must verify, got %v", err)
	}

	// Drop the anomaly unique index (the upsert dependency): verification
	// must fail closed with an actionable message.
	if err := db.Migrator().DropIndex(&types.WorkspaceOwnershipAnomaly{}, "idx_workspace_ownership_anomalies_tenant_id"); err != nil {
		t.Fatalf("drop index: %v", err)
	}
	err := phase.VerifySchema()
	if err == nil {
		t.Fatalf("missing anomaly unique index must fail verification")
	}
	if !strings.Contains(err.Error(), "unique index") {
		t.Fatalf("error must name the missing index, got %v", err)
	}
}

func TestTeachingMigrationPhase_RunAndRecord(t *testing.T) {
	ctx := context.Background()
	t.Cleanup(func() { database.CacheTeachingMigrationError("") })

	// Empty dataset + missing schema objects: no pending data must NOT be
	// treated as success (#25).
	bare, err := gorm.Open(sqlite.Open("file:phase_bare_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	database.CacheTeachingMigrationError("")
	phase := NewTeachingMigrationPhase(bare, nil)
	if _, err := phase.RunAndRecord(ctx); err == nil {
		t.Fatalf("missing schema must fail the retry even with no pending data")
	}
	if database.CachedTeachingMigrationError() == "" {
		t.Fatalf("blocked state must be preserved, not cleared")
	}
	if strings.Contains(database.CachedTeachingMigrationError(), "token") {
		t.Fatalf("blocked state must not leak invitation tokens: %q", database.CachedTeachingMigrationError())
	}

	// Schema repair + legacy rows: the retry completes normalization,
	// records the real audits and clears the blocked state.
	db := newPhaseFixtureDB(t, "file:phase_run_"+t.Name()+"?mode=memory&cache=shared")
	seedTenant(t, db, 1)
	seedMember(t, db, "u-lead", 1, types.TenantRoleOwner)
	seedMember(t, db, "u-admin", 1, types.TenantRoleAdmin)
	seedInvitation(t, db, 1, "u-new", "", types.TenantRoleContributor, types.TenantInvitationStatusPending)

	phase = NewTeachingMigrationPhase(db, nil)
	report, err := phase.RunAndRecord(ctx)
	if err != nil {
		t.Fatalf("repaired retry must succeed: %v", err)
	}
	if report.Downgraded != 1 || report.InvitationsDowngraded != 1 {
		t.Fatalf("retry must normalize both rows, got %+v", report)
	}
	if database.CachedTeachingMigrationError() != "" {
		t.Fatalf("success must clear the blocked state, got %q", database.CachedTeachingMigrationError())
	}
	var member types.TenantMember
	if err := db.Where("user_id = ? AND tenant_id = ?", "u-admin", 1).First(&member).Error; err != nil {
		t.Fatalf("read member: %v", err)
	}
	if member.Role != types.TenantRoleViewer {
		t.Fatalf("legacy admin must be demoted, got %s", member.Role)
	}

	// A second successful run is idempotent: no new changes, no new audits.
	report2, err := phase.RunAndRecord(ctx)
	if err != nil {
		t.Fatalf("rerun must succeed: %v", err)
	}
	if report2.Downgraded != 0 || report2.InvitationsDowngraded != 0 {
		t.Fatalf("rerun must change nothing, got %+v", report2)
	}
	var audits int64
	if err := db.Model(&types.AuditLog{}).
		Where("action = ? AND target_type IN ?", types.AuditActionMemberRoleChanged, []string{"tenant_member", "tenant_invitation"}).
		Count(&audits).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if audits != 2 {
		t.Fatalf("successful retry must not duplicate audits, got %d", audits)
	}
}
