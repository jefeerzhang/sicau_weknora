package service

import (
	"context"
	"errors"
	"testing"
	"time"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Atomicity coverage for invitation acceptance (#21): the invitation flip,
// the membership insert and both audit rows must commit or roll back as
// one unit, and acceptance must materialise viewer even when the pending
// row still carries a legacy elevated role. These tests use the real
// repositories over in-memory SQLite because fakes cannot express
// transactional rollback.

func newInvitationAtomicDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := "file:inv_atomic_" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&types.TenantInvitation{},
		&types.TenantMember{},
		&types.AuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

type atomicFixture struct {
	db       *gorm.DB
	svc      interfaces.TenantInvitationService
	memberSvc interfaces.TenantMemberService
	audit    *flakyAudit
}

// flakyAudit wraps the real audit service but fails the transaction-scoped
// write for the configured action a fixed number of times, simulating an
// audit persistence outage. The wrapped service keeps recording nothing on
// failed calls because the surrounding transaction rolls back.
type flakyAudit struct {
	interfaces.AuditLogService
	failAction types.AuditAction
	failures   int
}

func (f *flakyAudit) LogTx(ctx context.Context, tx *gorm.DB, entry *types.AuditLog) error {
	if f.failures > 0 && entry != nil && entry.Action == f.failAction {
		f.failures--
		return errors.New("audit table unavailable")
	}
	return f.AuditLogService.LogTx(ctx, tx, entry)
}

func newAtomicFixture(t *testing.T) *atomicFixture {
	t.Helper()
	db := newInvitationAtomicDB(t)
	realAudit := NewAuditLogService(apprepo.NewAuditLogRepository(db))
	audit := &flakyAudit{AuditLogService: realAudit}
	memberSvc := NewTenantMemberService(apprepo.NewTenantMemberRepository(db), audit, nil, nil)
	invRepo := apprepo.NewTenantInvitationRepository(db)
	svc := NewTenantInvitationService(db, invRepo, memberSvc, audit)
	return &atomicFixture{db: db, svc: svc, memberSvc: memberSvc, audit: audit}
}

func (f *atomicFixture) seedEmailInvitation(t *testing.T, invitee string, role types.TenantRole) uint64 {
	t.Helper()
	inv := &types.TenantInvitation{
		TenantID:      1,
		InviteeUserID: invitee,
		Role:          role,
		Status:        types.TenantInvitationStatusPending,
		ExpiresAt:     time.Now().Add(time.Hour),
	}
	if err := f.db.Create(inv).Error; err != nil {
		t.Fatalf("seed email invitation: %v", err)
	}
	return inv.ID
}

func (f *atomicFixture) seedShareLink(t *testing.T, role types.TenantRole) (uint64, string) {
	t.Helper()
	token := "legacy-link-" + string(role)
	inv := &types.TenantInvitation{
		TenantID:  1,
		Token:     token,
		Role:      role,
		Status:    types.TenantInvitationStatusPending,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	if err := f.db.Create(inv).Error; err != nil {
		t.Fatalf("seed share link: %v", err)
	}
	return inv.ID, token
}

func (f *atomicFixture) auditCount(t *testing.T, action types.AuditAction) int64 {
	t.Helper()
	var n int64
	if err := f.db.Model(&types.AuditLog{}).Where("action = ?", action).Count(&n).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	return n
}

func TestAccept_LegacyElevatedPendingInvitationYieldsViewer(t *testing.T) {
	f := newAtomicFixture(t)
	ctx := context.Background()
	invID := f.seedEmailInvitation(t, "u-bob", types.TenantRoleAdmin)

	member, err := f.svc.Accept(ctx, invID, "u-bob")
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	if member == nil || member.Role != types.TenantRoleViewer {
		t.Fatalf("legacy admin invitation must materialise viewer, got %+v", member)
	}
	// The persisted invitation row stays untouched in role terms — it was
	// accepted, not rewritten; the clamp happened at the membership write.
	var invRow types.TenantInvitation
	if err := f.db.First(&invRow, invID).Error; err != nil {
		t.Fatalf("read invitation: %v", err)
	}
	if invRow.Status != types.TenantInvitationStatusAccepted {
		t.Fatalf("invitation must be accepted, got %s", invRow.Status)
	}
	// Both audits committed exactly once with the state change.
	if n := f.auditCount(t, types.AuditActionMemberAdded); n != 1 {
		t.Fatalf("want 1 member_added audit, got %d", n)
	}
	if n := f.auditCount(t, types.AuditActionInvitationAccepted); n != 1 {
		t.Fatalf("want 1 invitation_accepted audit, got %d", n)
	}
}

func TestAcceptByToken_LegacyElevatedShareLinkYieldsViewer(t *testing.T) {
	f := newAtomicFixture(t)
	ctx := context.Background()
	linkID, token := f.seedShareLink(t, types.TenantRoleContributor)

	member, err := f.svc.AcceptByToken(ctx, token, "u-alice")
	if err != nil {
		t.Fatalf("accept-by-token: %v", err)
	}
	if member == nil || member.Role != types.TenantRoleViewer {
		t.Fatalf("legacy contributor link must materialise viewer, got %+v", member)
	}
	var linkRow types.TenantInvitation
	if err := f.db.First(&linkRow, linkID).Error; err != nil {
		t.Fatalf("read link: %v", err)
	}
	if linkRow.Status != types.TenantInvitationStatusPending {
		t.Fatalf("share link must stay pending, got %s", linkRow.Status)
	}
	if linkRow.AcceptedCount != 1 {
		t.Fatalf("usage counter must be bumped once, got %d", linkRow.AcceptedCount)
	}
	if n := f.auditCount(t, types.AuditActionMemberAdded); n != 1 {
		t.Fatalf("want 1 member_added audit, got %d", n)
	}
	if n := f.auditCount(t, types.AuditActionInvitationAccepted); n != 1 {
		t.Fatalf("want 1 invitation_accepted audit, got %d", n)
	}
}

func TestAccept_AuditFailureRollsBackAndRetries(t *testing.T) {
	f := newAtomicFixture(t)
	ctx := context.Background()
	invID := f.seedEmailInvitation(t, "u-bob", types.TenantRoleAdmin)

	f.audit.failAction = types.AuditActionMemberAdded
	f.audit.failures = 1
	if _, err := f.svc.Accept(ctx, invID, "u-bob"); err == nil {
		t.Fatalf("accept must fail when the member_added audit write fails")
	}
	// Everything rolled back: invitation still pending, no membership,
	// no audit rows at all.
	var invRow types.TenantInvitation
	if err := f.db.First(&invRow, invID).Error; err != nil {
		t.Fatalf("read invitation: %v", err)
	}
	if invRow.Status != types.TenantInvitationStatusPending {
		t.Fatalf("invitation must roll back to pending, got %s", invRow.Status)
	}
	existing, _ := f.memberSvc.GetMembership(ctx, "u-bob", 1)
	if existing != nil {
		t.Fatalf("membership must roll back, got %+v", existing)
	}
	if n := f.auditCount(t, types.AuditActionMemberAdded); n != 0 {
		t.Fatalf("failed audit must not persist, got %d rows", n)
	}

	// Retry after the outage heals commits the full unit exactly once.
	f.audit.failures = 0
	member, err := f.svc.Accept(ctx, invID, "u-bob")
	if err != nil {
		t.Fatalf("retry accept: %v", err)
	}
	if member == nil || member.Role != types.TenantRoleViewer {
		t.Fatalf("retry must materialise viewer, got %+v", member)
	}
	if n := f.auditCount(t, types.AuditActionMemberAdded); n != 1 {
		t.Fatalf("retry must leave exactly 1 member_added audit, got %d", n)
	}
	if n := f.auditCount(t, types.AuditActionInvitationAccepted); n != 1 {
		t.Fatalf("retry must leave exactly 1 invitation_accepted audit, got %d", n)
	}
}

func TestAcceptByToken_AuditFailureRollsBackAndRetries(t *testing.T) {
	f := newAtomicFixture(t)
	ctx := context.Background()
	linkID, token := f.seedShareLink(t, types.TenantRoleAdmin)

	f.audit.failAction = types.AuditActionInvitationAccepted
	f.audit.failures = 1
	if _, err := f.svc.AcceptByToken(ctx, token, "u-alice"); err == nil {
		t.Fatalf("accept-by-token must fail when the accepted audit write fails")
	}
	existing, _ := f.memberSvc.GetMembership(ctx, "u-alice", 1)
	if existing != nil {
		t.Fatalf("membership must roll back, got %+v", existing)
	}
	var linkRow types.TenantInvitation
	if err := f.db.First(&linkRow, linkID).Error; err != nil {
		t.Fatalf("read link: %v", err)
	}
	if linkRow.AcceptedCount != 0 {
		t.Fatalf("usage counter must roll back with the tx, got %d", linkRow.AcceptedCount)
	}

	f.audit.failures = 0
	member, err := f.svc.AcceptByToken(ctx, token, "u-alice")
	if err != nil {
		t.Fatalf("retry accept-by-token: %v", err)
	}
	if member == nil || member.Role != types.TenantRoleViewer {
		t.Fatalf("retry must materialise viewer, got %+v", member)
	}
	if n := f.auditCount(t, types.AuditActionInvitationAccepted); n != 1 {
		t.Fatalf("retry must leave exactly 1 accepted audit, got %d", n)
	}
}

func TestAccept_AlreadyMemberKeepsRoleAndAvoidsDuplicateAudit(t *testing.T) {
	f := newAtomicFixture(t)
	ctx := context.Background()
	// The invitee became an active owner through another path while the
	// invitation was still pending.
	if _, err := f.memberSvc.EnsureOwner(ctx, "u-bob", 1); err != nil {
		t.Fatalf("seed owner membership: %v", err)
	}
	invID := f.seedEmailInvitation(t, "u-bob", types.TenantRoleAdmin)

	member, err := f.svc.Accept(ctx, invID, "u-bob")
	if err != nil {
		t.Fatalf("accept must fold into already-member, got %v", err)
	}
	if member == nil || member.Role != types.TenantRoleOwner {
		t.Fatalf("existing membership must be preserved untouched, got %+v", member)
	}
	// The flip + accepted audit commit; no member_added duplicate.
	if n := f.auditCount(t, types.AuditActionInvitationAccepted); n != 1 {
		t.Fatalf("want 1 invitation_accepted audit, got %d", n)
	}
	if n := f.auditCount(t, types.AuditActionMemberAdded); n != 0 {
		t.Fatalf("already-member accept must not duplicate member_added, got %d", n)
	}
}

func TestAcceptByToken_RepeatJoinIsFullyIdempotent(t *testing.T) {
	f := newAtomicFixture(t)
	ctx := context.Background()
	linkID, token := f.seedShareLink(t, types.TenantRoleViewer)

	first, err := f.svc.AcceptByToken(ctx, token, "u-alice")
	if err != nil {
		t.Fatalf("first accept: %v", err)
	}
	second, err := f.svc.AcceptByToken(ctx, token, "u-alice")
	if err != nil {
		t.Fatalf("repeat accept must be idempotent, got %v", err)
	}
	if second == nil || second.ID != first.ID || second.Role != types.TenantRoleViewer {
		t.Fatalf("repeat must return the same membership, got %+v", second)
	}
	var linkRow types.TenantInvitation
	if err := f.db.First(&linkRow, linkID).Error; err != nil {
		t.Fatalf("read link: %v", err)
	}
	if linkRow.AcceptedCount != 1 {
		t.Fatalf("usage counter must not double-bump on repeat, got %d", linkRow.AcceptedCount)
	}
	if n := f.auditCount(t, types.AuditActionInvitationAccepted); n != 1 {
		t.Fatalf("repeat must not duplicate accepted audit, got %d", n)
	}
	if n := f.auditCount(t, types.AuditActionMemberAdded); n != 1 {
		t.Fatalf("repeat must not duplicate member_added audit, got %d", n)
	}
}
