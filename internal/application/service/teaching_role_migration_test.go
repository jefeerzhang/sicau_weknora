package service

import (
	"errors"
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
	// failLogTx makes the next N LogTx calls fail, simulating an audit
	// persistence outage in the middle of a migration run.
	failLogTx int
}

func (a *recordingAudit) Log(_ context.Context, entry *types.AuditLog) error {
	cp := *entry
	a.entries = append(a.entries, &cp)
	return nil
}

// LogTx records the entry as if committed with the caller's transaction;
// the fake has no real transaction semantics. When failLogTx is armed the
// write fails so the caller's transaction must roll back.
func (a *recordingAudit) LogTx(_ context.Context, _ *gorm.DB, entry *types.AuditLog) error {
	if a.failLogTx > 0 {
		a.failLogTx--
		return errors.New("audit table unavailable")
	}
	return a.Log(context.Background(), entry)
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
		&types.TenantInvitation{},
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

// --- #23: ownership recovery audited atomically with the member changes ---

func TestTeachingRoleResolve_AuditFailureRollsBackEverything(t *testing.T) {
	db := newTeachingMigrationDB(t)
	audit := &recordingAudit{failLogTx: 1}
	m := NewTeachingRoleMigrator(db, audit)
	seedTenant(t, db, 10)
	seedUser(t, db, "u-teacher", true, false)
	seedUser(t, db, "u-other", false, false)
	seedMember(t, db, "u-teacher", 10, types.TenantRoleOwner)
	seedMember(t, db, "u-other", 10, types.TenantRoleOwner)
	_, _ = m.Run(context.Background())

	if err := m.ResolveAnomaly(context.Background(), 10, "u-teacher", "actor-sa"); err == nil {
		t.Fatalf("resolve must fail when the success audit write fails")
	}
	// Everything rolled back: memberships unchanged, anomaly stays open,
	// no success audit persisted for a recovery that did not happen.
	if roleOf(t, db, "u-other", 10) != types.TenantRoleOwner {
		t.Fatalf("failed audit must roll back the demotion, got %s", roleOf(t, db, "u-other", 10))
	}
	if roleOf(t, db, "u-teacher", 10) != types.TenantRoleOwner {
		t.Fatalf("promotion must roll back with the audit, got %s", roleOf(t, db, "u-teacher", 10))
	}
	open, err := m.ListOpenAnomalies(context.Background())
	if err != nil {
		t.Fatalf("ListOpenAnomalies: %v", err)
	}
	if len(open) != 1 {
		t.Fatalf("anomaly must stay open after failure, got %d", len(open))
	}
	if len(audit.entries) != 0 {
		t.Fatalf("no success audit may survive a rolled-back recovery, got %d", len(audit.entries))
	}

	// Retry after the outage heals completes the recovery exactly once.
	audit.failLogTx = 0
	if err := m.ResolveAnomaly(context.Background(), 10, "u-teacher", "actor-sa"); err != nil {
		t.Fatalf("retry resolve: %v", err)
	}
	if roleOf(t, db, "u-other", 10) != types.TenantRoleViewer {
		t.Fatalf("retry must demote the other owner, got %s", roleOf(t, db, "u-other", 10))
	}
	if roleOf(t, db, "u-teacher", 10) != types.TenantRoleOwner {
		t.Fatalf("retry must promote the teacher, got %s", roleOf(t, db, "u-teacher", 10))
	}
	if len(audit.entries) != 1 {
		t.Fatalf("retry must leave exactly 1 recovery audit, got %d", len(audit.entries))
	}
	e := audit.entries[0]
	if e.ActorUserID != "actor-sa" || e.TenantID != 10 || e.TargetUserID != "u-other" {
		t.Fatalf("audit must carry actor, workspace and target: %+v", e)
	}
	var details map[string]string
	if err := json.Unmarshal(e.Details, &details); err != nil {
		t.Fatalf("unmarshal details: %v", err)
	}
	if details["old_role"] != string(types.TenantRoleOwner) ||
		details["new_role"] != string(types.TenantRoleViewer) ||
		details["source"] != "superadmin" {
		t.Fatalf("audit must carry old/new roles and source, got %v", details)
	}
}

func TestTeachingRoleResolve_RerunDoesNotDuplicateAudits(t *testing.T) {
	db := newTeachingMigrationDB(t)
	audit := &recordingAudit{}
	m := NewTeachingRoleMigrator(db, audit)
	seedTenant(t, db, 11)
	seedUser(t, db, "u-teacher", true, false)
	seedUser(t, db, "u-other", false, false)
	seedMember(t, db, "u-teacher", 11, types.TenantRoleOwner)
	seedMember(t, db, "u-other", 11, types.TenantRoleOwner)
	_, _ = m.Run(context.Background())

	if err := m.ResolveAnomaly(context.Background(), 11, "u-teacher", "actor-sa"); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	first := len(audit.entries)

	// Repeat execution of an already-recovered workspace must be rejected
	// without new member changes and without new audits.
	if err := m.ResolveAnomaly(context.Background(), 11, "u-teacher", "actor-sa"); !errors.Is(err, ErrTeachingResolveNoOpenAnomaly) {
		t.Fatalf("rerun must report no open anomaly, got %v", err)
	}
	if len(audit.entries) != first {
		t.Fatalf("rerun must not duplicate audits, before=%d after=%d", first, len(audit.entries))
	}
	if roleOf(t, db, "u-other", 11) != types.TenantRoleViewer {
		t.Fatalf("rerun must not change memberships")
	}
}

// --- #21: pending-invitation normalization + atomic downgrade audits ---

func seedInvitation(
	t *testing.T,
	db *gorm.DB,
	tenantID uint64,
	invitee, token string,
	role types.TenantRole,
	status types.TenantInvitationStatus,
) uint64 {
	t.Helper()
	inv := &types.TenantInvitation{
		TenantID:      tenantID,
		InviteeUserID: invitee,
		Token:         token,
		Role:          role,
		Status:        status,
		ExpiresAt:     time.Now().Add(time.Hour),
	}
	if err := db.Create(inv).Error; err != nil {
		t.Fatalf("seed invitation: %v", err)
	}
	return inv.ID
}

func readInvitationRole(t *testing.T, db *gorm.DB, id uint64) types.TenantRole {
	t.Helper()
	var inv types.TenantInvitation
	if err := db.First(&inv, id).Error; err != nil {
		t.Fatalf("read invitation %d: %v", id, err)
	}
	return inv.Role
}

func TestTeachingRoleMigration_NormalizesPendingElevatedInvitations(t *testing.T) {
	db := newTeachingMigrationDB(t)
	seedTenant(t, db, 7)

	pendingAdmin := seedInvitation(t, db, 7, "u-bob", "", types.TenantRoleAdmin, types.TenantInvitationStatusPending)
	pendingLink := seedInvitation(t, db, 7, "", "legacy-share-token", types.TenantRoleContributor, types.TenantInvitationStatusPending)
	pendingViewer := seedInvitation(t, db, 7, "u-carol", "", types.TenantRoleViewer, types.TenantInvitationStatusPending)
	acceptedAdmin := seedInvitation(t, db, 7, "u-dave", "", types.TenantRoleAdmin, types.TenantInvitationStatusAccepted)
	declinedContrib := seedInvitation(t, db, 7, "u-eve", "", types.TenantRoleContributor, types.TenantInvitationStatusDeclined)

	report, err := NewTeachingRoleMigrator(db, nil).Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.InvitationsDowngraded != 2 || report.Failed != 0 {
		t.Fatalf("want 2 normalized invitations, 0 failed; got %+v", report)
	}
	if role := readInvitationRole(t, db, pendingAdmin); role != types.TenantRoleViewer {
		t.Fatalf("pending admin email must be viewer, got %s", role)
	}
	if role := readInvitationRole(t, db, pendingLink); role != types.TenantRoleViewer {
		t.Fatalf("pending share link must be viewer, got %s", role)
	}
	if role := readInvitationRole(t, db, pendingViewer); role != types.TenantRoleViewer {
		t.Fatalf("pending viewer must stay viewer, got %s", role)
	}
	// Terminal history is never rewritten.
	if role := readInvitationRole(t, db, acceptedAdmin); role != types.TenantRoleAdmin {
		t.Fatalf("accepted history must keep its role, got %s", role)
	}
	if role := readInvitationRole(t, db, declinedContrib); role != types.TenantRoleContributor {
		t.Fatalf("declined history must keep its role, got %s", role)
	}

	// One audit per normalized row, carrying the downgrade facts.
	var audits []types.AuditLog
	if err := db.Where("target_type = ?", "tenant_invitation").Find(&audits).Error; err != nil {
		t.Fatalf("read audits: %v", err)
	}
	if len(audits) != 2 {
		t.Fatalf("want 2 invitation audits, got %d", len(audits))
	}
	for _, a := range audits {
		var details map[string]any
		if err := json.Unmarshal(a.Details, &details); err != nil {
			t.Fatalf("unmarshal details: %v", err)
		}
		if details["new_role"] != string(types.TenantRoleViewer) {
			t.Fatalf("audit must record viewer, got %v", details)
		}
		if _, hasOld := details["old_role"]; !hasOld {
			t.Fatalf("audit must record old_role, got %v", details)
		}
		if details["reason"] != teachingLegacyRoleMigrationReason {
			t.Fatalf("audit must record migration reason, got %v", details)
		}
		if _, hasToken := details["token"]; hasToken {
			t.Fatalf("audit details must not carry the invitation token: %v", details)
		}
	}

	// Rerun is idempotent: nothing left to normalize, no new audits.
	report2, err := NewTeachingRoleMigrator(db, nil).Run(context.Background())
	if err != nil {
		t.Fatalf("rerun: %v", err)
	}
	if report2.InvitationsDowngraded != 0 || report2.Failed != 0 {
		t.Fatalf("rerun must normalize nothing, got %+v", report2)
	}
	var count int64
	if err := db.Model(&types.AuditLog{}).Where("target_type = ?", "tenant_invitation").Count(&count).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if count != 2 {
		t.Fatalf("rerun must not duplicate audits, got %d", count)
	}
}

func TestTeachingRoleMigration_NormalizationAuditFailureRetries(t *testing.T) {
	db := newTeachingMigrationDB(t)
	seedTenant(t, db, 8)
	invID := seedInvitation(t, db, 8, "u-bob", "", types.TenantRoleAdmin, types.TenantInvitationStatusPending)

	audit := &recordingAudit{failLogTx: 1}
	report, err := NewTeachingRoleMigrator(db, audit).Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.Failed != 1 {
		t.Fatalf("audit failure must be reported, got %+v", report)
	}
	if role := readInvitationRole(t, db, invID); role != types.TenantRoleAdmin {
		t.Fatalf("role update must roll back with the audit, got %s", role)
	}
	if len(audit.entries) != 0 {
		t.Fatalf("failed audit must not persist, got %d entries", len(audit.entries))
	}

	// Retry after the outage heals downgrades the row exactly once.
	audit.failLogTx = 0
	report2, err := NewTeachingRoleMigrator(db, audit).Run(context.Background())
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if report2.InvitationsDowngraded != 1 || report2.Failed != 0 {
		t.Fatalf("retry must normalize the row, got %+v", report2)
	}
	if role := readInvitationRole(t, db, invID); role != types.TenantRoleViewer {
		t.Fatalf("retry must downgrade to viewer, got %s", role)
	}
	if len(audit.entries) != 1 {
		t.Fatalf("retry must leave exactly 1 audit, got %d", len(audit.entries))
	}
}

func TestTeachingRoleMigration_MembershipDowngradeAuditFailureRollsBack(t *testing.T) {
	db := newTeachingMigrationDB(t)
	seedTenant(t, db, 9)
	seedMember(t, db, "u-owner", 9, types.TenantRoleOwner)
	seedMember(t, db, "u-admin", 9, types.TenantRoleAdmin)

	audit := &recordingAudit{failLogTx: 1}
	report, err := NewTeachingRoleMigrator(db, audit).Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if report.Failed != 1 {
		t.Fatalf("audit failure must be reported, got %+v", report)
	}
	var admin types.TenantMember
	if err := db.Where("user_id = ? AND tenant_id = ?", "u-admin", 9).First(&admin).Error; err != nil {
		t.Fatalf("read member: %v", err)
	}
	if admin.Role != types.TenantRoleAdmin {
		t.Fatalf("role update must roll back with the audit, got %s", admin.Role)
	}
	if len(audit.entries) != 0 {
		t.Fatalf("failed audit must not persist, got %d entries", len(audit.entries))
	}

	audit.failLogTx = 0
	report2, err := NewTeachingRoleMigrator(db, audit).Run(context.Background())
	if err != nil {
		t.Fatalf("retry: %v", err)
	}
	if report2.Downgraded != 1 || report2.Failed != 0 {
		t.Fatalf("retry must downgrade the member, got %+v", report2)
	}
	if err := db.Where("user_id = ? AND tenant_id = ?", "u-admin", 9).First(&admin).Error; err != nil {
		t.Fatalf("read member: %v", err)
	}
	if admin.Role != types.TenantRoleViewer {
		t.Fatalf("retry must downgrade to viewer, got %s", admin.Role)
	}
	if len(audit.entries) != 1 {
		t.Fatalf("retry must leave exactly 1 audit, got %d", len(audit.entries))
	}
}
