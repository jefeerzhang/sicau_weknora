package handler

// sicau-v1 notes: /me/notes CRUD. The caller (tenant + user) comes from the
// auth context; the frontend never passes user identifiers. Registered only
// on the web JWT path (see notes design §7 — IM synthetic accounts share a
// user_id, so these routes must never hang off IM auth).

import (
	"errors"
	"io"
	"net/http"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MeNoteHandler struct {
	service interfaces.TenantNoteService
}

func NewMeNoteHandler(service interfaces.TenantNoteService) *MeNoteHandler {
	return &MeNoteHandler{service: service}
}

// List godoc
// @Summary      我的笔记列表
// @Description  返回当前用户自己的笔记（id / 标题 / 更新时间），按更新时间倒序；不含正文
// @Tags         Me
// @Success      200  {object}  map[string]interface{}
// @Failure      401  {object}  apperrors.AppError  "未登录或缺少工作区上下文"
// @Security     Bearer
// @Router       /me/notes [get]
func (h *MeNoteHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}
	if items == nil {
		items = []types.TenantNoteListItem{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"notes": items}})
}

// Create godoc
// @Summary      新建笔记
// @Description  以给定 Markdown 内容创建一篇本人笔记（限额：200 篇/人、单篇 1MB）
// @Tags         Me
// @Accept       json
// @Param        request  body  object  true  "{\"content\":\"markdown\"}"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  apperrors.AppError  "内容超限或已达篇数上限"
// @Security     Bearer
// @Router       /me/notes [post]
func (h *MeNoteHandler) Create(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("invalid request body").WithDetails(err.Error()))
		return
	}
	note, err := h.service.Create(c.Request.Context(), req.Content)
	if err != nil {
		c.Error(mapNoteError(err))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": note})
}

// Get godoc
// @Summary      读取单篇笔记
// @Description  返回本人笔记全文；他人的笔记与不存在的笔记同样返回 404（不泄露存在性）
// @Tags         Me
// @Param        id  path  string  true  "笔记 ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /me/notes/{id} [get]
func (h *MeNoteHandler) Get(c *gin.Context) {
	note, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.Error(mapNoteError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": note})
}

// Update godoc
// @Summary      保存笔记
// @Description  全量覆盖本人笔记的 Markdown 内容（限额：单篇 1MB）
// @Tags         Me
// @Accept       json
// @Param        id      path  string  true  "笔记 ID"
// @Param        request  body  object  true  "{\"content\":\"markdown\"}"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /me/notes/{id} [put]
func (h *MeNoteHandler) Update(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("invalid request body").WithDetails(err.Error()))
		return
	}
	if err := h.service.Update(c.Request.Context(), c.Param("id"), req.Content); err != nil {
		c.Error(mapNoteError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// Delete godoc
// @Summary      删除笔记
// @Description  删除本人笔记；他人笔记与不存在的笔记同样返回 404
// @Tags         Me
// @Param        id  path  string  true  "笔记 ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /me/notes/{id} [delete]
func (h *MeNoteHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		c.Error(mapNoteError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// readUploadFull reads exactly size bytes from r. A single Read is not
// enough — the io.Reader contract allows short reads (slow networks,
// streaming sources), which would silently truncate the upload. Empty
// uploads and streams serving more than the declared size are rejected.
func readUploadFull(r io.Reader, size int64) ([]byte, error) {
	if size <= 0 {
		return nil, apperrors.NewValidationError("empty upload")
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, apperrors.NewValidationError("cannot read uploaded file")
	}
	var probe [1]byte
	if n, _ := r.Read(probe[:]); n > 0 {
		return nil, apperrors.NewValidationError("uploaded file is larger than declared")
	}
	return data, nil
}

// mapNoteError converts service-layer sentinels into HTTP-appropriate
// app errors: limits → 400, missing/foreign notes → 404 (existence is not
// leaked), anything else passes through for the error middleware.
func mapNoteError(err error) error {
	switch {
	case errors.Is(err, types.ErrNoteLimitReached), errors.Is(err, types.ErrNoteTooLarge),
		errors.Is(err, types.ErrNoteImageLimitReached), errors.Is(err, types.ErrNoteImageTooLarge),
		errors.Is(err, types.ErrNoteImageTypeUnsupport), errors.Is(err, types.ErrNoteImageQuotaExceeded):
		return apperrors.NewValidationError(err.Error())
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apperrors.NewNotFoundError("note not found")
	default:
		return err
	}
}

// UploadImage godoc
// @Summary      上传笔记图片
// @Description  multipart 上传（字段名 file），≤2MB；类型按 magic bytes 嗅探，仅 png/jpeg/gif/webp；每人 ≤200 张
// @Tags         Me
// @Accept       multipart/form-data
// @Param        file  formData  file  true  "图片文件"
// @Success      201  {object}  map[string]interface{}  "{"url":"/api/v1/me/notes/images/<id>","id":"<id>"}"
// @Failure      400  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /me/notes/images [post]
func (h *MeNoteHandler) UploadImage(c *gin.Context) {
	file, err := c.FormFile("file")
	if err != nil {
		c.Error(apperrors.NewValidationError("file field is required"))
		return
	}
	f, err := file.Open()
	if err != nil {
		c.Error(apperrors.NewValidationError("cannot open uploaded file").WithDetails(err.Error()))
		return
	}
	defer f.Close()
	data, err := readUploadFull(f, file.Size)
	if err != nil {
		c.Error(err)
		return
	}

	image, err := h.service.CreateImage(c.Request.Context(), data)
	if err != nil {
		c.Error(mapNoteError(err))
		return
	}
	url := types.NoteImageURLPrefix + image.ID
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    gin.H{"id": image.ID, "url": url, "mime": image.Mime},
	})
}

// GetImage godoc
// @Summary      读取笔记图片
// @Description  返回图片字节；仅 owner 可读（非本人 404）。带 private immutable 缓存头与 ETag
// @Tags         Me
// @Param        id  path  string  true  "图片 ID"
// @Success      200  {file}  binary
// @Failure      404  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /me/notes/images/{id} [get]
func (h *MeNoteHandler) GetImage(c *gin.Context) {
	image, err := h.service.GetImage(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.Error(mapNoteError(err))
		return
	}
	etag := `"` + image.ID + `"`
	c.Header("Cache-Control", "private, max-age=31536000, immutable")
	c.Header("ETag", etag)
	if c.GetHeader("If-None-Match") == etag {
		c.Status(http.StatusNotModified)
		return
	}
	c.Data(http.StatusOK, image.Mime, image.Bytes)
}

// DeleteImage godoc
// @Summary      删除笔记图片
// @Description  删除本人图片并即时释放配额
// @Tags         Me
// @Param        id  path  string  true  "图片 ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      404  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /me/notes/images/{id} [delete]
func (h *MeNoteHandler) DeleteImage(c *gin.Context) {
	if err := h.service.DeleteImage(c.Request.Context(), c.Param("id")); err != nil {
		c.Error(mapNoteError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
