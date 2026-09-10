package codeaction

import (
	"context"
	"strings"
	"testing"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPHPExtractMethodUsesOverlayAndInheritedNames(t *testing.T) {
	index, err := php.NewPHPIndex(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })
	source := "<?php\nclass ParentClass { protected function extractedMethod() {} }\nclass Child extends ParentClass { public function run($a) { /* 😀 */ return $a + 1; } }"
	doc := lsp.NewTextDocument("file:///workspace/live.php", source, 9)
	start := uint32(strings.Index(source, "return $a"))
	end := start + uint32(len("return $a + 1;"))
	line, col := doc.LineIndex.PositionUTF16(start)
	endLine, endCol := doc.LineIndex.PositionUTF16(end)
	params := &protocol.CodeActionParams{Range: protocol.Range{Start: protocol.Position{Line: int(line), Character: int(col)}, End: protocol.Position{Line: int(endLine), Character: int(endCol)}}}
	actions := NewPHPExtractMethodProvider(index).GetCodeActions(context.Background(), &lsp.CodeActionRequest{CodeActionParams: params, SyntaxContext: lsp.SyntaxContext{Document: doc}})
	require.Len(t, actions, 1)
	assert.Equal(t, "Extract method 'extractedMethod2'", actions[0].Title)
	require.NotNil(t, actions[0].Edit)
	change := actions[0].Edit.DocumentChanges[0]
	assert.EqualValues(t, 9, *change.TextDocument.Version)
	require.Len(t, change.Edits, 2)
	assert.Equal(t, params.Range, change.Edits[0].Range)
	assert.Equal(t, "return $this->extractedMethod2($a);", change.Edits[0].NewText)
}

func TestPHPExtractMethodRejectsUnknownHierarchyAndIncompleteSyntax(t *testing.T) {
	index, err := php.NewPHPIndex(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })
	for _, source := range []string{
		"<?php class Child extends Missing { public function run($a) { return $a; } }",
		"<?php class Child { public function run($a) { return $a; }",
	} {
		doc := lsp.NewTextDocument("file:///workspace/live.php", source, 1)
		start := strings.Index(source, "return")
		params := &protocol.CodeActionParams{Range: protocol.Range{Start: protocol.Position{Character: start}, End: protocol.Position{Character: start}}}
		actions := NewPHPExtractMethodProvider(index).GetCodeActions(context.Background(), &lsp.CodeActionRequest{CodeActionParams: params, SyntaxContext: lsp.SyntaxContext{Document: doc}})
		assert.Empty(t, actions)
	}
}

func TestPHPExtractFunctionNamespaceAndCollision(t *testing.T) {
	index, err := php.NewPHPIndex(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })
	source := "<?php namespace App; function extractedFunction() {} function run($a) { echo $a; }"
	doc := lsp.NewTextDocument("file:///workspace/live.php", source, 3)
	start := strings.Index(source, "echo")
	params := &protocol.CodeActionParams{Range: protocol.Range{Start: protocol.Position{Character: start}, End: protocol.Position{Character: start}}}
	actions := NewPHPExtractMethodProvider(index).GetCodeActions(context.Background(), &lsp.CodeActionRequest{CodeActionParams: params, SyntaxContext: lsp.SyntaxContext{Document: doc}})
	require.Len(t, actions, 1)
	assert.Equal(t, "Extract function 'extractedFunction2'", actions[0].Title)
	assert.Equal(t, "extractedFunction2($a);", actions[0].Edit.DocumentChanges[0].Edits[0].NewText)
	assert.NotContains(t, actions[0].Edit.DocumentChanges[0].Edits[1].NewText, "private")
}
