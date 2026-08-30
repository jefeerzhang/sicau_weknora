package repository

import (
	"context"
	"errors"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

type tenantAnnouncementRepository struct {
	db *gorm.DB
}

func NewTenantAnnouncementRepository(db *gorm.DB) interfaces.TenantAnnouncementRepository {
	return &tenantAnnouncementRepository{db: db}
}

func (r *tenantAnnouncementRepository) Create(ctx context.Context, announcement *types.Announcement) error {
	return r.db.WithContext(ctx).Create(announcement).Error
}

// ListHead selects a bounded content head — list cards render title and
// author only, so pulling every full notice body is waste.
func (r *tenantAnnouncementRepository) ListHead(ctx context.Context, tenantID uint64) ([]*types.Announcement, error) {
	var notes []*types.Announcement
	err := r.db.WithContext(ctx).
		Table("announcements").
		Select("id, tenant_id, user_id, title, LEFT(content, ?) AS content, attachments, created_at, updated_at", 2048).
		Where("tenant_id = ?", tenantID).
		Order("created_at DESC").
		Scan(&notes).Error
	return notes, err
}

func (r *tenantAnnouncementRepository) GetByID(ctx context.Context, tenantID uint64, announcementID string) (*types.Announcement, error) {
	var announcement types.Announcement
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", announcementID, tenantID).
		First(&announcement).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &announcement, err
}

func (r *tenantAnnouncementRepository) Delete(ctx context.Context, tenantID uint64, announcementID string) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ?", announcementID, tenantID).
		Delete(&types.Announcement{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *tenantAnnouncementRepository) ListComments(ctx context.Context, tenantID uint64, announcementID string) ([]*types.AnnouncementComment, error) {
	var comments []*types.AnnouncementComment
	err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND announcement_id = ?", tenantID, announcementID).
		Order("created_at ASC").
		Find(&comments).Error
	return comments, err
}

func (r *tenantAnnouncementRepository) CreateComment(ctx context.Context, comment *types.AnnouncementComment) error {
	return r.db.WithContext(ctx).Create(comment).Error
}

func (r *tenantAnnouncementRepository) GetComment(ctx context.Context, tenantID uint64, announcementID, commentID string) (*types.AnnouncementComment, error) {
	var comment types.AnnouncementComment
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND announcement_id = ?", commentID, tenantID, announcementID).
		First(&comment).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &comment, err
}

func (r *tenantAnnouncementRepository) DeleteComment(ctx context.Context, tenantID uint64, announcementID, commentID string) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND announcement_id = ?", commentID, tenantID, announcementID).
		Delete(&types.AnnouncementComment{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *tenantAnnouncementRepository) CountCommentsByAnnouncement(ctx context.Context, tenantID uint64, announcementID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&types.AnnouncementComment{}).
		Where("tenant_id = ? AND announcement_id = ?", tenantID, announcementID).
		Count(&count).Error
	return count, err
}
