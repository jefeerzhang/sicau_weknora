package handler

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

type teacherIdentityRequest struct {
	UserID string `json:"user_id"`
	Email  string `json:"email"`
}

// AppointTeacher appoints an existing account as a platform Teacher (#9).
func (h *SystemHandler) AppointTeacher(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	var req teacherIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}
	user, err := h.resolveTeacherTarget(ctx, req.UserID, req.Email)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if user.IsTeacher {
		h.emitAdminAudit(ctx, types.AuditActionTeacherAppointed, user, map[string]any{
			"target_email": user.Email, "idempotent": true,
		})
		c.JSON(http.StatusOK, user.ToUserInfo())
		return
	}
	user.IsTeacher = true
	if err := h.userSvc.UpdateUser(ctx, user); err != nil {
		logger.Errorf(ctx, "AppointTeacher update failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to appoint teacher"})
		return
	}
	h.emitAdminAudit(ctx, types.AuditActionTeacherAppointed, user, map[string]any{
		"target_email": user.Email, "idempotent": false,
	})
	c.JSON(http.StatusOK, user.ToUserInfo())
}

// RevokeTeacher removes the platform Teacher flag without deleting workspaces.
func (h *SystemHandler) RevokeTeacher(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	var req teacherIdentityRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request: " + err.Error()})
		return
	}
	user, err := h.resolveTeacherTarget(ctx, req.UserID, req.Email)
	if err != nil || user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if !user.IsTeacher {
		h.emitAdminAudit(ctx, types.AuditActionTeacherRevoked, user, map[string]any{
			"target_email": user.Email, "changed": false,
		})
		c.JSON(http.StatusOK, user.ToUserInfo())
		return
	}
	user.IsTeacher = false
	if err := h.userSvc.UpdateUser(ctx, user); err != nil {
		logger.Errorf(ctx, "RevokeTeacher update failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke teacher"})
		return
	}
	h.emitAdminAudit(ctx, types.AuditActionTeacherRevoked, user, map[string]any{
		"target_email": user.Email, "changed": true,
	})
	c.JSON(http.StatusOK, user.ToUserInfo())
}

// ListTeachers returns appointed Teachers for the SuperAdmin console.
func (h *SystemHandler) ListTeachers(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	users, total, err := h.userSvc.ListTeachers(ctx, offset, limit)
	if err != nil {
		logger.Errorf(ctx, "ListTeachers failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list teachers"})
		return
	}
	out := make([]*types.UserInfo, 0, len(users))
	for _, u := range users {
		out = append(out, u.ToUserInfo())
	}
	c.JSON(http.StatusOK, gin.H{"users": out, "total": total, "offset": offset, "limit": limit})
}

func (h *SystemHandler) resolveTeacherTarget(ctx context.Context, userID, email string) (*types.User, error) {
	userID = strings.TrimSpace(userID)
	email = strings.TrimSpace(email)
	if userID == "" && email == "" {
		return nil, nil
	}
	if userID != "" {
		return h.userSvc.GetUserByID(ctx, userID)
	}
	return h.userSvc.GetUserByEmail(ctx, email)
}
