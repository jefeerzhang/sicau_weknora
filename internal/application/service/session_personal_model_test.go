package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubPersonalModelSvc struct {
	interfaces.TenantPersonalModelService
	resolved *interfaces.ResolvedPersonalModel
	err      error
}

func (s *stubPersonalModelSvc) ResolveForChat(context.Context, string) (*interfaces.ResolvedPersonalModel, error) {
	return s.resolved, s.err
}

func TestAttachPersonalChatModel_BindsContext(t *testing.T) {
	svc := &sessionService{
		personalModelService: &stubPersonalModelSvc{
			resolved: &interfaces.ResolvedPersonalModel{
				ID: "row-1", ModelName: "m", BaseURL: "https://api.deepseek.com/v1", APIKey: "sk",
			},
		},
	}
	req := &types.QARequest{PersonalModelID: "row-1"}
	ctx, err := svc.attachPersonalChatModel(context.Background(), req)
	require.NoError(t, err)
	bound := types.PersonalChatResolvedFromContext(ctx)
	require.NotNil(t, bound)
	assert.Equal(t, "sk", bound.APIKey)
	assert.Equal(t, "pm:row-1", req.SummaryModelID)
}

func TestAttachPersonalChatModel_NoFallbackOnResolveError(t *testing.T) {
	svc := &sessionService{
		personalModelService: &stubPersonalModelSvc{err: ErrPersonalModelsDisabled},
	}
	req := &types.QARequest{SummaryModelID: "pm:row-1"}
	_, err := svc.attachPersonalChatModel(context.Background(), req)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPersonalModelsDisabled) || err != nil)
}

func TestResolveChatModelID_PersonalRefUsesBoundContext(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"agent-model": {ID: "agent-model", Type: types.ModelTypeKnowledgeQA},
			},
		},
	}
	ctx := types.WithPersonalChatResolved(context.Background(), &types.PersonalChatResolved{
		ID: "row-1", ModelName: "m", BaseURL: "https://x", APIKey: "k",
	})
	req := &types.QARequest{
		Session:        &types.Session{},
		SummaryModelID: "pm:row-1",
		CustomAgent: &types.CustomAgent{
			ID: "a1",
			Config: types.CustomAgentConfig{ModelID: "agent-model"},
		},
	}
	id, err := svc.resolveChatModelID(ctx, req, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, "pm:row-1", id)
}

func TestResolveChatModelID_PersonalRefWithoutContextErrors(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			modelsByID: map[string]*types.Model{
				"agent-model": {ID: "agent-model", Type: types.ModelTypeKnowledgeQA},
			},
		},
	}
	req := &types.QARequest{
		Session:        &types.Session{},
		SummaryModelID: "pm:row-1",
		CustomAgent: &types.CustomAgent{
			ID: "a1",
			Config: types.CustomAgentConfig{ModelID: "agent-model"},
		},
	}
	_, err := svc.resolveChatModelID(context.Background(), req, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "personal model")
}
