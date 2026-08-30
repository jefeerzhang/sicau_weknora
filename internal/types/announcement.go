package types

import (
	"database/sql/driver"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// sicau-v1 announcement limits (design §3). Constants, validated in the
// handler/service boundary.
const (
	MaxAnnouncementAttachments   = 5
	MaxAnnouncementFileBytes     = 50 << 20 // 50MB per file
	MaxAnnouncementTitleRunes    = 200
	MaxAnnouncementContentBytes  = 1 << 20 // 1MB of notice body text
)

// AnnouncementAttachment allowed extensions (design A-3: generic document
// formats for homework distribution).
var AnnouncementAttachmentExtensions = map[string]bool{
	".pdf": true, ".doc": true, ".docx": true,
	".ppt": true, ".pptx": true,
	".xls": true, ".xlsx": true,
	".zip": true, ".rar": true, ".7z": true,
	".txt": true, ".md": true,
}

// AnnouncementAttachment is one file attached to an announcement. Path is
// the FileService reference (provider://...); the bytes live in the
// workspace's storage backend, not in PG.
type AnnouncementAttachment struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// Announcement is a course notice posted by Contributor+, readable by
// every member of the workspace.
type Announcement struct {
	ID          string                  `gorm:"type:varchar(36);primaryKey" json:"id"`
	TenantID    uint64                  `gorm:"not null" json:"tenant_id"`
	UserID      string                  `gorm:"type:varchar(512);not null" json:"user_id"`
	Title       string                  `gorm:"type:varchar(200);not null" json:"title"`
	Content     string                  `gorm:"type:text" json:"content"`
	Attachments AnnouncementAttachments `gorm:"type:jsonb" json:"attachments"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`

	// Hydrated, not persisted — the author's username for display.
	AuthorName string `gorm:"-" json:"author_name,omitempty"`
}

func (Announcement) TableName() string { return "announcements" }

func (a *Announcement) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	return nil
}

// ScanAttachments unmarshals the jsonb column after a read. Missing/blank
// column yields an empty slice, never nil-of-nothing surprises.
func (a *Announcement) ScanAttachments(raw []byte) error {
	a.Attachments = []AnnouncementAttachment{}
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, &a.Attachments)
}

// AnnouncementComment is a flat, plain-text comment under an announcement.
type AnnouncementComment struct {
	ID             string    `gorm:"type:varchar(36);primaryKey" json:"id"`
	TenantID       uint64    `gorm:"not null" json:"tenant_id"`
	AnnouncementID string    `gorm:"type:varchar(36);not null" json:"announcement_id"`
	UserID         string    `gorm:"type:varchar(512);not null" json:"user_id"`
	Content        string    `gorm:"type:text" json:"content"`
	CreatedAt      time.Time `json:"created_at"`

	// Hydrated, not persisted.
	AuthorName string `gorm:"-" json:"author_name,omitempty"`
}

func (AnnouncementComment) TableName() string { return "announcement_comments" }

func (c *AnnouncementComment) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

// AnnouncementCommentItem is the API projection of a comment with its
// author's display name.
type AnnouncementCommentItem struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	AuthorName string    `json:"author_name"`
	Content    string    `json:"content"`
	CreatedAt  time.Time `json:"created_at"`
}

// Value implements driver.Valuer so the attachments slice serialises into
// the jsonb column (same pattern as AgentConfig in agent.go).
func (a AnnouncementAttachments) Value() (driver.Value, error) {
	return json.Marshal(a)
}

// Scan implements sql.Scanner for the attachments jsonb column.
func (a *AnnouncementAttachments) Scan(value interface{}) error {
	*a = AnnouncementAttachments{}
	if value == nil {
		return nil
	}
	var b []byte
	switch v := value.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return nil
	}
	if len(b) == 0 {
		return nil
	}
	return json.Unmarshal(b, a)
}

// AnnouncementAttachments is the jsonb-backed attachment list.
type AnnouncementAttachments []AnnouncementAttachment
