package service

import (
	"context"
	"testing"

	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSelectChatModelIDSkipsBuiltinFallback(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			models: []*types.Model{
				{ID: "builtin-chat", Type: types.ModelTypeKnowledgeQA, IsBuiltin: true},
			},
		},
	}

	modelID, err := svc.selectChatModelID(context.Background(), &types.Session{}, nil, nil)

	require.Error(t, err)
	assert.Empty(t, modelID)
	appErr, ok := apperrors.IsAppError(err)
	require.True(t, ok)
	assert.Equal(t, apperrors.ErrModelSetupRequired, appErr.Code)
}

func TestSelectChatModelIDUsesTenantOwnedModel(t *testing.T) {
	svc := &sessionService{
		modelService: &stubModelService{
			models: []*types.Model{
				{ID: "builtin-chat", Type: types.ModelTypeKnowledgeQA, IsBuiltin: true},
				{ID: "tenant-chat", Type: types.ModelTypeKnowledgeQA, IsBuiltin: false},
			},
		},
	}

	modelID, err := svc.selectChatModelID(context.Background(), &types.Session{}, nil, nil)

	require.NoError(t, err)
	assert.Equal(t, "tenant-chat", modelID)
}
