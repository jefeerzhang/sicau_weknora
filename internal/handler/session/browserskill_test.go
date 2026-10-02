package session

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/browserskill"
	"github.com/Tencent/WeKnora/internal/middleware"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestBrowserAccountStatusDoesNotRequireConversation(t *testing.T) {
	t.Setenv("BROWSERSKILL_BINARY", "/configured/bsk")
	t.Setenv("BROWSERSKILL_PUBLIC_URL", "")
	h := &Handler{browserSkill: browserskill.NewManager()}
	// No session service or conversation ID is supplied: pairing is a personal setting.
	request := httptest.NewRequest("GET", "/api/v1/me/browser", nil)
	ctx := context.WithValue(request.Context(), types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "alice")
	ctx = context.WithValue(ctx, types.UserContextKey, &types.User{ID: "alice", IsTeacher: true})
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = request.WithContext(ctx)
	h.BrowserSkillAccount(c)
	require.Equal(t, 200, response.Code)
	var payload struct {
		Data browserskill.Status `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &payload))
	require.True(t, payload.Data.Enabled)
	require.False(t, payload.Data.Connected)
}

func TestBrowserSkillRejectsStudentsEvenWithLegacyOwnerRole(t *testing.T) {
	t.Setenv("BROWSERSKILL_BINARY", "/configured/bsk")
	h := &Handler{browserSkill: browserskill.NewManager()}
	for _, endpoint := range []struct {
		name, method, body string
		handler            gin.HandlerFunc
	}{
		{"status", "GET", "", h.BrowserSkillAccount},
		{"pair", "POST", `{"action":"pair","origin":"https://classroom.example"}`, h.BrowserSkillAccount},
		{"extension", "GET", "", h.BrowserSkillDownload},
		{"conversation status", "GET", "", h.BrowserSkillConnection},
		{"conversation control", "POST", `{"action":"start"}`, h.BrowserSkillConnection},
	} {
		t.Run(endpoint.name, func(t *testing.T) {
			for _, role := range []types.TenantRole{types.TenantRoleViewer, types.TenantRoleOwner} {
				request := httptest.NewRequest(endpoint.method, "/browser", strings.NewReader(endpoint.body))
				ctx := context.WithValue(request.Context(), types.TenantIDContextKey, uint64(7))
				ctx = context.WithValue(ctx, types.UserIDContextKey, "student")
				ctx = context.WithValue(ctx, types.UserContextKey, &types.User{ID: "student"})
				ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
				response := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(response)
				c.Request = request.WithContext(ctx)
				endpoint.handler(c)
				require.Equal(t, 403, response.Code, "role=%s", role)
			}
		})
	}
}

func TestStudentCannotRequestLocalBrowserInQA(t *testing.T) {
	h := &Handler{}
	router := gin.New()
	router.Use(middleware.ErrorHandler())
	router.POST("/agent-chat/:session_id", h.AgentQA)
	request := httptest.NewRequest("POST", "/agent-chat/session", strings.NewReader(`{"query":"hello","local_browser_enabled":true}`))
	request.Header.Set("Content-Type", "application/json")
	ctx := context.WithValue(request.Context(), types.UserContextKey, &types.User{ID: "student"})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request.WithContext(ctx))
	require.Equal(t, 403, response.Code)
}

func TestBrowserAccountRequiresUser(t *testing.T) {
	h := &Handler{browserSkill: browserskill.NewManager()}
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest("GET", "/api/v1/me/browser", nil)
	h.BrowserSkillAccount(c)
	require.Equal(t, 401, response.Code)
}
