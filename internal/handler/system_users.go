package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
)

// ListRegisteredUsers returns every registered account for the SuperAdmin
// user directory. Identity is projected by ToUserInfo (platform_identity),
// so the page does not invent a fourth role from workspace membership.
func (h *SystemHandler) ListRegisteredUsers(c *gin.Context) {
	ctx := logger.CloneContext(c.Request.Context())
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	users, total, err := h.userSvc.ListUsersPage(ctx, strings.TrimSpace(c.Query("q")), offset, limit)
	if err != nil {
		logger.Errorf(ctx, "ListRegisteredUsers failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}
	out := make([]*types.UserInfo, 0, len(users))
	for _, u := range users {
		out = append(out, u.ToUserInfo())
	}
	c.JSON(http.StatusOK, gin.H{"users": out, "total": total, "offset": offset, "limit": limit})
}
