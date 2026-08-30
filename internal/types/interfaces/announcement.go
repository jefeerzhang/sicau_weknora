package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// TenantAnnouncementRepository persists course announcements and their
// flat comments. Announcements are workspace-wide reads (every member can
// see every notice); writes are gated upstream (Contributor+ for posts,
// author-or-admin for deletes).
type TenantAnnouncementRepository interface {
	// Create inserts an announcement with its attachment metadata.
	Create(ctx context.Context, announcement *types.Announcement) error

	// ListHead returns announcements ordered by created_at DESC with a
	// bounded content head (list cards show title + author only).
	ListHead(ctx context.Context, tenantID uint64) ([]*types.Announcement, error)

	// GetByID returns the full announcement.
	GetByID(ctx context.Context, tenantID uint64, announcementID string) (*types.Announcement, error)

	// Delete removes the announcement row. Returns gorm.ErrRecordNotFound
	// when missing.
	Delete(ctx context.Context, tenantID uint64, announcementID string) error

	// ListComments returns an announcement's comments ordered created_at ASC.
	ListComments(ctx context.Context, tenantID uint64, announcementID string) ([]*types.AnnouncementComment, error)

	// CreateComment appends a flat comment.
	CreateComment(ctx context.Context, comment *types.AnnouncementComment) error

	// GetComment returns one comment of one announcement.
	GetComment(ctx context.Context, tenantID uint64, announcementID, commentID string) (*types.AnnouncementComment, error)

	// DeleteComment removes a comment. Returns gorm.ErrRecordNotFound when
	// missing.
	DeleteComment(ctx context.Context, tenantID uint64, announcementID, commentID string) error

	// CountCommentsByAnnouncement returns how many comments reference the
	// announcement (cascade sanity check).
	CountCommentsByAnnouncement(ctx context.Context, tenantID uint64, announcementID string) (int64, error)
}

// TenantAnnouncementService is the business layer: role checks (who may
// post/delete), attachment handling via the workspace FileService,
// validation, and author-name hydration.
type TenantAnnouncementService interface {
	// List returns announcements (bounded head) with author names.
	List(ctx context.Context) ([]*types.Announcement, error)

	// Create stores a notice with attachments uploaded in the same
	// multipart request. Requires Contributor+.
	Create(ctx context.Context, title, content string, files []AnnouncementUploadFile) (*types.Announcement, error)

	// Get returns one announcement in full with author name.
	Get(ctx context.Context, announcementID string) (*types.Announcement, error)

	// Delete removes the announcement (author or admin), its stored
	// attachment files (FileService), and its comments.
	Delete(ctx context.Context, announcementID string) error

	// ListComments returns flat comments with author names.
	ListComments(ctx context.Context, announcementID string) ([]types.AnnouncementCommentItem, error)

	// CreateComment appends the caller's comment.
	CreateComment(ctx context.Context, announcementID, content string) (types.AnnouncementCommentItem, error)

	// DeleteComment removes a comment (author or admin).
	DeleteComment(ctx context.Context, announcementID, commentID string) error
}

// AnnouncementUploadFile is one multipart file handed from the handler to
// the service (the service owns FileService interaction).
type AnnouncementUploadFile struct {
	Name string
	Data []byte
}
