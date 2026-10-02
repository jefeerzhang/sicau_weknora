package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	secutils "github.com/Tencent/WeKnora/internal/utils"
	"gorm.io/gorm"
)

const maxPersonalModelsPerUser = 20

var (
	ErrPersonalModelsDisabled = errors.New("student personal models are disabled for this workspace")
	ErrPersonalModelHost      = errors.New("model base_url host is not on the student allowlist")
	ErrPersonalModelLimit     = errors.New("personal model limit reached")
)

type tenantPersonalModelService struct {
	repo interfaces.TenantPersonalModelRepository
}

func NewTenantPersonalModelService(repo interfaces.TenantPersonalModelRepository) interfaces.TenantPersonalModelService {
	return &tenantPersonalModelService{repo: repo}
}

func (s *tenantPersonalModelService) caller(ctx context.Context) (uint64, string, error) {
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

func (s *tenantPersonalModelService) requireEnabled(ctx context.Context) (types.StudentPersonalModelsConfig, error) {
	tenant, _ := types.TenantInfoFromContext(ctx)
	cfg := types.EffectiveStudentPersonalModels(nil)
	if tenant != nil {
		cfg = types.EffectiveStudentPersonalModels(tenant.StudentPersonalModels)
	}
	if !cfg.Enabled {
		return cfg, ErrPersonalModelsDisabled
	}
	return cfg, nil
}

func (s *tenantPersonalModelService) List(ctx context.Context) ([]types.TenantPersonalModelListItem, error) {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	rows, err := s.repo.List(ctx, tenantID, userID)
	if err != nil {
		return nil, err
	}
	out := make([]types.TenantPersonalModelListItem, 0, len(rows))
	for _, row := range rows {
		out = append(out, row.ToListItem())
	}
	return out, nil
}

func (s *tenantPersonalModelService) Create(ctx context.Context, in interfaces.TenantPersonalModelInput) (types.TenantPersonalModelListItem, error) {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return types.TenantPersonalModelListItem{}, err
	}
	cfg, err := s.requireEnabled(ctx)
	if err != nil {
		return types.TenantPersonalModelListItem{}, err
	}
	if err := validatePersonalModelInput(in, true); err != nil {
		return types.TenantPersonalModelListItem{}, err
	}
	// Allowlist before SSRF so classroom rejects are explicit (not DNS noise).
	if !types.StudentPersonalModelHostAllowed(in.BaseURL, cfg.AllowedHosts) {
		return types.TenantPersonalModelListItem{}, ErrPersonalModelHost
	}
	if err := secutils.ValidateURLForSSRF(in.BaseURL); err != nil {
		return types.TenantPersonalModelListItem{}, apperrors.NewBadRequestError(
			secutils.FormatSSRFError("Base URL", in.BaseURL, err))
	}
	n, err := s.repo.CountByUser(ctx, tenantID, userID)
	if err != nil {
		return types.TenantPersonalModelListItem{}, err
	}
	if n >= maxPersonalModelsPerUser {
		return types.TenantPersonalModelListItem{}, ErrPersonalModelLimit
	}
	enc, err := encryptPersonalAPIKey(in.APIKey)
	if err != nil {
		return types.TenantPersonalModelListItem{}, err
	}
	now := time.Now()
	row := &types.TenantPersonalModel{
		ID:        uuid.NewString(),
		TenantID:  tenantID,
		UserID:    userID,
		Name:      strings.TrimSpace(in.Name),
		ModelName: strings.TrimSpace(in.ModelName),
		BaseURL:   strings.TrimSpace(in.BaseURL),
		Provider:  strings.TrimSpace(in.Provider),
		APIKeyEnc: enc,
		Enabled:   true,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if in.Enabled != nil {
		row.Enabled = *in.Enabled
	}
	if row.Name == "" {
		row.Name = row.ModelName
	}
	if err := s.repo.Create(ctx, row); err != nil {
		return types.TenantPersonalModelListItem{}, err
	}
	logger.Infof(ctx, "[personal-models] created %s for user %s tenant %d", row.ID, userID, tenantID)
	return row.ToListItem(), nil
}

func (s *tenantPersonalModelService) Get(ctx context.Context, id string) (types.TenantPersonalModelListItem, error) {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return types.TenantPersonalModelListItem{}, err
	}
	if _, err := s.requireEnabled(ctx); err != nil {
		return types.TenantPersonalModelListItem{}, err
	}
	row, err := s.repo.GetByID(ctx, tenantID, userID, id)
	if err != nil {
		return types.TenantPersonalModelListItem{}, err
	}
	return row.ToListItem(), nil
}

func (s *tenantPersonalModelService) Update(ctx context.Context, id string, in interfaces.TenantPersonalModelInput) (types.TenantPersonalModelListItem, error) {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return types.TenantPersonalModelListItem{}, err
	}
	cfg, err := s.requireEnabled(ctx)
	if err != nil {
		return types.TenantPersonalModelListItem{}, err
	}
	row, err := s.repo.GetByID(ctx, tenantID, userID, id)
	if err != nil {
		return types.TenantPersonalModelListItem{}, err
	}
	// Allow partial updates: empty API key keeps existing ciphertext.
	needKey := strings.TrimSpace(in.APIKey) != ""
	merged := interfaces.TenantPersonalModelInput{
		Name:      firstNonEmpty(in.Name, row.Name),
		ModelName: firstNonEmpty(in.ModelName, row.ModelName),
		BaseURL:   firstNonEmpty(in.BaseURL, row.BaseURL),
		Provider:  firstNonEmpty(in.Provider, row.Provider),
		APIKey:    in.APIKey,
		Enabled:   in.Enabled,
	}
	if err := validatePersonalModelInput(merged, needKey); err != nil {
		return types.TenantPersonalModelListItem{}, err
	}
	if !types.StudentPersonalModelHostAllowed(merged.BaseURL, cfg.AllowedHosts) {
		return types.TenantPersonalModelListItem{}, ErrPersonalModelHost
	}
	if err := secutils.ValidateURLForSSRF(merged.BaseURL); err != nil {
		return types.TenantPersonalModelListItem{}, apperrors.NewBadRequestError(
			secutils.FormatSSRFError("Base URL", merged.BaseURL, err))
	}
	row.Name = strings.TrimSpace(merged.Name)
	row.ModelName = strings.TrimSpace(merged.ModelName)
	row.BaseURL = strings.TrimSpace(merged.BaseURL)
	row.Provider = strings.TrimSpace(merged.Provider)
	if needKey {
		enc, err := encryptPersonalAPIKey(merged.APIKey)
		if err != nil {
			return types.TenantPersonalModelListItem{}, err
		}
		row.APIKeyEnc = enc
	}
	if merged.Enabled != nil {
		row.Enabled = *merged.Enabled
	}
	if err := s.repo.Update(ctx, tenantID, userID, row); err != nil {
		return types.TenantPersonalModelListItem{}, err
	}
	return row.ToListItem(), nil
}

func (s *tenantPersonalModelService) Delete(ctx context.Context, id string) error {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return err
	}
	if _, err := s.requireEnabled(ctx); err != nil {
		return err
	}
	return s.repo.Delete(ctx, tenantID, userID, id)
}

func (s *tenantPersonalModelService) ResolveForChat(ctx context.Context, id string) (*interfaces.ResolvedPersonalModel, error) {
	tenantID, userID, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := s.requireEnabled(ctx); err != nil {
		return nil, err
	}
	row, err := s.repo.GetByID(ctx, tenantID, userID, id)
	if err != nil {
		return nil, err
	}
	if !row.Enabled {
		return nil, apperrors.NewBadRequestError("personal model is disabled")
	}
	key, err := decryptPersonalAPIKey(row.APIKeyEnc)
	if err != nil {
		return nil, err
	}
	return &interfaces.ResolvedPersonalModel{
		ID:        row.ID,
		ModelName: row.ModelName,
		BaseURL:   row.BaseURL,
		Provider:  row.Provider,
		APIKey:    key,
	}, nil
}

func (s *tenantPersonalModelService) DeleteAllForUser(ctx context.Context, tenantID uint64, userID string) error {
	return s.repo.DeleteAllForUser(ctx, tenantID, userID)
}

func (s *tenantPersonalModelService) ListMetaForTenant(ctx context.Context) ([]types.TenantPersonalModelListItem, error) {
	tenantID, ok := types.TenantIDFromContext(ctx)
	if !ok || tenantID == 0 {
		return nil, apperrors.NewUnauthorizedError("workspace context is required")
	}
	rows, err := s.repo.ListByTenant(ctx, tenantID)
	if err != nil {
		return nil, err
	}
	out := make([]types.TenantPersonalModelListItem, 0, len(rows))
	for _, row := range rows {
		item := row.ToListItem()
		// Never expose ciphertext length tricks; has_credential is enough.
		out = append(out, item)
	}
	return out, nil
}

func validatePersonalModelInput(in interfaces.TenantPersonalModelInput, requireKey bool) error {
	if strings.TrimSpace(in.ModelName) == "" {
		return apperrors.NewBadRequestError("model_name is required")
	}
	if strings.TrimSpace(in.BaseURL) == "" {
		return apperrors.NewBadRequestError("base_url is required")
	}
	if requireKey && strings.TrimSpace(in.APIKey) == "" {
		return apperrors.NewBadRequestError("api_key is required")
	}
	if len([]rune(in.ModelName)) > 255 || len([]rune(in.Name)) > 255 {
		return apperrors.NewBadRequestError("name is too long")
	}
	if len(in.BaseURL) > 1024 {
		return apperrors.NewBadRequestError("base_url is too long")
	}
	return nil
}

func encryptPersonalAPIKey(plain string) (string, error) {
	key := secutils.GetAESKey()
	if key == nil {
		// Dev/test without SYSTEM_AES_KEY: store opaque prefix so we never
		// confuse plaintext with ciphertext in prod paths that set the key.
		return "plain:" + plain, nil
	}
	return secutils.EncryptAESGCM(plain, key)
}

func decryptPersonalAPIKey(enc string) (string, error) {
	if strings.HasPrefix(enc, "plain:") {
		return strings.TrimPrefix(enc, "plain:"), nil
	}
	key := secutils.GetAESKey()
	if key == nil {
		return "", apperrors.NewInternalServerError("SYSTEM_AES_KEY is required to use personal models")
	}
	return secutils.DecryptAESGCM(enc, key)
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

// MapPersonalModelError converts service sentinels to HTTP app errors.
func MapPersonalModelError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrPersonalModelsDisabled):
		return apperrors.NewForbiddenError("student personal models are disabled")
	case errors.Is(err, ErrPersonalModelHost):
		return apperrors.NewBadRequestError("base_url host is not allowed for student personal models")
	case errors.Is(err, ErrPersonalModelLimit):
		return apperrors.NewBadRequestError("personal model limit reached")
	case errors.Is(err, gorm.ErrRecordNotFound):
		return apperrors.NewNotFoundError("personal model not found")
	default:
		return err
	}
}
