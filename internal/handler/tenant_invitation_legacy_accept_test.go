package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// HTTP-seam coverage for invitation acceptance over legacy pending rows
// (#21): whichever surface accepts the invitation (per-user inbox or
// shared link), the resulting membership is viewer and the audits commit
// with the state change. Real services over in-memory SQLite so the
// transactional boundary is exercised end to end.

func newLegacyInvitationHTTPFixture(t *testing.T, callerID string) (*gin.Engine, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	dsn := "file:legacy_inv_http_" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&types.TenantInvitation{},
		&types.TenantMember{},
		&types.AuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	auditSvc := service.NewAuditLogService(repository.NewAuditLogRepository(db))
	memberSvc := service.NewTenantMemberService(repository.NewTenantMemberRepository(db), auditSvc, nil, nil)
	invSvc := service.NewTenantInvitationService(
		db,
		repository.NewTenantInvitationRepository(db),
		memberSvc,
		auditSvc,
	)

	h := &TenantInvitationHandler{
		invitationService: invSvc,
		memberService:     memberSvc,
		userService:       &acceptByTokenUserSvc{homeTenant: 0},
		tenantService:     &acceptByTokenTenantSvc{},
	}

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(
			c.Request.Context(), types.UserIDContextKey, callerID))
		c.Next()
	}, errorCapture())
	r.POST("/me/invitations/:inv_id/accept", h.AcceptMyInvitation)
	r.POST("/me/invitations/accept-by-token", h.AcceptMyInvitationByToken)
	return r, db
}

func seedLegacyHTTPInvitation(t *testing.T, db *gorm.DB, invitee, token string, role types.TenantRole) uint64 {
	t.Helper()
	inv := &types.TenantInvitation{
		TenantID:      1,
		InviteeUserID: invitee,
		Token:         token,
		Role:          role,
		Status:        types.TenantInvitationStatusPending,
		ExpiresAt:     time.Now().Add(time.Hour),
	}
	if err := db.Create(inv).Error; err != nil {
		t.Fatalf("seed invitation: %v", err)
	}
	return inv.ID
}

func TestAcceptMyInvitation_HTTP_LegacyAdminInvitationYieldsViewer(t *testing.T) {
	r, db := newLegacyInvitationHTTPFixture(t, "u-bob")
	invID := seedLegacyHTTPInvitation(t, db, "u-bob", "", types.TenantRoleAdmin)

	req := httptest.NewRequest(http.MethodPost, "/me/invitations/"+strconv.FormatUint(invID, 10)+"/accept", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Membership struct {
				TenantID uint64 `json:"tenant_id"`
				Role     string `json:"role"`
			} `json:"membership"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if !resp.Success || resp.Data.Membership.Role != string(types.TenantRoleViewer) {
		t.Fatalf("legacy admin invitation must join as viewer over HTTP, got %+v", resp)
	}

	var member types.TenantMember
	if err := db.Where("user_id = ? AND tenant_id = ?", "u-bob", 1).First(&member).Error; err != nil {
		t.Fatalf("read member: %v", err)
	}
	if member.Role != types.TenantRoleViewer {
		t.Fatalf("persisted membership must be viewer, got %s", member.Role)
	}
	var invRow types.TenantInvitation
	if err := db.First(&invRow, invID).Error; err != nil {
		t.Fatalf("read invitation: %v", err)
	}
	if invRow.Status != types.TenantInvitationStatusAccepted {
		t.Fatalf("invitation must be accepted, got %s", invRow.Status)
	}
	var auditN int64
	if err := db.Model(&types.AuditLog{}).Where("action = ?", types.AuditActionInvitationAccepted).Count(&auditN).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if auditN != 1 {
		t.Fatalf("want 1 invitation_accepted audit, got %d", auditN)
	}
}

func TestAcceptMyInvitationByToken_HTTP_LegacyContributorLinkYieldsViewer(t *testing.T) {
	r, db := newLegacyInvitationHTTPFixture(t, "u-alice")
	seedLegacyHTTPInvitation(t, db, "", "legacy-http-link", types.TenantRoleContributor)

	body := []byte(`{"token":"legacy-http-link"}`)
	req := httptest.NewRequest(http.MethodPost, "/me/invitations/accept-by-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var resp struct {
		Success bool `json:"success"`
		Data    struct {
			Membership struct {
				Role string `json:"role"`
			} `json:"membership"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if !resp.Success || resp.Data.Membership.Role != string(types.TenantRoleViewer) {
		t.Fatalf("legacy contributor link must join as viewer over HTTP, got %+v", resp)
	}

	var member types.TenantMember
	if err := db.Where("user_id = ? AND tenant_id = ?", "u-alice", 1).First(&member).Error; err != nil {
		t.Fatalf("read member: %v", err)
	}
	if member.Role != types.TenantRoleViewer {
		t.Fatalf("persisted membership must be viewer, got %s", member.Role)
	}
	var auditN int64
	if err := db.Model(&types.AuditLog{}).Where("action = ?", types.AuditActionMemberAdded).Count(&auditN).Error; err != nil {
		t.Fatalf("count audits: %v", err)
	}
	if auditN != 1 {
		t.Fatalf("want 1 member_added audit, got %d", auditN)
	}
}
