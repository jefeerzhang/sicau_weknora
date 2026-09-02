package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type recordingAudit struct {
	entries []*types.AuditLog
}

func (a *recordingAudit) Log(_ context.Context, entry *types.AuditLog) error {
	cp := *entry
	a.entries = append(a.entries, &cp)
	return nil
}

func (a *recordingAudit) LogDenied(
	context.Context, *gin.Context, uint64, string, string, types.TenantRole,
) error {
	return nil
}

func (a *recordingAudit) List(context.Context, uint64, *interfaces.AuditLogQuery) ([]*types.AuditLog, error) {
	return nil, nil
}

func (a *recordingAudit) Purge(context.Context, int) (int64, error) { return 0, nil }

var _ interfaces.AuditLogService = (*recordingAudit)(nil)
func newTeachingMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:teaching_mig_" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&types.Tenant{},
		&types.User{},
		&types.TenantMember{},
		&types.WorkspaceOwnershipAnomaly{},
		&types.AuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedTenant(t *testing.T, db *gorm.DB, id uint64) {
	t.Helper()
	if err := db.Create(&types.Tenant{ID: id, Name: "t", Description: "d", CreatedAt: time.Now(), UpdatedAt: time.Now()}).Error; err != nil {
		t.Fatalf("seed tenant: %v", err)
	}
}

func seedMember(t *testing.T, db *gorm.DB, userID string, tenantID uint64, role types.TenantRole) {
	t.Helper()
	if err := db.Create(&types.TenantMember{
		UserID:    userID,
		TenantID:  tenantID,
		Role:      role,
		Status:    types.TenantMemberStatusActive,
		JoinedAt:  time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed member: %v", err)
	}
}

func seedUser(t *testing.T, db *gorm.DB, id string, teacher, admin bool) {
	t.Helper()
	if err := db.Create(&types.User{
		ID:            id,
		Username:      id,
		Email:         id + "@example.com",
		PasswordHash:  "x",
		IsActive:      true,
		IsTeacher:     teacher,
		IsSystemAdmin: admin,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
}

func roleOf(t *testing.T, db *gorm.DB, userID string, tenantID uint64) types.TenantRole {
	t.Helper()
	var m types.TenantMember
	if err := db.Where("user_id = ? AND tenant_id = ?", userID, tenantID).First(&m).Error; err != nil {
		t.Fatalf("load member: %v", err)
	}
	return m.Role
}

func TestTeachingRoleMigration_SingleOwnerDowngradesAdminContributor(t *testing.T) {
	db := newTeachingMigrationDB(t)
	audit := &recordingAudit{}
	seedTenant(t, db, 1)
	seedMember(t, db, "u-owner", 1, types.TenantRoleOwner)
	seedMember(t, db, "u-admin", 1, types.TenantRoleAdmin)
	seedMember(t, db, "u-contrib", 1, types.TenantRoleContributor)
	seedMember(t, db, "u-viewer", 1, types.TenantRoleViewer)

	report, err := NewTeachingRoleMigrator(db, audit).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.Downgraded != 2 {
		t.Fatalf("downgraded=%d, want 2", report.Downgraded)
	}
	if len(report.AnomalyTenantIDs) != 0 {
		t.Fatalf("unexpected anomalies: %v", report.AnomalyTenantIDs)
	}
	if roleOf(t, db, "u-owner", 1) != types.TenantRoleOwner {
		t.Fatalf("owner must stay owner")
	}
	if roleOf(t, db, "u-admin", 1) != types.TenantRoleViewer {
		t.Fatalf("admin must become viewer")
	}
	if roleOf(t, db, "u-contrib", 1) != types.TenantRoleViewer {
		t.Fatalf("contributor must become viewer")
	}
	if roleOf(t, db, "u-viewer", 1) != types.TenantRoleViewer {
		t.Fatalf("viewer must stay viewer")
	}
	if len(audit.entries) != 2 {
		t.Fatalf("want 2 audit entries, got %d", len(audit.entries))
	}
	for _, e := range audit.entries {
		if e.Action != types.AuditActionMemberRoleChanged {
			t.Fatalf("action=%s", e.Action)
		}
		var details map[string]string
		_ = json.Unmarshal(e.Details, &details)
		if details["reason"] != teachingLegacyRoleMigrationReason {
			t.Fatalf("details=%v", details)
		}
		if details["new_role"] != string(types.TenantRoleViewer) {
			t.Fatalf("new_role=%s", details["new_role"])
		}
	}
}

func TestTeachingRoleMigration_IdempotentRerun(t *testing.T) {
	db := newTeachingMigrationDB(t)
	audit := &recordingAudit{}
	m := NewTeachingRoleMigrator(db, audit)
	seedTenant(t, db, 2)
	seedMember(t, db, "u-owner", 2, types.TenantRoleOwner)
	seedMember(t, db, "u-admin", 2, types.TenantRoleAdmin)

	if _, err := m.Run(context.Background()); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	firstAudits := len(audit.entries)
	report, err := m.Run(context.Background())
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if report.Downgraded != 0 {
		t.Fatalf("second run downgraded=%d, want 0", report.Downgraded)
	}
	if len(audit.entries) != firstAudits {
		t.Fatalf("rerun must not emit extra audits")
	}
}

func TestTeachingRoleMigration_ZeroOwnerRecordsAnomalyKeepsOwnersUntouched(t *testing.T) {
	db := newTeachingMigrationDB(t)
	seedTenant(t, db, 3)
	seedMember(t, db, "u-admin", 3, types.TenantRoleAdmin)
	seedMember(t, db, "u-contrib", 3, types.TenantRoleContributor)

	report, err := NewTeachingRoleMigrator(db, nil).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.AnomalyTenantIDs) != 1 || report.AnomalyTenantIDs[0] != 3 {
		t.Fatalf("anomalies=%v", report.AnomalyTenantIDs)
	}
	if report.Downgraded != 2 {
		t.Fatalf("elevated roles still demoted even without owner: got %d", report.Downgraded)
	}
	anomalies, err := NewTeachingRoleMigrator(db, nil).ListOpenAnomalies(context.Background())
	if err != nil {
		t.Fatalf("ListOpenAnomalies: %v", err)
	}
	if len(anomalies) != 1 || anomalies[0].Kind != types.WorkspaceOwnershipAnomalyZeroOwner {
		t.Fatalf("anomalies=%+v", anomalies)
	}
}

func TestTeachingRoleMigration_MultiOwnerRecordsAnomalyWithoutPicking(t *testing.T) {
	db := newTeachingMigrationDB(t)
	seedTenant(t, db, 4)
	seedMember(t, db, "u-o1", 4, types.TenantRoleOwner)
	seedMember(t, db, "u-o2", 4, types.TenantRoleOwner)
	seedMember(t, db, "u-admin", 4, types.TenantRoleAdmin)

	report, err := NewTeachingRoleMigrator(db, nil).Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(report.AnomalyTenantIDs) != 1 {
		t.Fatalf("want 1 anomaly, got %v", report.AnomalyTenantIDs)
	}
	if roleOf(t, db, "u-o1", 4) != types.TenantRoleOwner || roleOf(t, db, "u-o2", 4) != types.TenantRoleOwner {
		t.Fatalf("multi-owner must not auto-pick; both owners stay owner")
	}
	if roleOf(t, db, "u-admin", 4) != types.TenantRoleViewer {
		t.Fatalf("admin still demoted under multi-owner")
	}
}

func TestTeachingRoleResolve_RequiresTeacherCapability(t *testing.T) {
	db := newTeachingMigrationDB(t)
	m := NewTeachingRoleMigrator(db, nil)
	seedTenant(t, db, 5)
	seedUser(t, db, "u-student", false, false)
	seedMember(t, db, "u-student", 5, types.TenantRoleViewer)
	seedMember(t, db, "u-o1", 5, types.TenantRoleOwner)
	seedMember(t, db, "u-o2", 5, types.TenantRoleOwner)
	_, _ = m.Run(context.Background())

	err := m.ResolveAnomaly(context.Background(), 5, "u-student", "actor-sa")
	if err != ErrTeachingResolveRequiresTeacher {
		t.Fatalf("err=%v, want requires teacher", err)
	}
	if roleOf(t, db, "u-o1", 5) != types.TenantRoleOwner {
		t.Fatalf("failed resolve must leave memberships unchanged")
	}
}

func TestTeachingRoleResolve_SuccessMakesSoleOwner(t *testing.T) {
	db := newTeachingMigrationDB(t)
	audit := &recordingAudit{}
	m := NewTeachingRoleMigrator(db, audit)
	seedTenant(t, db, 6)
	seedUser(t, db, "u-teacher", true, false)
	seedUser(t, db, "u-other", false, false)
	seedMember(t, db, "u-teacher", 6, types.TenantRoleOwner)
	seedMember(t, db, "u-other", 6, types.TenantRoleOwner)
	seedMember(t, db, "u-admin", 6, types.TenantRoleAdmin)
	_, _ = m.Run(context.Background())

	if err := m.ResolveAnomaly(context.Background(), 6, "u-teacher", "actor-sa"); err != nil {
		t.Fatalf("ResolveAnomaly: %v", err)
	}
	if roleOf(t, db, "u-teacher", 6) != types.TenantRoleOwner {
		t.Fatalf("chosen teacher must be sole owner")
	}
	if roleOf(t, db, "u-other", 6) != types.TenantRoleViewer {
		t.Fatalf("other owner demoted to viewer")
	}
	if roleOf(t, db, "u-admin", 6) != types.TenantRoleViewer {
		t.Fatalf("admin demoted to viewer")
	}
	open, err := m.ListOpenAnomalies(context.Background())
	if err != nil {
		t.Fatalf("ListOpenAnomalies: %v", err)
	}
	if len(open) != 0 {
		t.Fatalf("resolved anomaly must leave open list, got %d", len(open))
	}
}
