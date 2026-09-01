package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

func tenantPolicyErrorCapture() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		if appErr, ok := c.Errors.Last().Err.(*apperrors.AppError); ok {
			c.JSON(appErr.HTTPCode, gin.H{"error": appErr})
		}
	}
}

type tenantPolicySettingService struct {
	interfaces.SystemSettingService
	enabled bool
}

func (s *tenantPolicySettingService) GetBool(context.Context, string, string, bool) bool {
	return s.enabled
}

func (s *tenantPolicySettingService) GetInt(_ context.Context, _ string, _ string, def int64) int64 {
	return def
}

type tenantPolicyUserService struct {
	interfaces.UserService
	user *types.User
}

func (s *tenantPolicyUserService) GetCurrentUser(context.Context) (*types.User, error) {
	return s.user, nil
}

func (s *tenantPolicyUserService) BuildLoginMemberships(context.Context, *types.User, *types.Tenant) []types.Membership {
	return []types.Membership{}
}

type tenantPolicyTenantService struct {
	interfaces.TenantService
	createCalls int
}

func (s *tenantPolicyTenantService) CreateTenant(_ context.Context, tenant *types.Tenant) (*types.Tenant, error) {
	s.createCalls++
	tenant.ID = 99
	return tenant, nil
}

func TestCreateTenantRejectsRegularUserWhenSelfServiceDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tenants := &tenantPolicyTenantService{}
	h := &TenantHandler{
		service:          tenants,
		userService:      &tenantPolicyUserService{user: &types.User{ID: "regular-user"}},
		config:           &config.Config{Tenant: &config.TenantConfig{}},
		systemSettingSvc: &tenantPolicySettingService{enabled: false},
	}
	r := gin.New()
	r.Use(tenantPolicyErrorCapture())
	r.POST("/tenants", h.CreateTenant)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tenants", bytes.NewBufferString(`{"name":"blocked"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if tenants.createCalls != 0 {
		t.Fatalf("CreateTenant called %d times, want 0", tenants.createCalls)
	}
	if !strings.Contains(w.Body.String(), `"code":2005`) {
		t.Fatalf("response missing typed disabled code: %s", w.Body.String())
	}
}

func TestCreateTenantAllowsCrossTenantSuperuserWhenSelfServiceDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tenants := &tenantPolicyTenantService{}
	h := &TenantHandler{
		service: tenants,
		userService: &tenantPolicyUserService{user: &types.User{
			ID:                  "super-user",
			TenantID:            1,
			CanAccessAllTenants: true,
		}},
		config:           &config.Config{Tenant: &config.TenantConfig{}},
		systemSettingSvc: &tenantPolicySettingService{enabled: false},
	}
	r := gin.New()
	r.Use(errorCapture())
	r.POST("/tenants", h.CreateTenant)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tenants", bytes.NewBufferString(`{"name":"admin-created"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if tenants.createCalls != 1 {
		t.Fatalf("CreateTenant called %d times, want 1", tenants.createCalls)
	}
}

func TestCreateTenantAllowsTeacherWhenSelfServiceEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tenants := &tenantPolicyTenantService{}
	on := true
	h := &TenantHandler{
		service: tenants,
		userService: &tenantPolicyUserService{user: &types.User{
			ID:        "teacher-1",
			TenantID:  1, // already has a home tenant; skip default-tenant UpdateUser
			IsTeacher: true,
		}},
		config: &config.Config{Tenant: &config.TenantConfig{
			SelfServiceCreationEnabled: &on,
		}},
		systemSettingSvc: &tenantPolicySettingService{enabled: true},
	}
	r := gin.New()
	r.Use(tenantPolicyErrorCapture())
	r.POST("/tenants", h.CreateTenant)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tenants", bytes.NewBufferString(`{"name":"course-a"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if tenants.createCalls != 1 {
		t.Fatalf("CreateTenant called %d times, want 1", tenants.createCalls)
	}
}

func TestCreateTenantRejectsNonTeacherWhenSelfServiceEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tenants := &tenantPolicyTenantService{}
	on := true
	h := &TenantHandler{
		service: tenants,
		userService: &tenantPolicyUserService{user: &types.User{
			ID:        "student-1",
			IsTeacher: false,
		}},
		config: &config.Config{Tenant: &config.TenantConfig{
			SelfServiceCreationEnabled: &on,
		}},
		systemSettingSvc: &tenantPolicySettingService{enabled: true},
	}
	r := gin.New()
	r.Use(tenantPolicyErrorCapture())
	r.POST("/tenants", h.CreateTenant)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tenants", bytes.NewBufferString(`{"name":"blocked"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if tenants.createCalls != 0 {
		t.Fatalf("CreateTenant called %d times, want 0", tenants.createCalls)
	}
}

func TestAuthMeProjectsTenantCreationCapability(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{
		userService: &tenantPolicyUserService{user: &types.User{
			ID:       "tenantless-user",
			Username: "tenantless",
			Email:    "tenantless@example.com",
		}},
		configInfo:       &config.Config{Tenant: &config.TenantConfig{}},
		systemSettingSvc: &tenantPolicySettingService{enabled: false},
	}
	r := gin.New()
	r.GET("/auth/me", h.GetCurrentUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/me", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"can_create_tenant":false`) {
		t.Fatalf("response missing capability: %s", w.Body.String())
	}
}

func TestAuthMeTeacherCanCreateTenantWhenSelfServiceOn(t *testing.T) {
	gin.SetMode(gin.TestMode)
	on := true
	h := &AuthHandler{
		userService: &tenantPolicyUserService{user: &types.User{
			ID:        "teacher-me",
			Username:  "teacher",
			Email:     "teacher@example.com",
			IsTeacher: true,
		}},
		configInfo: &config.Config{Tenant: &config.TenantConfig{
			SelfServiceCreationEnabled: &on,
		}},
		systemSettingSvc: &tenantPolicySettingService{enabled: true},
	}
	r := gin.New()
	r.GET("/auth/me", h.GetCurrentUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/me", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"can_create_tenant":true`) {
		t.Fatalf("teacher should be able to create tenant: %s", w.Body.String())
	}
}

// #13: the composite SuperAdmin must be able to create its own workspace
// without a separate teacher appointment, even when public self-service
// creation is disabled (the teaching default).
func TestCreateTenantAllowsSuperAdminWhenSelfServiceDisabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tenants := &tenantPolicyTenantService{}
	h := &TenantHandler{
		service: tenants,
		userService: &tenantPolicyUserService{user: &types.User{
			ID:            "sa-1",
			TenantID:      1, // composite SuperAdmin is tenantless at bootstrap; pre-set home to skip UpdateUser
			IsSystemAdmin: true,
		}},
		config:           &config.Config{Tenant: &config.TenantConfig{}},
		systemSettingSvc: &tenantPolicySettingService{enabled: false},
	}
	r := gin.New()
	r.Use(tenantPolicyErrorCapture())
	r.POST("/tenants", h.CreateTenant)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tenants", bytes.NewBufferString(`{"name":"sa-space"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if tenants.createCalls != 1 {
		t.Fatalf("CreateTenant called %d times, want 1", tenants.createCalls)
	}
}

func TestCreateTenantAllowsSuperAdminWhenSelfServiceEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tenants := &tenantPolicyTenantService{}
	on := true
	h := &TenantHandler{
		service: tenants,
		userService: &tenantPolicyUserService{user: &types.User{
			ID:            "sa-1",
			TenantID:      1,
			IsSystemAdmin: true,
		}},
		config: &config.Config{Tenant: &config.TenantConfig{
			SelfServiceCreationEnabled: &on,
		}},
		systemSettingSvc: &tenantPolicySettingService{enabled: true},
	}
	r := gin.New()
	r.Use(tenantPolicyErrorCapture())
	r.POST("/tenants", h.CreateTenant)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tenants", bytes.NewBufferString(`{"name":"sa-space"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if tenants.createCalls != 1 {
		t.Fatalf("CreateTenant called %d times, want 1", tenants.createCalls)
	}
}

// #13: /auth/me must advertise the SuperAdmin's inherited teacher
// capability regardless of the self-service flag, so the workspace-creation
// entry stays consistent with the backend gate.
func TestAuthMeSuperAdminCanCreateTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{
		userService: &tenantPolicyUserService{user: &types.User{
			ID:            "sa-1",
			Username:      "sa",
			Email:         "sa@example.com",
			IsSystemAdmin: true,
		}},
		configInfo:       &config.Config{Tenant: &config.TenantConfig{}},
		systemSettingSvc: &tenantPolicySettingService{enabled: false},
	}
	r := gin.New()
	r.GET("/auth/me", h.GetCurrentUser)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/auth/me", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"can_create_tenant":true`) {
		t.Fatalf("superadmin should be able to create tenant: %s", w.Body.String())
	}
}

// #14: an API-key platform principal must be able to create a tenant even if
// the underlying user row carries no teacher/admin flags, and /auth/me must
// advertise that so the frontend capability never disagrees with the backend
// gate in CreateTenant (catalogManager || HasTeacherCapability).
func TestAuthMePlatformCallerCanCreateTenant(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := &AuthHandler{
		userService: &tenantPolicyUserService{user: &types.User{
			ID:       "platform-caller",
			Username: "platform",
			Email:    "platform@example.com",
			// deliberately no CanAccessAllTenants / IsTeacher / IsSystemAdmin
		}},
		configInfo:       &config.Config{Tenant: &config.TenantConfig{}},
		systemSettingSvc: &tenantPolicySettingService{enabled: false},
	}
	r := gin.New()
	r.GET("/auth/me", h.GetCurrentUser)

	// Inject a platform API-key scope into the request context, the same way
	// the API-key middleware populates it for platform-scoped calls.
	ctx := types.WithTenantAPIKeyScope(context.Background(), types.TenantAPIKeyScope{
		ScopeType: types.APIKeyScopePlatform,
	})
	w := httptest.NewRecorder()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/auth/me", nil)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"can_create_tenant":true`) {
		t.Fatalf("platform caller should be able to create tenant: %s", w.Body.String())
	}
}

// recordEnsureOwner is a minimal TenantMemberService stub that records the
// EnsureOwner call so we can assert the composite SuperAdmin becomes the
// Owner of the workspace it just created (#13 AC1/AC3).
type recordEnsureOwner struct {
	interfaces.TenantMemberService
	userID   string
	tenantID uint64
	calls    int
}

func (m *recordEnsureOwner) EnsureOwner(_ context.Context, userID string, tenantID uint64) (*types.TenantMember, error) {
	m.userID = userID
	m.tenantID = tenantID
	m.calls++
	return nil, nil
}

func (m *recordEnsureOwner) ListByUser(_ context.Context, _ string) ([]*types.TenantMember, error) {
	return nil, nil
}

func TestCreateTenantSuperAdminBecomesOwner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tenants := &tenantPolicyTenantService{}
	ms := &recordEnsureOwner{}
	h := &TenantHandler{
		service:        tenants,
		memberService:  ms,
		userService: &tenantPolicyUserService{user: &types.User{
			ID:            "sa-1",
			TenantID:      1,
			IsSystemAdmin: true,
		}},
		config:           &config.Config{Tenant: &config.TenantConfig{}},
		systemSettingSvc: &tenantPolicySettingService{enabled: false},
	}
	r := gin.New()
	r.Use(tenantPolicyErrorCapture())
	r.POST("/tenants", h.CreateTenant)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/tenants", bytes.NewBufferString(`{"name":"sa-space"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if ms.calls != 1 || ms.userID != "sa-1" || ms.tenantID != 99 {
		t.Fatalf("EnsureOwner not called once for SuperAdmin on tenant 99: userID=%q tenant=%d calls=%d", ms.userID, ms.tenantID, ms.calls)
	}
}
