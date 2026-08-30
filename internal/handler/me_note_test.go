package handler

// sicau-v1 notes: /me/notes CRUD. The handler maps service sentinels to
// HTTP (limits → 400, missing/foreign → 404); owner isolation itself lives
// in the service/repository (queries filter by ctx-derived tenant+user),
// so these tests pin the HTTP surface and the error mapping.

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

type fakeNoteService struct {
	interfaces.TenantNoteService
	list        []types.TenantNoteListItem
	created     string
	updateID    string
	updateBody  string
	deleteID    string
	getID       string
	getResult   *types.TenantNote
	getErr      error
	createErr   error
	updateErr   error
	deleteErr   error
}

func (s *fakeNoteService) List(ctx context.Context) ([]types.TenantNoteListItem, error) {
	return s.list, nil
}

func (s *fakeNoteService) Create(ctx context.Context, content string) (types.TenantNote, error) {
	if s.createErr != nil {
		return types.TenantNote{}, s.createErr
	}
	s.created = content
	return types.TenantNote{ID: "n-1", Content: content}, nil
}

func (s *fakeNoteService) Get(ctx context.Context, noteID string) (*types.TenantNote, error) {
	s.getID = noteID
	if s.getErr != nil {
		return nil, s.getErr
	}
	return s.getResult, nil
}

func (s *fakeNoteService) Update(ctx context.Context, noteID string, content string) error {
	s.updateID = noteID
	s.updateBody = content
	return s.updateErr
}

func (s *fakeNoteService) Delete(ctx context.Context, noteID string) error {
	s.deleteID = noteID
	return s.deleteErr
}

func noteTestRouter(h *MeNoteHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.UserIDContextKey, "u-student")
		ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}, errorCapture())
	me := r.Group("/me/notes")
	{
		me.GET("", h.List)
		me.POST("", h.Create)
		me.GET("/:id", h.Get)
		me.PUT("/:id", h.Update)
		me.DELETE("/:id", h.Delete)
		me.POST("/images", h.UploadImage)
		me.GET("/images/:id", h.GetImage)
		me.DELETE("/images/:id", h.DeleteImage)
	}
	return r
}

func doNote(t *testing.T, r *gin.Engine, method, path, body string) *httptest.ResponseRecorder {
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

func TestNotes_CreateReturns201(t *testing.T) {
	svc := &fakeNoteService{}
	r := noteTestRouter(&MeNoteHandler{service: svc})

	w := doNote(t, r, http.MethodPost, "/me/notes", `{"content":"# 笔记标题\n正文"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.created != "# 笔记标题\n正文" {
		t.Fatalf("content not passed through: %q", svc.created)
	}
}

func TestNotes_LimitMapsTo400(t *testing.T) {
	svc := &fakeNoteService{createErr: types.ErrNoteLimitReached}
	r := noteTestRouter(&MeNoteHandler{service: svc})

	w := doNote(t, r, http.MethodPost, "/me/notes", `{"content":"x"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "200 notes") {
		t.Fatalf("expected limit message, got %s", w.Body.String())
	}
}

func TestNotes_TooLargeMapsTo400(t *testing.T) {
	svc := &fakeNoteService{createErr: types.ErrNoteTooLarge}
	r := noteTestRouter(&MeNoteHandler{service: svc})

	w := doNote(t, r, http.MethodPost, "/me/notes", `{"content":"x"}`)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", w.Code, w.Body.String())
	}
}

func TestNotes_GetForeignOrMissingIs404(t *testing.T) {
	svc := &fakeNoteService{getErr: gorm.ErrRecordNotFound}
	r := noteTestRouter(&MeNoteHandler{service: svc})

	w := doNote(t, r, http.MethodGet, "/me/notes/some-id", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
}

func TestNotes_GetHappyPath(t *testing.T) {
	svc := &fakeNoteService{getResult: &types.TenantNote{ID: "n-9", Content: "hello"}}
	r := noteTestRouter(&MeNoteHandler{service: svc})

	w := doNote(t, r, http.MethodGet, "/me/notes/n-9", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if svc.getID != "n-9" {
		t.Fatalf("id not passed to service: %q", svc.getID)
	}
}

func TestNotes_UpdateAndDeletePassIDs(t *testing.T) {
	svc := &fakeNoteService{}
	r := noteTestRouter(&MeNoteHandler{service: svc})

	w := doNote(t, r, http.MethodPut, "/me/notes/n-5", `{"content":"updated"}`)
	if w.Code != http.StatusOK || svc.updateID != "n-5" || svc.updateBody != "updated" {
		t.Fatalf("update: status=%d id=%q body=%q", w.Code, svc.updateID, svc.updateBody)
	}

	w = doNote(t, r, http.MethodDelete, "/me/notes/n-5", "")
	if w.Code != http.StatusOK || svc.deleteID != "n-5" {
		t.Fatalf("delete: status=%d id=%q", w.Code, svc.deleteID)
	}
}

func TestNotes_UpdateForeignOrMissingIs404(t *testing.T) {
	svc := &fakeNoteService{updateErr: gorm.ErrRecordNotFound}
	r := noteTestRouter(&MeNoteHandler{service: svc})

	w := doNote(t, r, http.MethodPut, "/me/notes/n-5", `{"content":"x"}`)
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
}

// --- ticket 07：图片端点 ---

type fakeNoteImageService struct {
	interfaces.TenantNoteService
	image        types.TenantNoteImage
	imageErr     error
	createCalled bool
	deleteCalled bool
	deleteID     string
	deleteErr    error
}

func (s *fakeNoteImageService) CreateImage(ctx context.Context, data []byte) (types.TenantNoteImage, error) {
	s.createCalled = true
	if s.imageErr != nil {
		return types.TenantNoteImage{}, s.imageErr
	}
	return s.image, nil
}

func (s *fakeNoteImageService) GetImage(ctx context.Context, imageID string) (*types.TenantNoteImage, error) {
	if s.imageErr != nil {
		return nil, s.imageErr
	}
	img := s.image
	return &img, nil
}

func (s *fakeNoteImageService) DeleteImage(ctx context.Context, imageID string) error {
	s.deleteCalled = true
	s.deleteID = imageID
	return s.deleteErr
}

func TestNotes_UploadImageReturns201WithCapabilityURL(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 1, 2, 3, 4, 5, 6, 7, 8}
	svc := &fakeNoteImageService{image: types.TenantNoteImage{ID: "img-1", Mime: "image/png"}}
	r := noteTestRouter(&MeNoteHandler{service: svc})

	req := httptest.NewRequest(http.MethodPost, "/me/notes/images", nil)
	// 手工组 multipart（避免引入 mime/multipart 依赖断言）
	body := &bytes.Buffer{}
	body.WriteString("--X\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.png\"\r\n")
	body.WriteString("Content-Type: image/png\r\n\r\n")
	body.Write(png)
	body.WriteString("\r\n--X--\r\n")
	req = httptest.NewRequest(http.MethodPost, "/me/notes/images", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=X")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "/api/v1/me/notes/images/img-1") {
		t.Fatalf("expected capability url, got %s", w.Body.String())
	}
}

func TestNotes_UploadOversizeMapsTo400(t *testing.T) {
	svc := &fakeNoteImageService{imageErr: types.ErrNoteImageTooLarge}
	r := noteTestRouter(&MeNoteHandler{service: svc})

	req := httptest.NewRequest(http.MethodPost, "/me/notes/images", nil)
	body := &bytes.Buffer{}
	body.WriteString("--X\r\nContent-Disposition: form-data; name=\"file\"; filename=\"a.png\"\r\n\r\nbig\r\n--X--\r\n")
	req = httptest.NewRequest(http.MethodPost, "/me/notes/images", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=X")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s, want 400", w.Code, w.Body.String())
	}
}

func TestNotes_GetImageForeignOrMissingIs404(t *testing.T) {
	svc := &fakeNoteImageService{imageErr: gorm.ErrRecordNotFound}
	r := noteTestRouter(&MeNoteHandler{service: svc})

	w := doNote(t, r, http.MethodGet, "/me/notes/images/img-x", "")
	if w.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s, want 404", w.Code, w.Body.String())
	}
}

func TestNotes_DeleteImagePassesID(t *testing.T) {
	svc := &fakeNoteImageService{}
	r := noteTestRouter(&MeNoteHandler{service: svc})

	w := doNote(t, r, http.MethodDelete, "/me/notes/images/img-7", "")
	if w.Code != http.StatusOK || !svc.deleteCalled || svc.deleteID != "img-7" {
		t.Fatalf("status=%d called=%v id=%q", w.Code, svc.deleteCalled, svc.deleteID)
	}
}

// --- 100MB per-user image quota (user revision) ---

func TestDetectNoteImageMime_Whitelist(t *testing.T) {
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}
	if types.DetectNoteImageMime(png) != "image/png" {
		t.Fatal("png should sniff to image/png")
	}
	if types.DetectNoteImageMime([]byte("GIF89axxxxxx")) != "image/gif" {
		t.Fatal("GIF89a should sniff to image/gif")
	}
	if types.DetectNoteImageMime([]byte("<script>x</script>")) != "" {
		t.Fatal("non-image must not pass sniffing")
	}
}
