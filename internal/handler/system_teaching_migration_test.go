package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
)

func newTeachingHandlerFixture(t *testing.T) (*SystemHandler, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:sys_teach_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&types.Tenant{}, &types.User{}, &types.TenantMember{}, &types.WorkspaceOwnershipAnomaly{}, &types.AuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	h := &SystemHandler{teachingMigrator: service.NewTeachingRoleMigrator(db, nil)}
	return h, db
}

func TestRunTeachingRoleMigration_DowngradesAndReports(t *testing.T) {
	h, db := newTeachingHandlerFixture(t)
	_ = db.Create(&types.Tenant{ID: 1, Name: "t", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	_ = db.Create(&types.TenantMember{UserID: "o", TenantID: 1, Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive, JoinedAt: time.Now()})
	_ = db.Create(&types.TenantMember{UserID: "a", TenantID: 1, Role: types.TenantRoleAdmin, Status: types.TenantMemberStatusActive, JoinedAt: time.Now()})

	r := gin.New()
	r.Use(errorCapture())
	r.POST("/run", h.RunTeachingRoleMigration)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/run", nil)
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	data, _ := body["data"].(map[string]any)
	if data["downgraded"].(float64) != 1 {
		t.Fatalf("body=%s", w.Body.String())
	}
}

func TestResolveWorkspaceOwnershipAnomaly_RejectsNonTeacher(t *testing.T) {
	h, db := newTeachingHandlerFixture(t)
	_ = db.Create(&types.Tenant{ID: 2, Name: "t", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	_ = db.Create(&types.User{ID: "student", Username: "s", Email: "s@x.com", PasswordHash: "x", IsActive: true})
	_ = db.Create(&types.TenantMember{UserID: "o1", TenantID: 2, Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive, JoinedAt: time.Now()})
	_ = db.Create(&types.TenantMember{UserID: "o2", TenantID: 2, Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive, JoinedAt: time.Now()})
	_ = db.Create(&types.TenantMember{UserID: "student", TenantID: 2, Role: types.TenantRoleViewer, Status: types.TenantMemberStatusActive, JoinedAt: time.Now()})
	_, _ = h.teachingMigrator.Run(context.Background())

	r := gin.New()
	r.Use(errorCapture())
	r.POST("/workspace-anomalies/:tenant_id/resolve", h.ResolveWorkspaceOwnershipAnomaly)
	body := bytes.NewBufferString(`{"new_owner_user_id":"student"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/workspace-anomalies/2/resolve", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s, want 403", w.Code, w.Body.String())
	}
}
