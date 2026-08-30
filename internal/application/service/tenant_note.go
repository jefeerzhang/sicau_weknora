package service

import (
	"context"
	"strings"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

type tenantNoteService struct {
	repo interfaces.TenantNoteRepository
}

func NewTenantNoteService(repo interfaces.TenantNoteRepository) interfaces.TenantNoteService {
	return &tenantNoteService{repo: repo}
}

// caller resolves whose notes this call may touch. A missing principal is
// an error rather than a default: falling back to any other identity would
// let one member read or overwrite another's notes (notes design N-1).
func (s *tenantNoteService) caller(ctx context.Context) (uint64, string, error) {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		return 0, "", apperrors.NewUnauthorizedError(
			"workspace context is required to access notes")
	}
	userID, ok := types.UserIDFromContext(ctx)
	if !ok || userID == "" {
		return 0, "", apperrors.NewUnauthorizedError(
			"user context is required to access notes")
	}
	return tenantID, userID, nil
}

// List returns the caller's notes as (id, title, updated_at) items, newest
// first. Content never leaves the list endpoint.
func (s *tenantNoteService) List(ctx context.Context) ([]types.TenantNoteListItem, error) {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	notes, err := s.repo.ListHead(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	items := make([]types.TenantNoteListItem, 0, len(notes))
	for _, n := range notes {
		if n == nil {
			continue
		}
		items = append(items, types.TenantNoteListItem{
			ID:        n.ID,
			Title:     deriveNoteTitle(n.Content),
			UpdatedAt: n.UpdatedAt,
		})
	}
	return items, nil
}

func (s *tenantNoteService) Create(ctx context.Context, content string) (types.TenantNote, error) {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return types.TenantNote{}, err
	}
	if err := validateNoteContent(content); err != nil {
		return types.TenantNote{}, err
	}
	note := types.TenantNote{
		TenantID: tenantID,
		UserID:   userID,
		Content:  content,
	}
	if err := s.repo.CreateWithLimit(ctx, tenantID, userID, &note); err != nil {
		return types.TenantNote{}, err
	}
	logger.Infof(ctx, "[notes] created note %s for user %s tenant %d", note.ID, userID, tenantID)
	return note, nil
}

func (s *tenantNoteService) Get(ctx context.Context, noteID string) (*types.TenantNote, error) {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByID(ctx, tenantID, userID, noteID)
}

func (s *tenantNoteService) Update(ctx context.Context, noteID string, content string) error {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return err
	}
	if err := validateNoteContent(content); err != nil {
		return err
	}
	return s.repo.UpdateContent(ctx, tenantID, userID, noteID, content)
}

func (s *tenantNoteService) Delete(ctx context.Context, noteID string) error {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return err
	}
	note, err := s.repo.GetByID(ctx, tenantID, userID, noteID)
	if err != nil {
		return err
	}
	if err := s.repo.Delete(ctx, tenantID, userID, noteID); err != nil {
		return err
	}
	// ticket 07：图片 GC —— 回收仅被被删笔记引用的图片，即时释放配额。
	// Best-effort：GC 失败不影响笔记删除本身。
	if ids := types.ExtractNoteImageIDs(note.Content); len(ids) > 0 {
		if gcErr := s.repo.DeleteUnreferencedImages(ctx, tenantID, userID, noteID, ids); gcErr != nil {
			logger.Warnf(ctx, "[notes] image GC after delete failed: note=%s err=%v", noteID, gcErr)
		}
	}
	return nil
}

// CreateImage validates an uploaded note image (magic-byte sniffed type,
// ≤2MB, ≤200/user) and stores it. The multipart Content-Type header is
// ignored — sniffing wins (notes ticket 07).
func (s *tenantNoteService) CreateImage(ctx context.Context, data []byte) (types.TenantNoteImage, error) {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return types.TenantNoteImage{}, err
	}
	if len(data) > types.MaxNoteImageBytes {
		return types.TenantNoteImage{}, types.ErrNoteImageTooLarge
	}
	mime := types.DetectNoteImageMime(data)
	if mime == "" {
		return types.TenantNoteImage{}, types.ErrNoteImageTypeUnsupport
	}
	image := types.TenantNoteImage{
		TenantID: tenantID,
		UserID:   userID,
		Mime:     mime,
		Bytes:    data,
	}
	if err := s.repo.CreateImage(ctx, tenantID, userID, &image); err != nil {
		return types.TenantNoteImage{}, err
	}
	logger.Infof(ctx, "[notes] stored image %s (%d bytes, %s) for user %s", image.ID, len(data), mime, userID)
	return image, nil
}

func (s *tenantNoteService) GetImage(ctx context.Context, imageID string) (*types.TenantNoteImage, error) {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	return s.repo.GetImageByID(ctx, tenantID, userID, imageID)
}

func (s *tenantNoteService) DeleteImage(ctx context.Context, imageID string) error {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return err
	}
	return s.repo.DeleteImage(ctx, tenantID, userID, imageID)
}

// validateNoteContent enforces the per-note size limit (UTF-8 bytes).
func validateNoteContent(content string) error {
	if len(content) > types.MaxNoteContentBytes {
		return types.ErrNoteTooLarge
	}
	return nil
}

// deriveNoteTitle implements notes design N-6: the first `#` heading wins,
// else the first non-empty line; both truncated to 50 runes. Empty content
// yields an empty title (the frontend renders "无标题"). The input is a
// bounded content head from the repository, never the full content.
func deriveNoteTitle(contentHead string) string {
	for _, line := range strings.Split(contentHead, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		// A heading line may carry leading hashes; strip them for display.
		// Bare "###" separators keep scanning for a line with actual text.
		stripped := strings.TrimSpace(strings.TrimLeft(trimmed, "#"))
		if stripped == "" {
			continue
		}
		return truncateNoteTitle(stripped, types.NoteTitleMaxRunes)
	}
	return ""
}

func truncateNoteTitle(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
