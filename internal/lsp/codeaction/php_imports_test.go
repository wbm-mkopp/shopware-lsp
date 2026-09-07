package codeaction

import (
	"context"
	"testing"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPHPOrganizeImportsIsVersionedAndPreservesSuppressedImports(t *testing.T) {
	index, err := php.NewPHPIndex(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })
	source := "<?php\n/** @noinspection php.unusedImport */\nuse Vendor\\Keep;\nuse Vendor\\Unused;\n"
	document := lsp.NewTextDocument("file:///workspace/live.php", source, 8)
	actions := NewPHPImportsProvider(index).GetCodeActions(context.Background(), &lsp.CodeActionRequest{SyntaxContext: lsp.SyntaxContext{Document: document}})
	require.Len(t, actions, 1)
	assert.Equal(t, "Organize Imports", actions[0].Title)
	assert.Equal(t, protocol.CodeActionSourceOrganizeImports, actions[0].Kind)
	require.NotNil(t, actions[0].Edit)
	encoded := actions[0].Edit
	require.NotEmpty(t, encoded.DocumentChanges)
	// Workspace edits must carry the current version, so clients can reject stale plans.
	assert.Empty(t, encoded.Changes)
	require.NotNil(t, encoded.DocumentChanges[0].TextDocument.Version)
	assert.EqualValues(t, 8, *encoded.DocumentChanges[0].TextDocument.Version)
	for _, source := range []string{"<?php use Vendor\\Unused; function broken(", "<?php use Vendor\\Used; new Used;"} {
		document := lsp.NewTextDocument("file:///workspace/live.php", source, 9)
		actions := NewPHPImportsProvider(index).GetCodeActions(context.Background(), &lsp.CodeActionRequest{SyntaxContext: lsp.SyntaxContext{Document: document}})
		assert.Empty(t, actions)
	}
}
