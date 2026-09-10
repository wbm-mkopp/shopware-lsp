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

func TestPHPExtractVariableUsesSnapshotAndUTF16(t *testing.T) {
	source := "<?php\n/* 😀 */ $x = build();"
	doc := lsp.NewTextDocument("file:///workspace/live.php", source, 7)
	start := uint32(strings.Index(source, "build()"))
	line, column := doc.LineIndex.PositionUTF16(start)
	endLine, endColumn := doc.LineIndex.PositionUTF16(start + 7)
	params := &protocol.CodeActionParams{Range: protocol.Range{Start: protocol.Position{Line: int(line), Character: int(column)}, End: protocol.Position{Line: int(endLine), Character: int(endColumn)}}}
	actions := NewPHPExtractVariableProvider().GetCodeActions(context.Background(), &lsp.CodeActionRequest{CodeActionParams: params, SyntaxContext: lsp.SyntaxContext{Document: doc}})
	require.Len(t, actions, 1)
	assert.Equal(t, "Extract variable '$extracted'", actions[0].Title)
	assert.Equal(t, protocol.CodeActionRefactorExtract, actions[0].Kind)
	require.NotNil(t, actions[0].Edit)
	change := actions[0].Edit.DocumentChanges[0]
	require.NotNil(t, change.TextDocument.Version)
	assert.EqualValues(t, 7, *change.TextDocument.Version)
	require.Len(t, change.Edits, 2)
	assert.Equal(t, params.Range, change.Edits[1].Range)
	assert.Equal(t, "$extracted", change.Edits[1].NewText)
}

func TestPHPExtractVariableRejectsIncompleteSnapshot(t *testing.T) {
	for _, source := range []string{"<?php $x =& build();", "<?php return build(", "<?php return build(); function incomplete("} {
		doc := lsp.NewTextDocument("file:///workspace/live.php", source, 1)
		pos := strings.Index(source, "build")
		params := &protocol.CodeActionParams{Range: protocol.Range{Start: protocol.Position{Character: pos}, End: protocol.Position{Character: pos}}}
		assert.Empty(t, NewPHPExtractVariableProvider().GetCodeActions(context.Background(), &lsp.CodeActionRequest{CodeActionParams: params, SyntaxContext: lsp.SyntaxContext{Document: doc}}))
	}
}

func TestPHPExtractVariableArgumentReferenceResolution(t *testing.T) {
	index, err := php.NewPHPIndex(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })
	for _, tc := range []struct {
		declaration string
		available   bool
	}{
		{"function value() { return 1; }", true},
		{"function &value() { static $x = 1; return $x; }", false},
	} {
		source := "<?php " + tc.declaration + " consume(value());"
		doc := lsp.NewTextDocument("file:///workspace/live.php", source, 1)
		start := strings.LastIndex(source, "value()")
		params := &protocol.CodeActionParams{Range: protocol.Range{Start: protocol.Position{Character: start}, End: protocol.Position{Character: start + 7}}}
		actions := NewPHPExtractVariableProvider(index).GetCodeActions(context.Background(), &lsp.CodeActionRequest{CodeActionParams: params, SyntaxContext: lsp.SyntaxContext{Document: doc}})
		if tc.available {
			require.Len(t, actions, 1)
			assert.Contains(t, actions[0].Edit.DocumentChanges[0].Edits[0].NewText, "($extracted = value())")
		} else {
			assert.Empty(t, actions)
		}
	}
}
