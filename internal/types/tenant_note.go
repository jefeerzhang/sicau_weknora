package types

import (
	"errors"
	"strings"
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
	// MaxNoteImageStoragePerUser caps the SUM of a user's image bytes
	// (sicau-v1: user-requested 100MB personal image budget).
	MaxNoteImageStoragePerUser = 100 << 20 // 100MB
	NoteTitleMaxRunes      = 50
	NotePreviewContentHead = 4096 // bytes of content fetched for title derivation
)

// Sentinel errors mapped by handlers: both limits surface as 400.
var (
	ErrNoteLimitReached       = errors.New("note limit reached (200 notes per user)")
	ErrNoteTooLarge           = errors.New("note content too large (max 1MB)")
	ErrNoteImageLimitReached  = errors.New("note image limit reached (200 images per user)")
	ErrNoteImageTooLarge      = errors.New("note image too large (max 2MB)")
	ErrNoteImageTypeUnsupport = errors.New("image type must be png/jpeg/gif/webp")
	ErrNoteImageQuotaExceeded = errors.New("image storage quota exceeded (100MB per user)")
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

// BeforeCreate mints the image id (capability token in the URL).
func (img *TenantNoteImage) BeforeCreate(tx *gorm.DB) error {
	if img.ID == "" {
		img.ID = uuid.New().String()
	}
	return nil
}

// NoteImageURLPrefix is the markdown-facing URL stem for note images.
// Handlers mount the full route under /api/v1/me/notes/images/<id>.
const NoteImageURLPrefix = "/api/v1/me/notes/images/"

// ExtractNoteImageIDs pulls every note-image id referenced by a Markdown
// body. Used by the post-delete GC to reclaim images that only the deleted
// note referenced (ticket 07). UUID chars contain no LIKE wildcards, so the
// repository's LIKE-based reference check is safe.
func ExtractNoteImageIDs(content string) []string {
	const marker = NoteImageURLPrefix
	var ids []string
	for i := strings.Index(content, marker); i >= 0; i = strings.Index(content, marker) {
		rest := content[i+len(marker):]
		end := 0
		for end < len(rest) && isNoteImageIDChar(rest[end]) {
			end++
		}
		if end >= 36 {
			ids = append(ids, rest[:36])
		}
		next := strings.Index(rest, marker)
		if next < 0 {
			break
		}
		content = rest
	}
	return ids
}

func isNoteImageIDChar(b byte) bool {
	return b >= '0' && b <= '9' || b >= 'a' && b <= 'f' || b >= 'A' && b <= 'F' || b == '-'
}

// DetectNoteImageMime sniffs the payload's magic bytes (the multipart
// Content-Type header is client-controlled and untrustworthy, notes
// ticket 07). Returns "" for anything outside the whitelist.
func DetectNoteImageMime(data []byte) string {
	if len(data) < 12 {
		return ""
	}
	switch {
	// PNG
	case data[0] == 0x89 && data[1] == 'P' && data[2] == 'N' && data[3] == 'G':
		return "image/png"
	// JPEG
	case data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg"
	// GIF87a / GIF89a
	case string(data[:6]) == "GIF87a", string(data[:6]) == "GIF89a":
		return "image/gif"
	// WEBP: RIFF....WEBP
	case string(data[0:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp"
	default:
		return ""
	}
}
