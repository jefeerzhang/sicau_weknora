package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type directoryUserSvc struct {
	interfaces.UserService
	gotQuery string
}

func (s *directoryUserSvc) ListUsersPage(_ context.Context, query string, _, _ int) ([]*types.User, int64, error) {
	s.gotQuery = query
	return []*types.User{{
		ID: "u1", Username: "ada", Email: "ada@example.com", IsTeacher: true,
	}}, 1, nil
}

func TestListRegisteredUsersProjectsIdentity(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &directoryUserSvc{}
	h := &SystemHandler{userSvc: svc}
	r := gin.New()
	r.GET("/users", h.ListRegisteredUsers)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/users?q=ada&limit=20", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "ada", svc.gotQuery)
	var body struct {
		Total int `json:"total"`
		Users []struct {
			Email            string `json:"email"`
			PlatformIdentity string `json:"platform_identity"`
		} `json:"users"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Equal(t, 1, body.Total)
	require.Equal(t, "ada@example.com", body.Users[0].Email)
	require.Equal(t, "teacher", body.Users[0].PlatformIdentity)
}
