package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

type tenantPersonalModelRepository struct {
	db *gorm.DB
}

func NewTenantPersonalModelRepository(db *gorm.DB) interfaces.TenantPersonalModelRepository {
	return &tenantPersonalModelRepository{db: db}
}

func (r *tenantPersonalModelRepository) Create(ctx context.Context, model *types.TenantPersonalModel) error {
	return r.db.WithContext(ctx).Create(model).Error
}

func (r *tenantPersonalModelRepository) List(ctx context.Context, tenantID uint64, userID string) ([]types.TenantPersonalModel, error) {
	var rows []types.TenantPersonalModel
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("updated_at DESC").
		Find(&rows).Error
	return rows, err
}

func (r *tenantPersonalModelRepository) GetByID(ctx context.Context, tenantID uint64, userID, id string) (*types.TenantPersonalModel, error) {
	var row types.TenantPersonalModel
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND user_id = ?", id, tenantID, userID).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &row, err
}

func (r *tenantPersonalModelRepository) Update(ctx context.Context, tenantID uint64, userID string, model *types.TenantPersonalModel) error {
	result := r.db.WithContext(ctx).Model(&types.TenantPersonalModel{}).
		Where("id = ? AND tenant_id = ? AND user_id = ?", model.ID, tenantID, userID).
		Updates(map[string]any{
			"name":        model.Name,
			"model_name":  model.ModelName,
			"base_url":    model.BaseURL,
			"provider":    model.Provider,
			"api_key_enc": model.APIKeyEnc,
			"enabled":     model.Enabled,
			"updated_at":  time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *tenantPersonalModelRepository) Delete(ctx context.Context, tenantID uint64, userID, id string) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND user_id = ?", id, tenantID, userID).
		Delete(&types.TenantPersonalModel{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *tenantPersonalModelRepository) DeleteAllForUser(ctx context.Context, tenantID uint64, userID string) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Delete(&types.TenantPersonalModel{}).Error
}

func (r *tenantPersonalModelRepository) CountByUser(ctx context.Context, tenantID uint64, userID string) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&types.TenantPersonalModel{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Count(&n).Error
	return n, err
}

func (r *tenantPersonalModelRepository) ListByTenant(ctx context.Context, tenantID uint64) ([]types.TenantPersonalModel, error) {
	var rows []types.TenantPersonalModel
	err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Order("updated_at DESC").
		Find(&rows).Error
	return rows, err
}
