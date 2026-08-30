package repository

import (
	"context"
	"errors"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

type tenantNoteRepository struct {
	db *gorm.DB
}

func NewTenantNoteRepository(db *gorm.DB) interfaces.TenantNoteRepository {
	return &tenantNoteRepository{db: db}
}

// CreateWithLimit runs the count and the insert inside one transaction so
// the limit check and the write land together. The race window shrinks to
// the transaction itself; concurrent 200/201 submissions may both pass
// (accepted approximation, notes design §7).
func (r *tenantNoteRepository) CreateWithLimit(ctx context.Context, tenantID uint64, userID string, note *types.TenantNote) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&types.TenantNote{}).
			Where("tenant_id = ? AND user_id = ?", tenantID, userID).
			Count(&count).Error; err != nil {
			return err
		}
		if count >= types.MaxNotesPerUser {
			return types.ErrNoteLimitReached
		}
		return tx.Create(note).Error
	})
}

// ListHead selects a bounded content head instead of the full content —
// title derivation needs the first heading/line only, and pulling 200×1MB
// just to render a list would be reckless.
func (r *tenantNoteRepository) ListHead(ctx context.Context, tenantID uint64, userID string) ([]*types.TenantNote, error) {
	var notes []*types.TenantNote
	err := r.db.WithContext(ctx).
		Table("tenant_notes").
		Select("id, LEFT(content, ?) AS content, updated_at", types.NotePreviewContentHead).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Order("updated_at DESC").
		Scan(&notes).Error
	return notes, err
}

func (r *tenantNoteRepository) GetByID(ctx context.Context, tenantID uint64, userID, noteID string) (*types.TenantNote, error) {
	var note types.TenantNote
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND user_id = ?", noteID, tenantID, userID).
		First(&note).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &note, err
}

func (r *tenantNoteRepository) UpdateContent(ctx context.Context, tenantID uint64, userID, noteID, content string) error {
	result := r.db.WithContext(ctx).Model(&types.TenantNote{}).
		Where("id = ? AND tenant_id = ? AND user_id = ?", noteID, tenantID, userID).
		Updates(map[string]any{"content": content, "updated_at": time.Now()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *tenantNoteRepository) Delete(ctx context.Context, tenantID uint64, userID, noteID string) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND user_id = ?", noteID, tenantID, userID).
		Delete(&types.TenantNote{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *tenantNoteRepository) CountByUser(ctx context.Context, tenantID uint64, userID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&types.TenantNote{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Count(&count).Error
	return count, err
}

// CreateImage stores the image inside the same count+insert transaction
// pattern as notes (≤200 per user).
func (r *tenantNoteRepository) CreateImage(ctx context.Context, tenantID uint64, userID string, image *types.TenantNoteImage) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var count int64
		if err := tx.Model(&types.TenantNoteImage{}).
			Where("tenant_id = ? AND user_id = ?", tenantID, userID).
			Count(&count).Error; err != nil {
			return err
		}
		if count >= types.MaxNoteImagesPerUser {
			return types.ErrNoteImageLimitReached
		}
		return tx.Create(image).Error
	})
}

func (r *tenantNoteRepository) GetImageByID(ctx context.Context, tenantID uint64, userID, imageID string) (*types.TenantNoteImage, error) {
	var image types.TenantNoteImage
	err := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND user_id = ?", imageID, tenantID, userID).
		First(&image).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &image, err
}

func (r *tenantNoteRepository) DeleteImage(ctx context.Context, tenantID uint64, userID, imageID string) error {
	result := r.db.WithContext(ctx).
		Where("id = ? AND tenant_id = ? AND user_id = ?", imageID, tenantID, userID).
		Delete(&types.TenantNoteImage{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (r *tenantNoteRepository) CountImagesByUser(ctx context.Context, tenantID uint64, userID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&types.TenantNoteImage{}).
		Where("tenant_id = ? AND user_id = ?", tenantID, userID).
		Count(&count).Error
	return count, err
}

// DeleteUnreferencedImages GCs candidate images that no remaining note of
// this user references (ticket 07). The deleted note is already gone when
// this runs, so a LIKE against tenant_notes naturally excludes it. Best-
// effort per candidate: one unreferenced-but-busy image only postpones its
// own reclaim.
func (r *tenantNoteRepository) DeleteUnreferencedImages(ctx context.Context, tenantID uint64, userID, deletedNoteID string, candidateIDs []string) error {
	for _, id := range candidateIDs {
		var refs int64
		if err := r.db.WithContext(ctx).Model(&types.TenantNote{}).
			Where("tenant_id = ? AND user_id = ? AND id <> ? AND content LIKE ?",
				tenantID, userID, deletedNoteID, "%"+id+"%").
			Count(&refs).Error; err != nil {
			return err
		}
		if refs > 0 {
			continue
		}
		if err := r.db.WithContext(ctx).
			Where("id = ? AND tenant_id = ? AND user_id = ?", id, tenantID, userID).
			Delete(&types.TenantNoteImage{}).Error; err != nil {
			return err
		}
	}
	return nil
}
