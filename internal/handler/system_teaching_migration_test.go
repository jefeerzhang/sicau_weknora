package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/database"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

func newTeachingHandlerFixture(t *testing.T) (*SystemHandler, *gorm.DB) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open("file:sys_teach_"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&types.Tenant{}, &types.User{}, &types.TenantMember{}, &types.TenantInvitation{}, &types.WorkspaceOwnershipAnomaly{}, &types.AuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	h := &SystemHandler{
		teachingMigrator: service.NewTeachingRoleMigrator(db, nil),
		teachingPhase:    service.NewTeachingMigrationPhase(db, nil),
	}
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

// --- #23: recovery failure returns an actionable error and retries ---

// resolveFailingAudit fails transaction-scoped audit writes while armed;
// the wrapped service persists entries once disarmed.
type resolveFailingAudit struct {
	interfaces.AuditLogService
	failLogTx int
}

func (f *resolveFailingAudit) LogTx(ctx context.Context, tx *gorm.DB, entry *types.AuditLog) error {
	if f.failLogTx > 0 {
		f.failLogTx--
		return errors.New("audit table unavailable")
	}
	return f.AuditLogService.LogTx(ctx, tx, entry)
}

func TestResolveWorkspaceOwnershipAnomaly_AuditFailureActionableErrorAndRetry(t *testing.T) {
	h, db := newTeachingHandlerFixture(t)
	realAudit := service.NewAuditLogService(apprepo.NewAuditLogRepository(db))
	failing := &resolveFailingAudit{AuditLogService: realAudit, failLogTx: 1}
	h.teachingMigrator = service.NewTeachingRoleMigrator(db, failing)

	_ = db.Create(&types.Tenant{ID: 3, Name: "t", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	_ = db.Create(&types.User{ID: "teacher", Username: "t", Email: "t@x.com", PasswordHash: "x", IsActive: true, IsTeacher: true})
	_ = db.Create(&types.TenantMember{UserID: "o1", TenantID: 3, Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive, JoinedAt: time.Now()})
	_ = db.Create(&types.TenantMember{UserID: "o2", TenantID: 3, Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive, JoinedAt: time.Now()})
	_ = db.Create(&types.TenantMember{UserID: "teacher", TenantID: 3, Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive, JoinedAt: time.Now()})
	_, _ = h.teachingMigrator.Run(context.Background())

	post := func() *httptest.ResponseRecorder {
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Request = c.Request.WithContext(context.WithValue(
				c.Request.Context(), types.UserIDContextKey, "actor-sa"))
			c.Next()
		}, errorCapture())
		r.POST("/workspace-anomalies/:tenant_id/resolve", h.ResolveWorkspaceOwnershipAnomaly)
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/workspace-anomalies/3/resolve",
			bytes.NewBufferString(`{"new_owner_user_id":"teacher"}`))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}

	w := post()
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("audit failure must surface 500, got %d body=%s", w.Code, w.Body.String())
	}
	var errBody struct {
		Error  string `json:"error"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if errBody.Detail == "" {
		t.Fatalf("actionable error must carry the underlying detail, got %s", w.Body.String())
	}

	// Nothing committed: memberships unchanged and the anomaly stays open.
	var o2 types.TenantMember
	if err := db.Where("user_id = ? AND tenant_id = ?", "o2", 3).First(&o2).Error; err != nil {
		t.Fatalf("read member: %v", err)
	}
	if o2.Role != types.TenantRoleOwner {
		t.Fatalf("failed recovery must roll back the demotion, got %s", o2.Role)
	}
	open, _ := h.teachingMigrator.ListOpenAnomalies(context.Background())
	if len(open) != 1 {
		t.Fatalf("anomaly must stay open after failure, got %d", len(open))
	}

	// Retry after the outage heals completes the recovery.
	failing.failLogTx = 0
	w = post()
	if w.Code != http.StatusOK {
		t.Fatalf("retry must succeed, got %d body=%s", w.Code, w.Body.String())
	}
	if err := db.Where("user_id = ? AND tenant_id = ?", "o2", 3).First(&o2).Error; err != nil {
		t.Fatalf("read member: %v", err)
	}
	if o2.Role != types.TenantRoleViewer {
		t.Fatalf("retry must demote the other owner, got %s", o2.Role)
	}
	open, _ = h.teachingMigrator.ListOpenAnomalies(context.Background())
	if len(open) != 0 {
		t.Fatalf("retry must resolve the anomaly, got %d", len(open))
	}
}

// --- #25: manual retry reuses the startup schema pre-check ---

func TestRunTeachingRoleMigration_MissingSchemaActionableAndBlocked(t *testing.T) {
	h, db := newTeachingHandlerFixture(t)
	// Drop audit_logs: the schema pre-check must fail the retry with an
	// actionable error and keep the blocked state, even though the dataset
	// has nothing to normalize.
	if err := db.Migrator().DropTable("audit_logs"); err != nil {
		t.Fatalf("drop audit_logs: %v", err)
	}
	database.CacheTeachingMigrationError("")

	r := gin.New()
	r.Use(errorCapture())
	r.POST("/run", h.RunTeachingRoleMigration)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/run", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("missing schema must fail the retry, got %d body=%s", w.Code, w.Body.String())
	}
	var body struct {
		Error  string `json:"error"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.Detail == "" {
		t.Fatalf("actionable error must carry the underlying detail, got %s", w.Body.String())
	}
	if blocked := database.CachedTeachingMigrationError(); blocked == "" {
		t.Fatalf("blocked state must be preserved after a failed retry")
	}

	// Schema repaired: the manual retry completes, records the real
	// audits and clears the blocked state.
	if err := db.AutoMigrate(&types.AuditLog{}); err != nil {
		t.Fatalf("repair audit_logs: %v", err)
	}
	_ = db.Create(&types.Tenant{ID: 4, Name: "t", CreatedAt: time.Now(), UpdatedAt: time.Now()})
	_ = db.Create(&types.TenantMember{UserID: "o", TenantID: 4, Role: types.TenantRoleOwner, Status: types.TenantMemberStatusActive, JoinedAt: time.Now()})
	_ = db.Create(&types.TenantMember{UserID: "a", TenantID: 4, Role: types.TenantRoleAdmin, Status: types.TenantMemberStatusActive, JoinedAt: time.Now()})
	_ = db.Create(&types.TenantInvitation{TenantID: 4, InviteeUserID: "u-new", Role: types.TenantRoleContributor, Status: types.TenantInvitationStatusPending, ExpiresAt: time.Now().Add(time.Hour)})

	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/run", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("retry after repair must succeed, got %d body=%s", w.Code, w.Body.String())
	}
	if blocked := database.CachedTeachingMigrationError(); blocked != "" {
		t.Fatalf("successful retry must clear the blocked state, got %q", blocked)
	}
	var member types.TenantMember
	if err := db.Where("user_id = ? AND tenant_id = ?", "a", 4).First(&member).Error; err != nil {
		t.Fatalf("read member: %v", err)
	}
	if member.Role != types.TenantRoleViewer {
		t.Fatalf("retry must demote the legacy admin, got %s", member.Role)
	}

	// A second successful retry changes nothing and duplicates no audit.
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/run", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("rerun must succeed, got %d body=%s", w.Code, w.Body.String())
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
