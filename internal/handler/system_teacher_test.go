package handler

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type teacherUserSvc struct {
	interfaces.UserService
	byEmail map[string]*types.User
	byID    map[string]*types.User
	updated []*types.User
	listed  []*types.User
}

func (s *teacherUserSvc) GetUserByEmail(_ context.Context, email string) (*types.User, error) {
	return s.byEmail[email], nil
}

func (s *teacherUserSvc) GetUserByID(_ context.Context, id string) (*types.User, error) {
	return s.byID[id], nil
}

func (s *teacherUserSvc) UpdateUser(_ context.Context, user *types.User) error {
	s.updated = append(s.updated, user)
	return nil
}

func (s *teacherUserSvc) ListTeachers(context.Context, int, int) ([]*types.User, int64, error) {
	return s.listed, int64(len(s.listed)), nil
}

func TestAppointTeacherIdempotent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	u := &types.User{ID: "u1", Email: "t@example.com", IsTeacher: true}
	svc := &teacherUserSvc{byEmail: map[string]*types.User{"t@example.com": u}}
	h := &SystemHandler{userSvc: svc}
	r := gin.New()
	r.POST("/appoint", h.AppointTeacher)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/appoint", bytes.NewBufferString(`{"email":"t@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	assert.Empty(t, svc.updated)
}

func TestAppointTeacherSetsFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	u := &types.User{ID: "u1", Email: "t@example.com", IsTeacher: false}
	svc := &teacherUserSvc{byEmail: map[string]*types.User{"t@example.com": u}}
	h := &SystemHandler{userSvc: svc}
	r := gin.New()
	r.POST("/appoint", h.AppointTeacher)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/appoint", bytes.NewBufferString(`{"email":"t@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, svc.updated, 1)
	assert.True(t, svc.updated[0].IsTeacher)
}

func TestRevokeTeacherClearsFlag(t *testing.T) {
	gin.SetMode(gin.TestMode)
	u := &types.User{ID: "u1", Email: "t@example.com", IsTeacher: true}
	svc := &teacherUserSvc{byEmail: map[string]*types.User{"t@example.com": u}}
	h := &SystemHandler{userSvc: svc}
	r := gin.New()
	r.POST("/revoke", h.RevokeTeacher)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/revoke", bytes.NewBufferString(`{"email":"t@example.com"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)
	require.Len(t, svc.updated, 1)
	assert.False(t, svc.updated[0].IsTeacher)
}

func TestListTeachers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &teacherUserSvc{listed: []*types.User{
		{ID: "u1", Email: "a@example.com", IsTeacher: true},
		{ID: "u2", Email: "sa@example.com", IsTeacher: false, IsSystemAdmin: true},
	}}
	h := &SystemHandler{userSvc: svc}
	r := gin.New()
	r.GET("/teachers", h.ListTeachers)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/teachers", nil))
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "a@example.com")
	// CONTEXT.md "平台身份可见性": in the teacher-management scope the
	// SuperAdmin sees each managed account's platform identity, so the list
	// must carry it (appointed teacher = teacher; composite = superadmin).
	assert.Contains(t, w.Body.String(), `"platform_identity":"teacher"`)
	assert.Contains(t, w.Body.String(), `"platform_identity":"superadmin"`)
}
