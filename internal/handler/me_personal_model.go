package handler

// /me/personal-models — student-owned chat model CRUD (ADR-0001).

import (
	"errors"
	"net/http"

	"github.com/Tencent/WeKnora/internal/application/service"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type MePersonalModelHandler struct {
	service interfaces.TenantPersonalModelService
}

func NewMePersonalModelHandler(svc interfaces.TenantPersonalModelService) *MePersonalModelHandler {
	return &MePersonalModelHandler{service: svc}
}

func (h *MePersonalModelHandler) List(c *gin.Context) {
	items, err := h.service.List(c.Request.Context())
	if err != nil {
		c.Error(service.MapPersonalModelError(err))
		return
	}
	if items == nil {
		items = []types.TenantPersonalModelListItem{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"models": items}})
}

func (h *MePersonalModelHandler) Create(c *gin.Context) {
	var req personalModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	item, err := h.service.Create(c.Request.Context(), req.toInput())
	if err != nil {
		c.Error(service.MapPersonalModelError(err))
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": item})
}

func (h *MePersonalModelHandler) Get(c *gin.Context) {
	item, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		c.Error(service.MapPersonalModelError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

func (h *MePersonalModelHandler) Update(c *gin.Context) {
	var req personalModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.Error(apperrors.NewValidationError("Invalid request data").WithDetails(err.Error()))
		return
	}
	item, err := h.service.Update(c.Request.Context(), c.Param("id"), req.toInput())
	if err != nil {
		c.Error(service.MapPersonalModelError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": item})
}

func (h *MePersonalModelHandler) Delete(c *gin.Context) {
	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
		c.Error(service.MapPersonalModelError(err))
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ListMeta lists all student personal models in the workspace for 空间负责人.
// Credentials are never included (ToListItem).
func (h *MePersonalModelHandler) ListMeta(c *gin.Context) {
	items, err := h.service.ListMetaForTenant(c.Request.Context())
	if err != nil {
		c.Error(service.MapPersonalModelError(err))
		return
	}
	if items == nil {
		items = []types.TenantPersonalModelListItem{}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"models": items}})
}

type personalModelRequest struct {
	Name      string `json:"name"`
	ModelName string `json:"model_name"`
	BaseURL   string `json:"base_url"`
	Provider  string `json:"provider"`
	APIKey    string `json:"api_key"`
	Enabled   *bool  `json:"enabled"`
}

func (r personalModelRequest) toInput() interfaces.TenantPersonalModelInput {
	return interfaces.TenantPersonalModelInput{
		Name:      r.Name,
		ModelName: r.ModelName,
		BaseURL:   r.BaseURL,
		Provider:  r.Provider,
		APIKey:    r.APIKey,
		Enabled:   r.Enabled,
	}
}

// Silence unused imports from shared patterns.
var (
	_ = errors.Is
	_ = gorm.ErrRecordNotFound
)
