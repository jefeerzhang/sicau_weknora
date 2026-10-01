package handler

// sicau-v1 ticket 04: workspace default agent via the tenant-KV surface
// (GET/PUT /tenants/kv/default-agent-id). The GET reads the request-scoped
// tenant; the PUT goes through the dedicated map-based update so clearing
// (empty agent_id) actually persists.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// defaultAgentTenantSvc records UpdateTenantDefaultAgentID calls; every
// other TenantService method is inherited as a nil-panicking stub because
// these tests never touch them.
type defaultAgentTenantSvc struct {
	interfaces.TenantService
	receivedID   uint64
	receivedAID  string
	updateCalled bool
}

func (s *defaultAgentTenantSvc) UpdateTenantDefaultAgentID(_ context.Context, tenantID uint64, agentID string) error {
	s.updateCalled = true
	s.receivedID = tenantID
	s.receivedAID = agentID
	return nil
}

func defaultAgentTestRouter(h *TenantHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.UserIDContextKey, "u-owner")
		c.Request = c.Request.WithContext(ctx)
	}, errorCapture())
	r.GET("/tenants/kv/:key", h.GetTenantKV)
	r.PUT("/tenants/kv/:key", h.UpdateTenantKV)
	return r
}

func withTenantInContext(t *testing.T, tenant *types.Tenant) gin.HandlerFunc {
	t.Helper()
	return func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.TenantInfoContextKey, tenant)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func TestGetTenantDefaultAgent_ReturnsValue(t *testing.T) {
	aid := "agent-1"
	h := &TenantHandler{service: &defaultAgentTenantSvc{}}
	r := gin.New()
	r.Use(withTenantInContext(t, &types.Tenant{ID: 7, DefaultAgentID: &aid}), errorCapture())
	r.GET("/tenants/kv/:key", h.GetTenantKV)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/tenants/kv/default-agent-id", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"agent_id":"agent-1"`) {
		t.Fatalf("expected agent-1, got %s", w.Body.String())
	}
}

func TestGetTenantDefaultAgent_EmptyWhenUnset(t *testing.T) {
	h := &TenantHandler{service: &defaultAgentTenantSvc{}}
	r := gin.New()
	r.Use(withTenantInContext(t, &types.Tenant{ID: 7}), errorCapture())
	r.GET("/tenants/kv/:key", h.GetTenantKV)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/tenants/kv/default-agent-id", nil))
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"agent_id":""`) {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestUpdateTenantDefaultAgent_SetsAndTrims(t *testing.T) {
	svc := &defaultAgentTenantSvc{}
	h := &TenantHandler{service: svc}
	r := gin.New()
	r.Use(withTenantInContext(t, &types.Tenant{ID: 7}), errorCapture())
	r.PUT("/tenants/kv/:key", h.UpdateTenantKV)

	req := httptest.NewRequest(http.MethodPut, "/tenants/kv/default-agent-id",
		bytes.NewReader([]byte(`{"agent_id":"  agent-9  "}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !svc.updateCalled || svc.receivedID != 7 || svc.receivedAID != "agent-9" {
		t.Fatalf("service call mismatch: called=%v id=%d aid=%q", svc.updateCalled, svc.receivedID, svc.receivedAID)
	}
}

func TestUpdateTenantDefaultAgent_ClearWithEmpty(t *testing.T) {
	svc := &defaultAgentTenantSvc{}
	h := &TenantHandler{service: svc}
	r := gin.New()
	r.Use(withTenantInContext(t, &types.Tenant{ID: 7}), errorCapture())
	r.PUT("/tenants/kv/:key", h.UpdateTenantKV)

	req := httptest.NewRequest(http.MethodPut, "/tenants/kv/default-agent-id",
		bytes.NewReader([]byte(`{"agent_id":""}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK || !svc.updateCalled || svc.receivedAID != "" {
		t.Fatalf("clearing must pass empty string through, status=%d body=%s svc=%+v",
			w.Code, w.Body.String(), svc)
	}
}

func TestUpdateTenantDefaultAgent_RejectsOverlong(t *testing.T) {
	svc := &defaultAgentTenantSvc{}
	h := &TenantHandler{service: svc}
	r := gin.New()
	r.Use(withTenantInContext(t, &types.Tenant{ID: 7}), errorCapture())
	r.PUT("/tenants/kv/:key", h.UpdateTenantKV)

	long := strings.Repeat("a", 64)
	req := httptest.NewRequest(http.MethodPut, "/tenants/kv/default-agent-id",
		bytes.NewReader([]byte(`{"agent_id":"`+long+`"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", w.Code, w.Body.String())
	}
	if svc.updateCalled {
		t.Fatal("service must not be called for overlong id")
	}
}
