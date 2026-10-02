package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type fakePersonalModelService struct {
	interfaces.TenantPersonalModelService
	list       []types.TenantPersonalModelListItem
	created    interfaces.TenantPersonalModelInput
	createErr  error
	listErr    error
	createItem types.TenantPersonalModelListItem
}

func (s *fakePersonalModelService) List(context.Context) ([]types.TenantPersonalModelListItem, error) {
	return s.list, s.listErr
}

func (s *fakePersonalModelService) Create(_ context.Context, in interfaces.TenantPersonalModelInput) (types.TenantPersonalModelListItem, error) {
	s.created = in
	if s.createErr != nil {
		return types.TenantPersonalModelListItem{}, s.createErr
	}
	if s.createItem.ID == "" {
		return types.TenantPersonalModelListItem{ID: "pm-1", ModelName: in.ModelName, BaseURL: in.BaseURL, HasCredential: true}, nil
	}
	return s.createItem, nil
}

func personalModelRouter(h *MePersonalModelHandler, enabled bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.UserIDContextKey, "u-student")
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
		cfg := &types.StudentPersonalModelsConfig{Enabled: enabled}
		ctx = context.WithValue(ctx, types.TenantInfoContextKey, &types.Tenant{ID: 7, StudentPersonalModels: cfg})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}, errorCapture())
	me := r.Group("/me/personal-models")
	{
		me.GET("", h.List)
		me.POST("", h.Create)
		me.GET("/:id", h.Get)
		me.PUT("/:id", h.Update)
		me.DELETE("/:id", h.Delete)
	}
	return r
}

func TestPersonalModels_CreateReturns201(t *testing.T) {
	svc := &fakePersonalModelService{}
	r := personalModelRouter(&MePersonalModelHandler{service: svc}, true)

	req := httptest.NewRequest(http.MethodPost, "/me/personal-models", bytes.NewReader([]byte(
		`{"model_name":"MiniMax-M3","base_url":"https://api.siliconflow.cn/v1","api_key":"sk-test"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.created.ModelName != "MiniMax-M3" {
		t.Fatalf("input not passed: %+v", svc.created)
	}
	if strings.Contains(w.Body.String(), "sk-test") {
		t.Fatal("api key must not appear in response")
	}
}

func TestPersonalModels_DisabledMapsTo403(t *testing.T) {
	svc := &fakePersonalModelService{createErr: service.ErrPersonalModelsDisabled}
	r := personalModelRouter(&MePersonalModelHandler{service: svc}, false)

	req := httptest.NewRequest(http.MethodPost, "/me/personal-models", bytes.NewReader([]byte(
		`{"model_name":"x","base_url":"https://api.siliconflow.cn/v1","api_key":"sk"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}

func TestPersonalModels_HostRejectedMapsTo400(t *testing.T) {
	svc := &fakePersonalModelService{createErr: service.ErrPersonalModelHost}
	r := personalModelRouter(&MePersonalModelHandler{service: svc}, true)

	req := httptest.NewRequest(http.MethodPost, "/me/personal-models", bytes.NewReader([]byte(
		`{"model_name":"x","base_url":"https://evil.example/v1","api_key":"sk"}`)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
}
