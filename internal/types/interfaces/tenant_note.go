package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// TenantNoteRepository persists per-user Markdown notes. Every method is
// scoped by (tenantID, userID) taken from the caller's auth context —
// there is deliberately no method that reads or lists across users
// (notes design N-1: teachers cannot see student notes even in principle).
type TenantNoteRepository interface {
	// CreateWithLimit inserts a note inside the same transaction that
	// counts the user's existing notes, enforcing MaxNotesPerUser.
	// Returns types.ErrNoteLimitReached when the user is at the cap.
	CreateWithLimit(ctx context.Context, tenantID uint64, userID string, note *types.TenantNote) error

	// ListHead returns the user's notes ordered by updated_at DESC with a
	// bounded content head (for server-side title derivation) — never the
	// full content.
	ListHead(ctx context.Context, tenantID uint64, userID string) ([]*types.TenantNote, error)

	// GetByID returns the full note; gorm.ErrRecordNotFound when the note
	// does not exist for this (tenant, user) pair — a foreign note id is
	// indistinguishable from a missing one (no existence leak).
	GetByID(ctx context.Context, tenantID uint64, userID, noteID string) (*types.TenantNote, error)

	// UpdateContent overwrites the content of an owned note. Returns
	// gorm.ErrRecordNotFound when the note does not exist for this pair.
	UpdateContent(ctx context.Context, tenantID uint64, userID, noteID, content string) error

	// Delete removes an owned note. Returns gorm.ErrRecordNotFound when
	// the note does not exist for this pair.
	Delete(ctx context.Context, tenantID uint64, userID, noteID string) error

	// CountByUser returns how many notes the user owns in the tenant
	// (exposed for tests and the image-GC sanity checks).
	CountByUser(ctx context.Context, tenantID uint64, userID string) (int64, error)
}

// TenantNoteService is the business layer over TenantNoteRepository: it
// derives the caller from the auth context, enforces the content size
// limit, and derives list titles server-side (notes design N-6).
type TenantNoteService interface {
	// List returns the caller's notes as (id, title, updated_at) items,
	// newest first. Title comes from the note's first `#` heading, else
	// its first non-empty line truncated to 50 runes; empty notes carry
	// an empty title (the frontend renders "无标题").
	List(ctx context.Context) ([]types.TenantNoteListItem, error)

	// Create stores a new note with the given content. Returns
	// types.ErrNoteTooLarge above 1MB and types.ErrNoteLimitReached at
	// the per-user cap.
	Create(ctx context.Context, content string) (types.TenantNote, error)

	// Get returns one owned note in full.
	Get(ctx context.Context, noteID string) (*types.TenantNote, error)

	// Update overwrites an owned note's content (same size limit as Create).
	Update(ctx context.Context, noteID string, content string) error

	// Delete removes an owned note.
	Delete(ctx context.Context, noteID string) error
}
