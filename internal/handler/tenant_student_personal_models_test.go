package handler

// Student personal models workspace switch via tenant KV
// (GET/PUT /tenants/kv/student-personal-models). Default is off.

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

type personalModelsTenantSvc struct {
	interfaces.TenantService
	last *types.Tenant
}

func (s *personalModelsTenantSvc) UpdateTenant(_ context.Context, tenant *types.Tenant) (*types.Tenant, error) {
	s.last = tenant
	return tenant, nil
}

func TestGetStudentPersonalModels_DefaultDisabled(t *testing.T) {
	h := &TenantHandler{service: &personalModelsTenantSvc{}}
	r := gin.New()
	r.Use(withTenantInContext(t, &types.Tenant{ID: 7}), errorCapture())
	r.GET("/tenants/kv/:key", h.GetTenantKV)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/tenants/kv/student-personal-models", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"enabled":false`) {
		t.Fatalf("expected enabled:false, got %s", w.Body.String())
	}
}

func TestUpdateStudentPersonalModels_Enable(t *testing.T) {
	svc := &personalModelsTenantSvc{}
	h := &TenantHandler{service: svc}
	r := gin.New()
	r.Use(withTenantInContext(t, &types.Tenant{ID: 7}), errorCapture())
	r.PUT("/tenants/kv/:key", h.UpdateTenantKV)

	req := httptest.NewRequest(http.MethodPut, "/tenants/kv/student-personal-models",
		bytes.NewReader([]byte(`{"enabled":true,"allowed_hosts":[" llm.sicau.edu.cn "]}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.last == nil || svc.last.StudentPersonalModels == nil || !svc.last.StudentPersonalModels.Enabled {
		t.Fatalf("expected enabled config persisted, got %+v", svc.last)
	}
	if len(svc.last.StudentPersonalModels.AllowedHosts) != 1 ||
		svc.last.StudentPersonalModels.AllowedHosts[0] != "llm.sicau.edu.cn" {
		t.Fatalf("hosts not normalized: %+v", svc.last.StudentPersonalModels.AllowedHosts)
	}
}
