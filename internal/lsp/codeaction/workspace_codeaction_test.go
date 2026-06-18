package codeaction

import (
	"context"
	"testing"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspaceCodeActionProvider_ForceReindex(t *testing.T) {
	provider := NewWorkspaceCodeActionProvider()

	actions := provider.GetCodeActions(context.Background(), &protocol.CodeActionParams{
		Context: protocol.CodeActionContext{},
	})
	require.Len(t, actions, 1)
	assert.Equal(t, "Shopware: Force Reindex", actions[0].Title)
	require.NotNil(t, actions[0].Command)
	assert.Equal(t, ForceReindexCommand, actions[0].Command.Command)
}

func TestWorkspaceCodeActionProvider_RespectsOnlyFilter(t *testing.T) {
	provider := NewWorkspaceCodeActionProvider()

	actions := provider.GetCodeActions(context.Background(), &protocol.CodeActionParams{
		Context: protocol.CodeActionContext{
			Only: []string{"quickfix"},
		},
	})
	assert.Empty(t, actions)

	actions = provider.GetCodeActions(context.Background(), &protocol.CodeActionParams{
		Context: protocol.CodeActionContext{
			Only: []string{"source"},
		},
	})
	require.Len(t, actions, 1)
}
