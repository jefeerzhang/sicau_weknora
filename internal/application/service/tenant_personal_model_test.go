package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

type memPersonalModelRepo struct {
	rows map[string]*types.TenantPersonalModel
}

func newMemPersonalModelRepo() *memPersonalModelRepo {
	return &memPersonalModelRepo{rows: map[string]*types.TenantPersonalModel{}}
}

func (r *memPersonalModelRepo) Create(_ context.Context, model *types.TenantPersonalModel) error {
	cp := *model
	r.rows[model.ID] = &cp
	return nil
}

func (r *memPersonalModelRepo) List(_ context.Context, tenantID uint64, userID string) ([]types.TenantPersonalModel, error) {
	out := make([]types.TenantPersonalModel, 0)
	for _, row := range r.rows {
		if row.TenantID == tenantID && row.UserID == userID {
			out = append(out, *row)
		}
	}
	return out, nil
}

func (r *memPersonalModelRepo) GetByID(_ context.Context, tenantID uint64, userID, id string) (*types.TenantPersonalModel, error) {
	row, ok := r.rows[id]
	if !ok || row.TenantID != tenantID || row.UserID != userID {
		return nil, gorm.ErrRecordNotFound
	}
	cp := *row
	return &cp, nil
}

func (r *memPersonalModelRepo) Update(_ context.Context, tenantID uint64, userID string, model *types.TenantPersonalModel) error {
	cur, ok := r.rows[model.ID]
	if !ok || cur.TenantID != tenantID || cur.UserID != userID {
		return gorm.ErrRecordNotFound
	}
	cp := *model
	r.rows[model.ID] = &cp
	return nil
}

func (r *memPersonalModelRepo) Delete(_ context.Context, tenantID uint64, userID, id string) error {
	row, ok := r.rows[id]
	if !ok || row.TenantID != tenantID || row.UserID != userID {
		return gorm.ErrRecordNotFound
	}
	delete(r.rows, id)
	return nil
}

func (r *memPersonalModelRepo) DeleteAllForUser(_ context.Context, tenantID uint64, userID string) error {
	for id, row := range r.rows {
		if row.TenantID == tenantID && row.UserID == userID {
			delete(r.rows, id)
		}
	}
	return nil
}

func (r *memPersonalModelRepo) CountByUser(ctx context.Context, tenantID uint64, userID string) (int64, error) {
	rows, err := r.List(ctx, tenantID, userID)
	return int64(len(rows)), err
}

func (r *memPersonalModelRepo) ListByTenant(_ context.Context, tenantID uint64) ([]types.TenantPersonalModel, error) {
	out := make([]types.TenantPersonalModel, 0)
	for _, row := range r.rows {
		if row.TenantID == tenantID {
			out = append(out, *row)
		}
	}
	return out, nil
}

func personalModelCtx(enabled bool, hosts ...string) context.Context {
	ctx := context.Background()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u-student")
	cfg := &types.StudentPersonalModelsConfig{Enabled: enabled, AllowedHosts: hosts}
	ctx = context.WithValue(ctx, types.TenantInfoContextKey, &types.Tenant{ID: 7, StudentPersonalModels: cfg})
	return ctx
}

func TestPersonalModelCreate_Disabled(t *testing.T) {
	svc := NewTenantPersonalModelService(newMemPersonalModelRepo())
	_, err := svc.Create(personalModelCtx(false), interfaces.TenantPersonalModelInput{
		ModelName: "m", BaseURL: "https://api.siliconflow.cn/v1", APIKey: "sk",
	})
	if !errors.Is(err, ErrPersonalModelsDisabled) {
		t.Fatalf("want disabled, got %v", err)
	}
}

func TestPersonalModelCreate_HostRejected(t *testing.T) {
	svc := NewTenantPersonalModelService(newMemPersonalModelRepo())
	_, err := svc.Create(personalModelCtx(true), interfaces.TenantPersonalModelInput{
		ModelName: "m", BaseURL: "https://evil.example/v1", APIKey: "sk",
	})
	if !errors.Is(err, ErrPersonalModelHost) {
		t.Fatalf("want host err, got %v", err)
	}
}

func TestPersonalModelCreate_TeacherHostAllowed(t *testing.T) {
	svc := NewTenantPersonalModelService(newMemPersonalModelRepo())
	item, err := svc.Create(personalModelCtx(true, "llm.sicau.edu.cn"), interfaces.TenantPersonalModelInput{
		ModelName: "course-llm", BaseURL: "https://llm.sicau.edu.cn/v1", APIKey: "sk-secret",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if item.ID == "" || !item.HasCredential {
		t.Fatalf("bad item: %+v", item)
	}
	if strings.Contains(item.BaseURL, "sk-secret") {
		t.Fatal("key leaked into base_url field")
	}
}

func TestPersonalModelCreate_NoKeyInListItem(t *testing.T) {
	repo := newMemPersonalModelRepo()
	svc := NewTenantPersonalModelService(repo)
	item, err := svc.Create(personalModelCtx(true), interfaces.TenantPersonalModelInput{
		Name: "sf", ModelName: "MiniMax-M3", BaseURL: "https://api.siliconflow.cn/v1", APIKey: "sk-secret",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	stored := repo.rows[item.ID]
	if stored == nil || !strings.HasPrefix(stored.APIKeyEnc, "plain:") {
		t.Fatalf("expected plain: ciphertext in test, got %#v", stored)
	}
	if strings.Contains(item.Name, "sk-secret") || item.HasCredential != true {
		t.Fatalf("list item wrong: %+v", item)
	}
}

func TestPersonalModelResolveForChat(t *testing.T) {
	svc := NewTenantPersonalModelService(newMemPersonalModelRepo())
	item, err := svc.Create(personalModelCtx(true), interfaces.TenantPersonalModelInput{
		ModelName: "m", BaseURL: "https://api.deepseek.com/v1", APIKey: "sk-live",
	})
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := svc.ResolveForChat(personalModelCtx(true), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.APIKey != "sk-live" || resolved.ModelName != "m" {
		t.Fatalf("resolved: %+v", resolved)
	}
}

func TestPersonalModelDeleteAllForUser(t *testing.T) {
	repo := newMemPersonalModelRepo()
	svc := NewTenantPersonalModelService(repo)
	ctx := personalModelCtx(true)
	a, _ := svc.Create(ctx, interfaces.TenantPersonalModelInput{
		ModelName: "a", BaseURL: "https://api.openai.com/v1", APIKey: "k1",
	})
	_ = a
	// Other user row should survive.
	repo.rows["other"] = &types.TenantPersonalModel{
		ID: "other", TenantID: 7, UserID: "u-other", ModelName: "x",
		BaseURL: "https://api.openai.com/v1", APIKeyEnc: "plain:k", Enabled: true,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := svc.DeleteAllForUser(ctx, 7, "u-student"); err != nil {
		t.Fatal(err)
	}
	if len(repo.rows) != 1 || repo.rows["other"] == nil {
		t.Fatalf("expected only other user left, got %#v", repo.rows)
	}
}
