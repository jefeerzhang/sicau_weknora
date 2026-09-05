package handler

// sicau-v1: viewers (course students) see exactly the workspace default
// agent in GET /agents. Non-viewer roles keep the full list.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type viewerFilterAgentSvc struct {
	interfaces.CustomAgentService
	agents []*types.CustomAgent
}

func (s *viewerFilterAgentSvc) ListAgents(context.Context) ([]*types.CustomAgent, error) {
	return s.agents, nil
}

// nilDisabledRepo answers "nothing disabled" — the tail of ListAgents
// filters own agents by this and must not affect the pinning under test.
type nilDisabledRepo struct {
	interfaces.TenantDisabledSharedAgentRepository
}

func (nilDisabledRepo) ListDisabledOwnAgentIDs(context.Context, uint64) ([]string, error) {
	return []string{}, nil
}



func newViewerFilterRouter(h *CustomAgentHandler, role types.TenantRole, tenant *types.Tenant) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		// ListAgents reads tenant/user via gin c.Get; role/tenant-info via
		// the request context (types.*FromContext). Seed both, like the
		// real auth middleware does.
		c.Set(types.UserIDContextKey.String(), "u-1")
		c.Set(types.TenantIDContextKey.String(), uint64(7))
		ctx := context.WithValue(c.Request.Context(), types.UserIDContextKey, "u-1")
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.TenantInfoContextKey, tenant)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}, errorCapture())
	r.GET("/agents", h.ListAgents)
	return r
}

func getAgents(t *testing.T, r *gin.Engine) []string {
	t.Helper()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/agents", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data []*types.CustomAgent `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	ids := make([]string, 0, len(resp.Data))
	for _, a := range resp.Data {
		ids = append(ids, a.ID)
	}
	return ids
}

func TestListAgents_ViewerPinnedToWorkspaceDefault(t *testing.T) {
	def := "course-agent"
	svc := &viewerFilterAgentSvc{agents: []*types.CustomAgent{
		{ID: "builtin-quick-answer", IsBuiltin: true},
		{ID: "builtin-smart-reasoning", IsBuiltin: true},
		{ID: "other-mine", CreatedBy: "u-1"},
		{ID: def, CreatedBy: "u-owner"},
	}}
	h := &CustomAgentHandler{service: svc, disabledRepo: nilDisabledRepo{}}
	r := newViewerFilterRouter(h, types.TenantRoleViewer, &types.Tenant{ID: 7, DefaultAgentID: &def})

	ids := getAgents(t, r)
	if len(ids) != 1 || ids[0] != def {
		t.Fatalf("viewer must see only the default agent, got %v", ids)
	}
}

func TestListAgents_ViewerWithoutDefaultKeepsFullList(t *testing.T) {
	svc := &viewerFilterAgentSvc{agents: []*types.CustomAgent{
		{ID: "builtin-quick-answer", IsBuiltin: true},
		{ID: "other-mine", CreatedBy: "u-1"},
	}}
	h := &CustomAgentHandler{service: svc, disabledRepo: nilDisabledRepo{}}
	r := newViewerFilterRouter(h, types.TenantRoleViewer, &types.Tenant{ID: 7})

	ids := getAgents(t, r)
	if len(ids) != 2 {
		t.Fatalf("no default configured: full list expected, got %v", ids)
	}
}

func TestListAgents_AdminKeepsFullList(t *testing.T) {
	def := "course-agent"
	svc := &viewerFilterAgentSvc{agents: []*types.CustomAgent{
		{ID: "builtin-quick-answer", IsBuiltin: true},
		{ID: def, CreatedBy: "u-owner"},
	}}
	h := &CustomAgentHandler{service: svc, disabledRepo: nilDisabledRepo{}}
	r := newViewerFilterRouter(h, types.TenantRoleOwner, &types.Tenant{ID: 7, DefaultAgentID: &def})

	ids := getAgents(t, r)
	if len(ids) != 2 {
		t.Fatalf("owner must keep the full list, got %v", ids)
	}
}
