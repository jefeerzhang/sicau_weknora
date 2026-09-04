package handler

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// RunTeachingRoleMigration triggers the idempotent teaching membership
// migrator (#18): demote admin/contributor to viewer and refresh the
// open ownership-anomaly list. SuperAdmin only.
func (h *SystemHandler) RunTeachingRoleMigration(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	if h.teachingMigrator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "teaching role migrator unavailable"})
		return
	}
	report, err := h.teachingMigrator.Run(ctx)
	if err != nil {
		logger.Errorf(ctx, "RunTeachingRoleMigration: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "migration failed"})
		return
	}
	h.emitAdminAudit(ctx, types.AuditActionMemberRoleChanged, nil, map[string]any{
		"reason":             "teaching_legacy_role_migration",
		"source":             "superadmin_trigger",
		"downgraded":         report.Downgraded,
		"skipped":            report.Skipped,
		"failed":             report.Failed,
		"anomaly_tenant_ids": report.AnomalyTenantIDs,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "data": report})
}

// ListWorkspaceOwnershipAnomalies returns open zero/multi-owner workspaces
// awaiting SuperAdmin recovery (#18/#19), including teacher-capable member
// candidates for each row.
func (h *SystemHandler) ListWorkspaceOwnershipAnomalies(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	if h.teachingMigrator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "teaching role migrator unavailable"})
		return
	}
	rows, err := h.teachingMigrator.ListOpenAnomalies(ctx)
	if err != nil {
		logger.Errorf(ctx, "ListWorkspaceOwnershipAnomalies: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list anomalies"})
		return
	}
	out := make([]gin.H, 0, len(rows))
	for _, row := range rows {
		item := gin.H{
			"id":             row.ID,
			"tenant_id":      row.TenantID,
			"kind":           row.Kind,
			"owner_count":    row.OwnerCount,
			"owner_user_ids": row.OwnerUserIDs,
			"status":         row.Status,
			"created_at":     row.CreatedAt,
			"updated_at":     row.UpdatedAt,
			"candidates":     h.teacherCapableMembers(ctx, row.TenantID),
		}
		out = append(out, item)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"anomalies": out, "total": len(out)}})
}

func (h *SystemHandler) teacherCapableMembers(ctx context.Context, tenantID uint64) []service.TeacherCapableMember {
	if h.teachingMigrator == nil {
		return []service.TeacherCapableMember{}
	}
	return h.teachingMigrator.ListTeacherCapableMembers(ctx, tenantID)
}

type resolveOwnershipRequest struct {
	NewOwnerUserID string `json:"new_owner_user_id"`
}

// ResolveWorkspaceOwnershipAnomaly assigns a teacher-capable account as
// the sole workspace lead and demotes other elevated members (#19).
func (h *SystemHandler) ResolveWorkspaceOwnershipAnomaly(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	if h.teachingMigrator == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "teaching role migrator unavailable"})
		return
	}
	tenantID, err := strconv.ParseUint(strings.TrimSpace(c.Param("tenant_id")), 10, 64)
	if err != nil || tenantID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tenant_id"})
		return
	}
	var req resolveOwnershipRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.NewOwnerUserID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new_owner_user_id is required"})
		return
	}
	actor, _ := types.UserIDFromContext(ctx)
	err = h.teachingMigrator.ResolveAnomaly(ctx, tenantID, strings.TrimSpace(req.NewOwnerUserID), actor)
	switch {
	case errors.Is(err, service.ErrTeachingResolveRequiresTeacher):
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	case errors.Is(err, service.ErrTeachingResolveTargetNotMember):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	case errors.Is(err, service.ErrTeachingResolveNoOpenAnomaly):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	case err != nil:
		logger.Errorf(ctx, "ResolveWorkspaceOwnershipAnomaly: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to resolve ownership anomaly"})
		return
	}
	// Audit trail: individual membership changes (downgrades and owner assignment)
	// are already emitted per tenant by teachingMigrator.ResolveAnomaly.
	c.JSON(http.StatusOK, gin.H{"success": true})
}
