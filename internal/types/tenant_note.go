package types

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// sicau-v1 notes limits (notes design N-7). Plain constants on purpose —
// a full quota system is explicitly out of scope for v1.
const (
	MaxNotesPerUser        = 200
	MaxNoteContentBytes    = 1 << 20 // 1MB per note (UTF-8 bytes)
	MaxNoteImagesPerUser   = 200
	MaxNoteImageBytes      = 2 << 20 // 2MB per image
	NoteTitleMaxRunes      = 50
	NotePreviewContentHead = 4096 // bytes of content fetched for title derivation
)

// Sentinel errors mapped by handlers: both limits surface as 400.
var (
	ErrNoteLimitReached = errors.New("note limit reached (200 notes per user)")
	ErrNoteTooLarge     = errors.New("note content too large (max 1MB)")
)

// TenantNote is one user's private Markdown note inside a workspace.
// Isolation is structural: every read/write filters on (tenant_id, user_id)
// taken from the auth context, and no endpoint lists notes across users —
// teachers cannot see student notes even in principle (framework ADR-012,
// notes design N-1).
type TenantNote struct {
	ID        string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	TenantID  uint64    `gorm:"not null" json:"tenant_id"`
	UserID    string    `gorm:"type:varchar(512);not null" json:"user_id"`
	Content   string    `gorm:"type:text" json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName pins the table so GORM's pluralizer cannot drift.
func (TenantNote) TableName() string { return "tenant_notes" }

// BeforeCreate mints the note id.
func (n *TenantNote) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	return nil
}

// TenantNoteListItem is the list-endpoint projection: id, server-derived
// title (notes design N-6) and timestamps — never the content.
type TenantNoteListItem struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TenantNoteImage is a paste/screenshot upload owned by one user.
// ≤2MB each, ≤200 per user (notes design N-5/N-7).
type TenantNoteImage struct {
	ID        string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	TenantID  uint64    `gorm:"not null" json:"tenant_id"`
	UserID    string    `gorm:"type:varchar(512);not null" json:"user_id"`
	Mime      string    `gorm:"type:varchar(100);not null" json:"mime"`
	Bytes     []byte    `gorm:"not null" json:"-"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName pins the table so GORM's pluralizer cannot drift.
func (TenantNoteImage) TableName() string { return "tenant_note_images" }
