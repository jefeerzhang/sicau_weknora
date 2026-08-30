package service

import (
	"context"
	"path/filepath"
	"strings"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

type tenantAnnouncementService struct {
	repo       interfaces.TenantAnnouncementRepository
	fileSvc    interfaces.FileService
	userRepo   interfaces.UserRepository
}

func NewTenantAnnouncementService(
	repo interfaces.TenantAnnouncementRepository,
	fileSvc interfaces.FileService,
	userRepo interfaces.UserRepository,
) interfaces.TenantAnnouncementService {
	return &tenantAnnouncementService{repo: repo, fileSvc: fileSvc, userRepo: userRepo}
}

func (s *tenantAnnouncementService) caller(ctx context.Context) (uint64, string, error) {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		return 0, "", apperrors.NewUnauthorizedError("workspace context is required")
	}
	userID, ok := types.UserIDFromContext(ctx)
	if !ok || userID == "" {
		return 0, "", apperrors.NewUnauthorizedError("user context is required")
	}
	return tenantID, userID, nil
}

// callerRole resolves the caller's tenant role for author-or-admin checks.
// Missing membership is an error, not a default.
func (s *tenantAnnouncementService) callerRole(ctx context.Context, tenantID uint64, userID string) (types.TenantRole, error) {
	role := types.TenantRoleFromContext(ctx)
	if role != "" {
		return role, nil
	}
	return "", apperrors.NewUnauthorizedError("role context is required")
}

func (s *tenantAnnouncementService) canDelete(role types.TenantRole, authorID, callerID string) bool {
	return role == types.TenantRoleAdmin || role == types.TenantRoleOwner || authorID == callerID
}

// hydrateAuthors fills AuthorName for a mixed batch of announcements.
func (s *tenantAnnouncementService) hydrateAuthors(ctx context.Context, userIDs []string, setName func(id string, name string)) {
	unique := make([]string, 0, len(userIDs))
	seen := make(map[string]bool, len(userIDs))
	for _, id := range userIDs {
		if id != "" && !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 || s.userRepo == nil {
		return
	}
	users, err := s.userRepo.GetUsersByIDs(ctx, unique)
	if err != nil {
		logger.Warnf(ctx, "[announcements] author hydrate failed: %v", err)
		return
	}
	for _, id := range unique {
		if u := users[id]; u != nil {
			setName(id, u.Username)
		}
	}
}

func (s *tenantAnnouncementService) List(ctx context.Context) ([]*types.Announcement, error) {
	tenantID, _, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	announcements, err := s.repo.ListHead(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(announcements))
	for _, a := range announcements {
		ids = append(ids, a.UserID)
	}
	byID := map[string]string{}
	s.hydrateAuthors(ctx, ids, func(id, name string) { byID[id] = name })
	for _, a := range announcements {
		a.AuthorName = byID[a.UserID]
	}
	return announcements, nil
}

// Create stores a notice and uploads its attachments through the
// workspace FileService. ponytail: attachments ride the default
// FileService (no backend-scoped resolution) — course notices don't need
// per-KB storage routing; switch to resolveFileService if that changes.
func (s *tenantAnnouncementService) Create(ctx context.Context, title, content string, files []interfaces.AnnouncementUploadFile) (*types.Announcement, error) {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	role, err := s.callerRole(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	if role != types.TenantRoleAdmin && role != types.TenantRoleOwner && role != types.TenantRoleContributor {
		return nil, apperrors.NewForbiddenError("only teachers can post announcements")
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return nil, apperrors.NewValidationError("title is required")
	}
	if len([]rune(title)) > types.MaxAnnouncementTitleRunes {
		return nil, apperrors.NewValidationError("title too long")
	}
	if len(content) > types.MaxAnnouncementContentBytes {
		return nil, apperrors.NewValidationError("content too large (max 1MB)")
	}
	if len(files) > types.MaxAnnouncementAttachments {
		return nil, apperrors.NewValidationError("too many attachments (max 5)")
	}

	announced := &types.Announcement{
		TenantID:  tenantID,
		UserID:    userID,
		Title:     title,
		Content:   content,
		Attachments: []types.AnnouncementAttachment{},
	}

	for _, file := range files {
		if file.Name == "" {
			file.Name = "attachment"
		}
		ext := strings.ToLower(filepath.Ext(file.Name))
		if !types.AnnouncementAttachmentExtensions[ext] {
			return nil, apperrors.NewValidationError("file type not allowed: " + ext)
		}
		if len(file.Data) > types.MaxAnnouncementFileBytes {
			return nil, apperrors.NewValidationError("file too large (max 50MB): " + file.Name)
		}
		// knowledgeID slot carries the announcement namespace segment.
		path, err := s.fileSvc.SaveBytes(ctx, file.Data, tenantID, "announcements/"+file.Name, false)
		if err != nil {
			logger.Errorf(ctx, "[announcements] SaveBytes failed: %v", err)
			return nil, apperrors.NewInternalServerError("failed to store attachment").WithDetails(err.Error())
		}
		announced.Attachments = append(announced.Attachments, types.AnnouncementAttachment{
			Name: filepath.Base(file.Name),
			Path: path,
			Size: int64(len(file.Data)),
		})
	}

	if err := s.repo.Create(ctx, announced); err != nil {
		// Roll back already-stored files so a failed insert leaves no orphans.
		for _, att := range announced.Attachments {
			if delErr := s.fileSvc.DeleteFile(ctx, att.Path); delErr != nil {
				logger.Warnf(ctx, "[announcements] orphan attachment after failed create: path=%s err=%v", att.Path, delErr)
			}
		}
		return nil, err
	}
	announced.AuthorName = ""
	logger.Infof(ctx, "[announcements] created %s by %s with %d attachments", announced.ID, userID, len(announced.Attachments))
	return announced, nil
}

func (s *tenantAnnouncementService) Get(ctx context.Context, announcementID string) (*types.Announcement, error) {
	tenantID, _, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	announcement, err := s.repo.GetByID(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	s.hydrateAuthors(ctx, []string{announcement.UserID}, func(id, name string) { announcement.AuthorName = name })
	return announcement, nil
}

// Delete removes the announcement (author or admin), its stored files,
// and its comments. File deletion is best-effort after the row is gone —
// a failed DeleteFile leaves an orphaned blob but the API still succeeds,
// matching the "row is the source of truth" pattern used elsewhere.
func (s *tenantAnnouncementService) Delete(ctx context.Context, announcementID string) error {
	tenantID, callerID, err := s.caller(ctx)
	if err != nil {
		return err
	}
	role, err := s.callerRole(ctx, tenantID, callerID)
	if err != nil {
		return err
	}
	announcement, err := s.repo.GetByID(ctx, tenantID, announcementID)
	if err != nil {
		return err
	}
	if !s.canDelete(role, announcement.UserID, callerID) {
		return apperrors.NewForbiddenError("only the author or an admin can delete this announcement")
	}
	if err := s.repo.Delete(ctx, tenantID, announcementID); err != nil {
		return err
	}
	for _, att := range announcement.Attachments {
		if delErr := s.fileSvc.DeleteFile(ctx, att.Path); delErr != nil {
			logger.Warnf(ctx, "[announcements] attachment delete failed: path=%s err=%v", att.Path, delErr)
		}
	}
	logger.Infof(ctx, "[announcements] deleted %s by %s", announcementID, callerID)
	return nil
}

func (s *tenantAnnouncementService) ListComments(ctx context.Context, announcementID string) ([]types.AnnouncementCommentItem, error) {
	tenantID, _, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	comments, err := s.repo.ListComments(ctx, tenantID, announcementID)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(comments))
	for _, c := range comments {
		ids = append(ids, c.UserID)
	}
	byID := map[string]string{}
	s.hydrateAuthors(ctx, ids, func(id, name string) { byID[id] = name })
	items := make([]types.AnnouncementCommentItem, 0, len(comments))
	for _, c := range comments {
		items = append(items, types.AnnouncementCommentItem{
			ID:         c.ID,
			UserID:     c.UserID,
			AuthorName: byID[c.UserID],
			Content:    c.Content,
			CreatedAt:  c.CreatedAt,
		})
	}
	return items, nil
}

func (s *tenantAnnouncementService) CreateComment(ctx context.Context, announcementID, content string) (types.AnnouncementCommentItem, error) {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return types.AnnouncementCommentItem{}, err
	}
	content = strings.TrimSpace(content)
	if content == "" {
		return types.AnnouncementCommentItem{}, apperrors.NewValidationError("comment cannot be empty")
	}
	if _, err := s.repo.GetByID(ctx, tenantID, announcementID); err != nil {
		return types.AnnouncementCommentItem{}, err
	}
	comment := &types.AnnouncementComment{
		TenantID:       tenantID,
		AnnouncementID: announcementID,
		UserID:         userID,
		Content:        content,
	}
	if err := s.repo.CreateComment(ctx, comment); err != nil {
		return types.AnnouncementCommentItem{}, err
	}
	authorName := ""
	s.hydrateAuthors(ctx, []string{userID}, func(id, name string) { authorName = name })
	return types.AnnouncementCommentItem{
		ID:         comment.ID,
		UserID:     userID,
		AuthorName: authorName,
		Content:    comment.Content,
		CreatedAt:  comment.CreatedAt,
	}, nil
}

// DeleteComment removes a comment: its author, or a workspace admin/owner.
func (s *tenantAnnouncementService) DeleteComment(ctx context.Context, announcementID, commentID string) error {
	tenantID, callerID, err := s.caller(ctx)
	if err != nil {
		return err
	}
	role, err := s.callerRole(ctx, tenantID, callerID)
	if err != nil {
		return err
	}
	comment, err := s.repo.GetComment(ctx, tenantID, announcementID, commentID)
	if err != nil {
		return err
	}
	if comment.UserID != callerID && role != types.TenantRoleAdmin && role != types.TenantRoleOwner {
		return apperrors.NewForbiddenError("only the comment author or an admin can delete this comment")
	}
	return s.repo.DeleteComment(ctx, tenantID, announcementID, commentID)
}

// keep gorm imported for the gorm.ErrRecordNotFound mapping in handlers.
var _ = gorm.ErrRecordNotFound
