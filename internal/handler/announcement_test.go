package handler

// sicau-v1 announcements: /announcements HTTP surface. Mirrors the
// me_note_test seam — the handler maps service sentinels to HTTP
// (missing → 404); role gating itself lives in the service, so these
// tests pin the HTTP surface and the error mapping.

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
	"gorm.io/gorm"
)

type fakeAnnouncementService struct {
	interfaces.TenantAnnouncementService
	getErr           error
	deleteErr        error
	listCommentsErr  error
	createComment    types.AnnouncementCommentItem
	createCommentErr error
	deleteCommentErr error
}

func (s *fakeAnnouncementService) List(ctx context.Context) ([]*types.Announcement, error) {
	return []*types.Announcement{}, nil
}

func (s *fakeAnnouncementService) Get(ctx context.Context, announcementID string) (*types.Announcement, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return &types.Announcement{ID: announcementID, Title: "t"}, nil
}

func (s *fakeAnnouncementService) Delete(ctx context.Context, announcementID string) error {
	return s.deleteErr
}

func (s *fakeAnnouncementService) ListComments(ctx context.Context, announcementID string) ([]types.AnnouncementCommentItem, error) {
	if s.listCommentsErr != nil {
		return nil, s.listCommentsErr
	}
	return nil, nil
}

func (s *fakeAnnouncementService) CreateComment(ctx context.Context, announcementID, content string) (types.AnnouncementCommentItem, error) {
	if s.createCommentErr != nil {
		return types.AnnouncementCommentItem{}, s.createCommentErr
	}
	s.createComment.Content = content
	return s.createComment, nil
}

func (s *fakeAnnouncementService) DeleteComment(ctx context.Context, announcementID, commentID string) error {
	return s.deleteCommentErr
}

func announcementTestRouter(h *MeAnnouncementHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.UserIDContextKey, "u-student")
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
		ctx = context.WithValue(ctx, types.TenantRoleContextKey, types.TenantRoleViewer)
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}, errorCapture())
	g := r.Group("/announcements")
	{
		g.GET("", h.List)
		g.GET("/:id", h.Get)
		g.DELETE("/:id", h.Delete)
		g.GET("/:id/comments", h.ListComments)
		g.POST("/:id/comments", h.CreateComment)
		g.DELETE("/:id/comments/:cid", h.DeleteComment)
	}
	return r
}

func doAnnouncement(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, bytes.NewReader([]byte(body)))
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestAnnouncements_GetMissingIs404(t *testing.T) {
	svc := &fakeAnnouncementService{getErr: gorm.ErrRecordNotFound}
	r := announcementTestRouter(NewMeAnnouncementHandler(svc, nil))

	w := doAnnouncement(t, r, http.MethodGet, "/announcements/missing", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
}

func TestAnnouncements_DeleteMissingIs404(t *testing.T) {
	svc := &fakeAnnouncementService{deleteErr: gorm.ErrRecordNotFound}
	r := announcementTestRouter(NewMeAnnouncementHandler(svc, nil))

	w := doAnnouncement(t, r, http.MethodDelete, "/announcements/missing", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
}

func TestAnnouncements_ListCommentsOnMissingIs404(t *testing.T) {
	svc := &fakeAnnouncementService{listCommentsErr: gorm.ErrRecordNotFound}
	r := announcementTestRouter(NewMeAnnouncementHandler(svc, nil))

	w := doAnnouncement(t, r, http.MethodGet, "/announcements/missing/comments", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
}

func TestAnnouncements_CreateCommentOnMissingIs404(t *testing.T) {
	svc := &fakeAnnouncementService{createCommentErr: gorm.ErrRecordNotFound}
	r := announcementTestRouter(NewMeAnnouncementHandler(svc, nil))

	w := doAnnouncement(t, r, http.MethodPost, "/announcements/missing/comments", `{"content":"hi"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
}

func TestAnnouncements_DeleteCommentMissingIs404(t *testing.T) {
	svc := &fakeAnnouncementService{deleteCommentErr: gorm.ErrRecordNotFound}
	r := announcementTestRouter(NewMeAnnouncementHandler(svc, nil))

	w := doAnnouncement(t, r, http.MethodDelete, "/announcements/a-1/comments/c-1", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
}

func TestAnnouncements_DownloadAttachmentOnMissingIs404(t *testing.T) {
	svc := &fakeAnnouncementService{getErr: gorm.ErrRecordNotFound}
	h := NewMeAnnouncementHandler(svc, nil)
	r := announcementTestRouter(h)
	r.GET("/announcements/:id/attachments/:index", h.DownloadAttachment)

	w := doAnnouncement(t, r, http.MethodGet, "/announcements/missing/attachments/0", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
}

func TestAnnouncements_HappyPathStill200(t *testing.T) {
	svc := &fakeAnnouncementService{createComment: types.AnnouncementCommentItem{ID: "c-1", Content: "hi"}}
	r := announcementTestRouter(NewMeAnnouncementHandler(svc, nil))

	w := doAnnouncement(t, r, http.MethodGet, "/announcements/a-1", "")
	if w.Code != http.StatusOK {
		t.Fatalf("get: status=%d body=%s", w.Code, w.Body.String())
	}
	w = doAnnouncement(t, r, http.MethodPost, "/announcements/a-1/comments", `{"content":"hi"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("comment: status=%d body=%s", w.Code, w.Body.String())
	}
	w = doAnnouncement(t, r, http.MethodDelete, "/announcements/a-1/comments/c-1", "")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "success") {
		t.Fatalf("delete comment: status=%d body=%s", w.Code, w.Body.String())
	}
}
