package interfaces

import (
	"context"

	"github.com/Tencent/WeKnora/internal/types"
)

// TenantPersonalModelRepository persists 学生个人模型 rows, always scoped by
// (tenantID, userID) from the auth context.
type TenantPersonalModelRepository interface {
	Create(ctx context.Context, model *types.TenantPersonalModel) error
	List(ctx context.Context, tenantID uint64, userID string) ([]types.TenantPersonalModel, error)
	GetByID(ctx context.Context, tenantID uint64, userID, id string) (*types.TenantPersonalModel, error)
	Update(ctx context.Context, tenantID uint64, userID string, model *types.TenantPersonalModel) error
	Delete(ctx context.Context, tenantID uint64, userID, id string) error
	// DeleteAllForUser removes every personal model for a member leaving the workspace.
	DeleteAllForUser(ctx context.Context, tenantID uint64, userID string) error
	CountByUser(ctx context.Context, tenantID uint64, userID string) (int64, error)
	// ListByTenant returns all personal models in a workspace (admin metadata; no keys).
	ListByTenant(ctx context.Context, tenantID uint64) ([]types.TenantPersonalModel, error)
}

// TenantPersonalModelService is the /me/personal-models surface.
type TenantPersonalModelService interface {
	List(ctx context.Context) ([]types.TenantPersonalModelListItem, error)
	Create(ctx context.Context, in TenantPersonalModelInput) (types.TenantPersonalModelListItem, error)
	Get(ctx context.Context, id string) (types.TenantPersonalModelListItem, error)
	Update(ctx context.Context, id string, in TenantPersonalModelInput) (types.TenantPersonalModelListItem, error)
	Delete(ctx context.Context, id string) error
	// ResolveForChat returns decrypted credentials when the caller owns id
	// and the workspace switch is on; used by chat override (#40).
	ResolveForChat(ctx context.Context, id string) (*ResolvedPersonalModel, error)
	DeleteAllForUser(ctx context.Context, tenantID uint64, userID string) error
	// ListMetaForTenant returns workspace-wide metadata for 空间负责人 (no credentials).
	ListMetaForTenant(ctx context.Context) ([]types.TenantPersonalModelListItem, error)
}

// TenantPersonalModelInput is the write payload (API key plaintext in, never out).
type TenantPersonalModelInput struct {
	Name      string
	ModelName string
	BaseURL   string
	Provider  string
	APIKey    string
	Enabled   *bool
}

// ResolvedPersonalModel is the runtime chat override payload.
type ResolvedPersonalModel struct {
	ID        string
	ModelName string
	BaseURL   string
	Provider  string
	APIKey    string
}
