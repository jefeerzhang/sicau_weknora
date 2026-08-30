package handler

// sicau-v1 announcements: /announcements CRUD + comments + attachment
// download. Posting is Contributor+; reads are Viewer+; deletes are
// author-or-admin (service-enforced via role from ctx). Web JWT path only
// (IM synthetic accounts share a user_id — notes design §7 applies here too).

import (
	"net/http"
	"path/filepath"
	"strings"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
)

type MeAnnouncementHandler struct {
	service interfaces.TenantAnnouncementService
	fileSvc interfaces.FileService
}

func NewMeAnnouncementHandler(service interfaces.TenantAnnouncementService, fileSvc interfaces.FileService) *MeAnnouncementHandler {
	return &MeAnnouncementHandler{service: service, fileSvc: fileSvc}
}

// List godoc
// @Summary      公告列表
// @Description  全员可读；按创建时间倒序；不含正文与留言
// @Tags         Announcements
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /announcements [get]
func (h *MeAnnouncementHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"announcements": items}})
}

// Create godoc
// @Summary      发布公告
// @Description  multipart：title + content + files[]（≤5 个、单个 ≤50MB、白名单类型）；Contributor+
// @Tags         Announcements
// @Accept       multipart/form-data
// @Param        title    formData  string  true  "标题"
// @Param        content  formData  string  false "正文（Markdown）"
// @Param        files    formData  file    false "附件（可多个）"
// @Success      201  {object}  map[string]interface{}
// @Failure      400  {object}  apperrors.AppError
// @Failure      403  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /announcements [post]
func (h *MeAnnouncementHandler) Create(c *gin.Context) {
	title := strings.TrimSpace(c.PostForm("title"))
	content := c.PostForm("content")

	form, err := c.MultipartForm()
	if err != nil {
		c.Error(apperrors.NewValidationError("invalid multipart form").WithDetails(err.Error()))
		return
	}
	var files []interfaces.AnnouncementUploadFile
	if form != nil {
		for _, fh := range form.File["files"] {
			f, err := fh.Open()
			if err != nil {
				c.Error(apperrors.NewValidationError("cannot read uploaded file: " + fh.Filename))
				return
			}
			// 50MB cap re-checked here: MultipartForm already parsed the
			// body, but the per-file read is bounded explicitly.
			data := make([]byte, fh.Size)
			if _, err := f.Read(data); err != nil && fh.Size > 0 {
				f.Close()
				c.Error(apperrors.NewValidationError("cannot read uploaded file: " + fh.Filename))
				return
			}
			f.Close()
			files = append(files, interfaces.AnnouncementUploadFile{Name: filepath.Base(fh.Filename), Data: data})
		}
	}

	announcement, err := h.service.Create(c.Request.Context(), title, content, files)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": announcement})
}

// Get godoc
// @Summary      公告详情
// @Description  全文 + 附件元数据 + 作者名
// @Tags         Announcements
// @Param        id  path  string  true  "公告 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /announcements/{id} [get]
func (h *MeAnnouncementHandler) Get(c *gin.Context) {
	announcement, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": announcement})
}

// Delete godoc
// @Summary      删除公告
// @Description  作者或 admin；连带删除附件文件与留言
// @Tags         Announcements
// @Param        id  path  string  true  "公告 ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      403  {object}  apperrors.AppError
// @Failure      404  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /announcements/{id} [delete]
func (h *MeAnnouncementHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DownloadAttachment godoc
// @Summary      下载公告附件
// @Description  按 jsonb 数组下标取附件并流式返回；登录即可
// @Tags         Announcements
// @Param        id     path  string  true  "公告 ID"
// @Param        index  path  int     true  "附件下标（0 起）"
// @Success      200  {file}  binary
// @Failure      404  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /announcements/{id}/attachments/{index} [get]
func (h *MeAnnouncementHandler) DownloadAttachment(c *gin.Context) {
	announcement, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.Error(err)
		return
	}
	index := 0
	for _, ch := range c.Param("index") {
		if ch < '0' || ch > '9' {
			c.Error(apperrors.NewValidationError("invalid attachment index"))
			return
		}
		index = index*10 + int(ch-'0')
	}
	if index < 0 || index >= len(announcement.Attachments) {
		c.Error(apperrors.NewNotFoundError("attachment not found"))
		return
	}
	att := announcement.Attachments[index]
	stream, err := h.fileSvc.GetFile(c.Request.Context(), att.Path)
	if err != nil {
		logger.Errorf(c.Request.Context(), "[announcements] attachment open failed: path=%s err=%v", att.Path, err)
		c.Error(apperrors.NewNotFoundError("attachment not found"))
		return
	}
	defer stream.Close()

	c.Header("Content-Disposition", "attachment; filename=\""+strings.ReplaceAll(att.Name, "\"", "")+"\"")
	c.DataFromReader(http.StatusOK, att.Size, "application/octet-stream", stream, nil)
}

// ListComments godoc
// @Summary      公告留言列表
// @Description  平铺，时间正序，含作者名
// @Tags         Announcements
// @Param        id  path  string  true  "公告 ID"
// @Success      200  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /announcements/{id}/comments [get]
func (h *MeAnnouncementHandler) ListComments(c *gin.Context) {
	items, err := h.service.ListComments(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.Error(err)
		return
	}
	if items == nil {
		items = []types.AnnouncementCommentItem{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"comments": items}})
}

// CreateComment godoc
// @Summary      发表留言
// @Description  纯文本；全员可用；公告不存在 404
// @Tags         Announcements
// @Accept       json
// @Param        id      path  string  true  "公告 ID"
// @Param        request  body  object  true  "{\"content\":\"text\"}"
// @Success      201  {object}  map[string]interface{}
// @Security     Bearer
// @Router       /announcements/{id}/comments [post]
func (h *MeAnnouncementHandler) CreateComment(c *gin.Context) {
	var req struct {
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("invalid request body").WithDetails(err.Error()))
		return
	}
	item, err := h.service.CreateComment(c.Request.Context(), c.Param("id"), req.Content)
	if err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": item})
}

// DeleteComment godoc
// @Summary      删除留言
// @Description  留言作者或 admin
// @Tags         Announcements
// @Param        id   path  string  true  "公告 ID"
// @Param        cid  path  string  true  "留言 ID"
// @Success      200  {object}  map[string]interface{}
// @Failure      403  {object}  apperrors.AppError
// @Failure      404  {object}  apperrors.AppError
// @Security     Bearer
// @Router       /announcements/{id}/comments/{cid} [delete]
func (h *MeAnnouncementHandler) DeleteComment(c *gin.Context) {
	if err := h.service.DeleteComment(c.Request.Context(), c.Param("id"), c.Param("cid")); err != nil {
		c.Error(err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}
