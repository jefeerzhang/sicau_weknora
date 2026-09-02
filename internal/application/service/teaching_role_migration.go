package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const teachingLegacyRoleMigrationReason = "teaching_legacy_role_migration"

// TeachingRoleMigrator demotes legacy admin/contributor memberships to
// viewer and records zero/multi-owner workspaces for SuperAdmin recovery
// (#18). It never auto-picks an owner.
type TeachingRoleMigrator struct {
	db    *gorm.DB
	audit interfaces.AuditLogService // optional
}

// NewTeachingRoleMigrator constructs the migrator. audit may be nil.
func NewTeachingRoleMigrator(db *gorm.DB, audit interfaces.AuditLogService) *TeachingRoleMigrator {
	return &TeachingRoleMigrator{db: db, audit: audit}
}

// Run walks every tenant once. Idempotent: already-viewer rows are
// skipped; open anomalies are upserted without duplicating audit on
// re-run when no role actually changes.
func (m *TeachingRoleMigrator) Run(ctx context.Context) (*types.TeachingRoleMigrationReport, error) {
	report := &types.TeachingRoleMigrationReport{
		AnomalyTenantIDs: []uint64{},
	}
	if m == nil || m.db == nil {
		return report, fmt.Errorf("teaching role migrator is not configured")
	}

	var tenants []types.Tenant
	if err := m.db.WithContext(ctx).Select("id").Find(&tenants).Error; err != nil {
		return report, err
	}

	for _, tenant := range tenants {
		if err := m.migrateTenant(ctx, tenant.ID, report); err != nil {
			report.Failed++
			logger.Warnf(ctx, "teaching role migration failed for tenant %d: %v", tenant.ID, err)
		}
	}
	return report, nil
}

func (m *TeachingRoleMigrator) migrateTenant(
	ctx context.Context,
	tenantID uint64,
	report *types.TeachingRoleMigrationReport,
) error {
	var members []types.TenantMember
	if err := m.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ?", tenantID, types.TenantMemberStatusActive).
		Find(&members).Error; err != nil {
		return err
	}

	ownerIDs := make([]string, 0)
	for _, mem := range members {
		if mem.Role == types.TenantRoleOwner {
			ownerIDs = append(ownerIDs, mem.UserID)
		}
	}
	ownerCount := len(ownerIDs)
	if ownerCount != 1 {
		kind := types.WorkspaceOwnershipAnomalyZeroOwner
		if ownerCount > 1 {
			kind = types.WorkspaceOwnershipAnomalyMultiOwner
		}
		if err := m.upsertOpenAnomaly(ctx, tenantID, kind, ownerCount, ownerIDs); err != nil {
			return err
		}
		report.AnomalyTenantIDs = append(report.AnomalyTenantIDs, tenantID)
	} else {
		// Healthy single-owner workspace: clear a previously open anomaly
		// if an earlier run flagged it and ownership has since been fixed
		// outside this migrator (e.g. #19 resolve on another node).
		_ = m.db.WithContext(ctx).
			Model(&types.WorkspaceOwnershipAnomaly{}).
			Where("tenant_id = ? AND status = ?", tenantID, types.WorkspaceOwnershipAnomalyOpen).
			Updates(map[string]any{
				"status":          types.WorkspaceOwnershipAnomalyResolved,
				"resolution_note": "auto-cleared: single owner present",
				"updated_at":      time.Now(),
			}).Error
	}

	for _, mem := range members {
		if mem.Role != types.TenantRoleAdmin && mem.Role != types.TenantRoleContributor {
			report.Skipped++
			continue
		}
		oldRole := mem.Role
		res := m.db.WithContext(ctx).
			Model(&types.TenantMember{}).
			Where("user_id = ? AND tenant_id = ? AND role = ?", mem.UserID, tenantID, oldRole).
			Update("role", types.TenantRoleViewer)
		if res.Error != nil {
			report.Failed++
			logger.Warnf(ctx, "downgrade member %s tenant %d: %v", mem.UserID, tenantID, res.Error)
			continue
		}
		if res.RowsAffected == 0 {
			report.Skipped++
			continue
		}
		report.Downgraded++
		m.emitDowngradeAudit(ctx, tenantID, mem.UserID, oldRole)
	}
	return nil
}

func (m *TeachingRoleMigrator) upsertOpenAnomaly(
	ctx context.Context,
	tenantID uint64,
	kind types.WorkspaceOwnershipAnomalyKind,
	ownerCount int,
	ownerIDs []string,
) error {
	idsJSON, err := json.Marshal(ownerIDs)
	if err != nil {
		return err
	}
	now := time.Now()
	row := types.WorkspaceOwnershipAnomaly{
		TenantID:     tenantID,
		Kind:         kind,
		OwnerCount:   ownerCount,
		OwnerUserIDs: types.JSON(idsJSON),
		Status:       types.WorkspaceOwnershipAnomalyOpen,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	// Re-open on conflict: clear any prior resolve metadata so #19 can
	// pick the workspace up again after a regression.
	return m.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "tenant_id"}},
		DoUpdates: clause.Assignments(map[string]any{
			"kind":            kind,
			"owner_count":     ownerCount,
			"owner_user_ids":  types.JSON(idsJSON),
			"status":          types.WorkspaceOwnershipAnomalyOpen,
			"resolved_by":     nil,
			"resolved_at":     nil,
			"resolution_note": "",
			"updated_at":      now,
		}),
	}).Create(&row).Error
}

func (m *TeachingRoleMigrator) emitDowngradeAudit(
	ctx context.Context,
	tenantID uint64,
	targetUserID string,
	oldRole types.TenantRole,
) {
	details, _ := json.Marshal(map[string]string{
		"old_role": string(oldRole),
		"new_role": string(types.TenantRoleViewer),
		"reason":   teachingLegacyRoleMigrationReason,
		"source":   "system",
	})
	entry := &types.AuditLog{
		TenantID:     tenantID,
		ActorUserID:  "system",
		ActorRole:    "system",
		Action:       types.AuditActionMemberRoleChanged,
		TargetType:   "tenant_member",
		TargetUserID: targetUserID,
		Outcome:      types.AuditOutcomeSuccess,
		Details:      types.JSON(details),
		CreatedAt:    time.Now(),
	}
	if m.audit != nil {
		_ = m.audit.Log(ctx, entry)
		return
	}
	// Startup path may construct the migrator before AuditLogService is
	// wired; persist directly so demotions still leave an audit trail.
	_ = m.db.WithContext(ctx).Create(entry).Error
}

// ListOpenAnomalies returns unresolved ownership anomalies for SuperAdmin (#19).
func (m *TeachingRoleMigrator) ListOpenAnomalies(ctx context.Context) ([]*types.WorkspaceOwnershipAnomaly, error) {
	if m == nil || m.db == nil {
		return nil, fmt.Errorf("teaching role migrator is not configured")
	}
	var rows []*types.WorkspaceOwnershipAnomaly
	err := m.db.WithContext(ctx).
		Where("status = ?", types.WorkspaceOwnershipAnomalyOpen).
		Order("tenant_id ASC").
		Find(&rows).Error
	return rows, err
}

// ListTeacherCapableMembers returns active members of tenantID who have
// effective teacher capability (appointed Teacher or composite SuperAdmin).
// Used by the SuperAdmin ownership-recovery UI (#19) as selectable leads.
func (m *TeachingRoleMigrator) ListTeacherCapableMembers(
	ctx context.Context,
	tenantID uint64,
) []TeacherCapableMember {
	out := []TeacherCapableMember{}
	if m == nil || m.db == nil || tenantID == 0 {
		return out
	}
	var members []types.TenantMember
	if err := m.db.WithContext(ctx).
		Where("tenant_id = ? AND status = ?", tenantID, types.TenantMemberStatusActive).
		Find(&members).Error; err != nil {
		return out
	}
	ids := make([]string, 0, len(members))
	for _, mem := range members {
		ids = append(ids, mem.UserID)
	}
	if len(ids) == 0 {
		return out
	}
	var users []types.User
	if err := m.db.WithContext(ctx).Where("id IN ?", ids).Find(&users).Error; err != nil {
		return out
	}
	byID := make(map[string]types.User, len(users))
	for _, u := range users {
		byID[u.ID] = u
	}
	for _, mem := range members {
		u, ok := byID[mem.UserID]
		if !ok || !u.HasTeacherCapability() {
			continue
		}
		out = append(out, TeacherCapableMember{
			UserID:   u.ID,
			Email:    u.Email,
			Username: u.Username,
		})
	}
	return out
}

// TeacherCapableMember is a resolve-candidate for ownership recovery.
type TeacherCapableMember struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Username string `json:"username"`
}

// ResolveAnomaly makes newOwnerUserID the sole active owner of tenantID,
// demotes every other elevated membership to viewer, audits the changes,
// and marks the anomaly resolved (#19). The chosen user must already be an
// active member and have teacher capability (IsTeacher || IsSystemAdmin).
func (m *TeachingRoleMigrator) ResolveAnomaly(
	ctx context.Context,
	tenantID uint64,
	newOwnerUserID string,
	actorUserID string,
) error {
	if m == nil || m.db == nil {
		return fmt.Errorf("teaching role migrator is not configured")
	}
	if tenantID == 0 || newOwnerUserID == "" {
		return fmt.Errorf("tenant_id and new_owner_user_id are required")
	}

	var user types.User
	if err := m.db.WithContext(ctx).Where("id = ?", newOwnerUserID).First(&user).Error; err != nil {
		return err
	}
	if !user.HasTeacherCapability() {
		return ErrTeachingResolveRequiresTeacher
	}

	return m.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var target types.TenantMember
		if err := tx.Where(
			"user_id = ? AND tenant_id = ? AND status = ?",
			newOwnerUserID, tenantID, types.TenantMemberStatusActive,
		).First(&target).Error; err != nil {
			return ErrTeachingResolveTargetNotMember
		}

		var members []types.TenantMember
		if err := tx.Where(
			"tenant_id = ? AND status = ?",
			tenantID, types.TenantMemberStatusActive,
		).Find(&members).Error; err != nil {
			return err
		}

		for _, mem := range members {
			desired := types.TenantRoleViewer
			if mem.UserID == newOwnerUserID {
				desired = types.TenantRoleOwner
			}
			if mem.Role == desired {
				continue
			}
			oldRole := mem.Role
			if err := tx.Model(&types.TenantMember{}).
				Where("user_id = ? AND tenant_id = ?", mem.UserID, tenantID).
				Update("role", desired).Error; err != nil {
				return err
			}
			m.emitResolveAudit(ctx, tenantID, mem.UserID, actorUserID, oldRole, desired)
		}

		now := time.Now()
		res := tx.Model(&types.WorkspaceOwnershipAnomaly{}).
			Where("tenant_id = ? AND status = ?", tenantID, types.WorkspaceOwnershipAnomalyOpen).
			Updates(map[string]any{
				"status":          types.WorkspaceOwnershipAnomalyResolved,
				"resolved_by":     actorUserID,
				"resolved_at":     now,
				"resolution_note": "resolved by SuperAdmin",
				"owner_count":     1,
				"owner_user_ids":  types.JSON(mustJSON([]string{newOwnerUserID})),
				"updated_at":      now,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			// No open anomaly — still allow repair of memberships, but
			// surface that nothing was on the open list.
			return ErrTeachingResolveNoOpenAnomaly
		}
		return nil
	})
}

// Sentinel errors for SuperAdmin ownership recovery (#19).
var (
	ErrTeachingResolveRequiresTeacher = fmt.Errorf("selected account lacks effective teacher capability")
	ErrTeachingResolveTargetNotMember = fmt.Errorf("selected account is not an active member of the workspace")
	ErrTeachingResolveNoOpenAnomaly   = fmt.Errorf("no open ownership anomaly for this workspace")
)

func (m *TeachingRoleMigrator) emitResolveAudit(
	ctx context.Context,
	tenantID uint64,
	targetUserID, actorUserID string,
	oldRole, newRole types.TenantRole,
) {
	if m.audit == nil {
		return
	}
	details, _ := json.Marshal(map[string]string{
		"old_role": string(oldRole),
		"new_role": string(newRole),
		"reason":   "teaching_ownership_resolve",
		"source":   "superadmin",
	})
	_ = m.audit.Log(ctx, &types.AuditLog{
		TenantID:     tenantID,
		ActorUserID:  actorUserID,
		ActorRole:    "system_admin",
		Action:       types.AuditActionMemberRoleChanged,
		TargetType:   "tenant_member",
		TargetUserID: targetUserID,
		Outcome:      types.AuditOutcomeSuccess,
		Details:      types.JSON(details),
	})
}

func mustJSON(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
