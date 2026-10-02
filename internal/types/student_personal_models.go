package types

import (
	"database/sql/driver"
	"encoding/json"
	"net/url"
	"strings"
	"time"

	"gorm.io/gorm"
)

// StudentPersonalModelsConfig is the workspace switch and teacher-appended
// outbound host suffixes for 学生个人模型 (ADR-0001). Default is off so
// existing courses keep zero-config behaviour until a 空间负责人 opts in.
type StudentPersonalModelsConfig struct {
	Enabled      bool     `json:"enabled"`
	AllowedHosts []string `json:"allowed_hosts,omitempty"`
}

// Normalize trims and lowercases host suffixes; nil stays "off".
func (c *StudentPersonalModelsConfig) Normalize() {
	if c == nil {
		return
	}
	out := make([]string, 0, len(c.AllowedHosts))
	seen := map[string]struct{}{}
	for _, h := range c.AllowedHosts {
		h = strings.ToLower(strings.TrimSpace(h))
		h = strings.TrimPrefix(h, "*.")
		if h == "" {
			continue
		}
		if _, ok := seen[h]; ok {
			continue
		}
		seen[h] = struct{}{}
		out = append(out, h)
	}
	c.AllowedHosts = out
}

// EffectiveStudentPersonalModels returns a non-nil config; missing means disabled.
func EffectiveStudentPersonalModels(c *StudentPersonalModelsConfig) StudentPersonalModelsConfig {
	if c == nil {
		return StudentPersonalModelsConfig{Enabled: false}
	}
	cp := *c
	cp.Normalize()
	return cp
}

// Value implements driver.Valuer for jsonb.
func (c StudentPersonalModelsConfig) Value() (driver.Value, error) {
	c.Normalize()
	return json.Marshal(c)
}

// Scan implements sql.Scanner for jsonb.
func (c *StudentPersonalModelsConfig) Scan(value interface{}) error {
	if value == nil {
		*c = StudentPersonalModelsConfig{}
		return nil
	}
	b, ok := value.([]byte)
	if !ok {
		return nil
	}
	if len(b) == 0 {
		*c = StudentPersonalModelsConfig{}
		return nil
	}
	return json.Unmarshal(b, c)
}

// PresetStudentModelHostSuffixes are platform-default vendor host fragments
// students may use for 学生个人模型 before any teacher append. Kept as a
// stable list (not every runtime vendor) so classroom allowlisting is
// predictable and reviewable.
func PresetStudentModelHostSuffixes() []string {
	return []string{
		"api.openai.com",
		"openai.azure.com",
		"api.anthropic.com",
		"generativelanguage.googleapis.com",
		"siliconflow.cn",
		"api.deepseek.com",
		"dashscope.aliyuncs.com",
		"open.bigmodel.cn",
		"volces.com",
		"minimaxi.com",
		"minimax.io",
		"moonshot.cn",
		"moonshot.ai",
		"openrouter.ai",
		"api.novita.ai",
	}
}

// StudentPersonalModelHostAllowed reports whether host (or URL) is in the
// union of platform presets and teacher-appended suffixes.
func StudentPersonalModelHostAllowed(rawURL string, teacherHosts []string) bool {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || u.Hostname() == "" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	candidates := append([]string{}, PresetStudentModelHostSuffixes()...)
	for _, h := range teacherHosts {
		h = strings.ToLower(strings.TrimSpace(h))
		h = strings.TrimPrefix(h, "*.")
		if h != "" {
			candidates = append(candidates, h)
		}
	}
	for _, suf := range candidates {
		if host == suf || strings.HasSuffix(host, "."+suf) {
			return true
		}
	}
	return false
}

// TenantPersonalModel is one 学生个人模型 row: owner-scoped chat model
// credentials for a single (tenant, user).
type TenantPersonalModel struct {
	ID           string         `json:"id" gorm:"type:varchar(36);primaryKey"`
	TenantID     uint64         `json:"tenant_id" gorm:"index:idx_tenant_personal_models_user,priority:1;not null"`
	UserID       string         `json:"user_id" gorm:"type:varchar(512);index:idx_tenant_personal_models_user,priority:2;not null"`
	Name         string         `json:"name" gorm:"type:varchar(255);not null;default:''"`
	ModelName    string         `json:"model_name" gorm:"type:varchar(255);not null"`
	BaseURL      string         `json:"base_url" gorm:"type:varchar(1024);not null"`
	Provider     string         `json:"provider" gorm:"type:varchar(64);default:''"`
	APIKeyEnc    string         `json:"-" gorm:"column:api_key_enc;type:text;not null;default:''"`
	Enabled      bool           `json:"enabled" gorm:"default:true"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `json:"deleted_at,omitempty" gorm:"index"`
}

func (TenantPersonalModel) TableName() string { return "tenant_personal_models" }

// TenantPersonalModelListItem is the redacted list/detail projection.
type TenantPersonalModelListItem struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id,omitempty"`
	Name          string    `json:"name"`
	ModelName     string    `json:"model_name"`
	BaseURL       string    `json:"base_url"`
	Provider      string    `json:"provider"`
	Enabled       bool      `json:"enabled"`
	HasCredential bool      `json:"has_credential"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ToListItem hides credentials.
func (m TenantPersonalModel) ToListItem() TenantPersonalModelListItem {
	return TenantPersonalModelListItem{
		ID:            m.ID,
		UserID:        m.UserID,
		Name:          m.Name,
		ModelName:     m.ModelName,
		BaseURL:       m.BaseURL,
		Provider:      m.Provider,
		Enabled:       m.Enabled,
		HasCredential: strings.TrimSpace(m.APIKeyEnc) != "",
		CreatedAt:     m.CreatedAt,
		UpdatedAt:     m.UpdatedAt,
	}
}
