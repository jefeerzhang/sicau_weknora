package handler

// sicau-v1 ticket 02 (ADR-009-4): course invitations are viewer-only.
// Both invitation-creation endpoints — per-person invites and multi-use
// share links — must reject any role above viewer. Collaborators get
// their roles through the Owner+ member-management flow instead, so a
// leaked link can never mint anything more than a read-only student.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

// viewerOnlyInvitationSvc records which creation path was reached.
type viewerOnlyInvitationSvc struct {
	interfaces.TenantInvitationService
	created        bool
	shareLinked    bool
	createdRole    types.TenantRole
	shareLinkRole  types.TenantRole
}

func (s *viewerOnlyInvitationSvc) Create(_ context.Context, tenantID uint64, _ string, role types.TenantRole, _ *string, _ string) (*types.TenantInvitation, error) {
	s.created = true
	s.createdRole = role
	return &types.TenantInvitation{
		ID:      1,
		TenantID: tenantID,
		Role:    role,
		Status:  types.TenantInvitationStatusPending,
	}, nil
}

func (s *viewerOnlyInvitationSvc) CreateShareLink(_ context.Context, tenantID uint64, role types.TenantRole, _ *string, _ string) (*types.TenantInvitation, string, error) {
	s.shareLinked = true
	s.shareLinkRole = role
	return &types.TenantInvitation{
		ID:      2,
		TenantID: tenantID,
		Role:    role,
		Status:  types.TenantInvitationStatusPending,
	}, "", nil
}

func newViewerOnlyTestRouter(h *TenantInvitationHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), types.UserIDContextKey, "u-owner"))
	}, errorCapture())
	r.POST("/tenants/:id/invitations", h.CreateInvitation)
	r.POST("/tenants/:id/invite-links", h.CreateInviteLink)
	return r
}

func postViewerOnly(t *testing.T, r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func newViewerOnlyHandler(invites *viewerOnlyInvitationSvc) *TenantInvitationHandler {
	return &TenantInvitationHandler{
		invitationService: invites,
		userService: &autoAcceptUserSvc{
			user: &types.User{ID: "u-bob", Email: "bob@x.com", Username: "bob"},
		},
		memberService:    &autoAcceptMemberSvc{},
		systemSettingSvc: &autoAcceptSettingSvc{enabled: false},
	}
}

func TestCreateInvitation_RejectsRolesAboveViewer(t *testing.T) {
	for _, role := range []string{"contributor", "admin", "owner"} {
		t.Run(role, func(t *testing.T) {
			invites := &viewerOnlyInvitationSvc{}
			r := newViewerOnlyTestRouter(newViewerOnlyHandler(invites))

			w := postViewerOnly(t, r, "/tenants/7/invitations",
				`{"email":"bob@x.com","role":"`+role+`"}`)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("role=%s status=%d body=%s, want 400", role, w.Code, w.Body.String())
			}
			if invites.created {
				t.Fatalf("role=%s must not reach invitationService.Create", role)
			}
		})
	}
}

func TestCreateInvitation_ViewerStillAllowed(t *testing.T) {
	invites := &viewerOnlyInvitationSvc{}
	r := newViewerOnlyTestRouter(newViewerOnlyHandler(invites))

	w := postViewerOnly(t, r, "/tenants/7/invitations",
		`{"email":"bob@x.com","role":"viewer"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", w.Code, w.Body.String())
	}
	if !invites.created || invites.createdRole != types.TenantRoleViewer {
		t.Fatalf("viewer invite should be created, got created=%v role=%v", invites.created, invites.createdRole)
	}
}

func TestCreateInviteLink_RejectsRolesAboveViewer(t *testing.T) {
	for _, role := range []string{"contributor", "admin", "owner"} {
		t.Run(role, func(t *testing.T) {
			invites := &viewerOnlyInvitationSvc{}
			r := newViewerOnlyTestRouter(newViewerOnlyHandler(invites))

			w := postViewerOnly(t, r, "/tenants/7/invite-links",
				`{"role":"`+role+`"}`)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("role=%s status=%d body=%s, want 400", role, w.Code, w.Body.String())
			}
			if invites.shareLinked {
				t.Fatalf("role=%s must not reach invitationService.CreateShareLink", role)
			}
		})
	}
}

func TestCreateInviteLink_ViewerStillAllowed(t *testing.T) {
	invites := &viewerOnlyInvitationSvc{}
	r := newViewerOnlyTestRouter(newViewerOnlyHandler(invites))

	w := postViewerOnly(t, r, "/tenants/7/invite-links", `{"role":"viewer"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s, want 201", w.Code, w.Body.String())
	}
	if !invites.shareLinked || invites.shareLinkRole != types.TenantRoleViewer {
		t.Fatalf("viewer share link should be created, got linked=%v role=%v", invites.shareLinked, invites.shareLinkRole)
	}
}
