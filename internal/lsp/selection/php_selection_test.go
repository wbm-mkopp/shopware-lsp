package selection

import (
	"context"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestPHPSelectionsExpandThroughCurrentSyntax(t *testing.T) {
	source := "<?php\nclass Example {\n function run($value) {\n  echo '😀'; return $value->method();\n }\n}\n"
	document := lsp.NewTextDocument("file:///live.php", source, 1)
	offset := uint32(strings.Index(source, "method"))
	line, column := document.LineIndex.PositionUTF16(offset)
	positions := []protocol.Position{{Line: int(line), Character: int(column)}, {Line: 3, Character: 0}, {Line: 6, Character: 0}}
	result, err := NewPHPSelectionRangeProvider().GetSelectionRanges(context.Background(), &lsp.SelectionRangeRequest{Document: document, SelectionRangeParams: &protocol.SelectionRangeParams{Positions: positions}})
	require.NoError(t, err)
	require.Len(t, result, len(positions))
	texts := adminSelectionTexts(t, source, result[0])
	assert.Equal(t, "method", texts[0])
	assert.Contains(t, texts, "$value->method()")
	assert.Contains(t, texts, "return $value->method();")
	assert.Contains(t, texts, source)
	for i, item := range result {
		assertAdminSelectionStrictlyNested(t, item)
		for current := &item; current != nil; current = current.Parent {
			cursor := positions[i]
			start, end := current.Range.Start, current.Range.End
			assert.True(t, start.Line < cursor.Line || start.Line == cursor.Line && start.Character <= cursor.Character)
			assert.True(t, end.Line > cursor.Line || end.Line == cursor.Line && end.Character >= cursor.Character)
		}
	}
}

func TestPHPSelectionEmptyIncompleteAndUnsupported(t *testing.T) {
	provider := NewPHPSelectionRangeProvider()
	for _, source := range []string{"", "<?php function incomplete($value", "<?php\n/* 😀 */\n"} {
		document := lsp.NewTextDocument("file:///live.php", source, 1)
		result, err := provider.GetSelectionRanges(context.Background(), &lsp.SelectionRangeRequest{Document: document, SelectionRangeParams: &protocol.SelectionRangeParams{Positions: []protocol.Position{{}}}})
		require.NoError(t, err)
		require.Len(t, result, 1)
		assertAdminSelectionStrictlyNested(t, result[0])
	}
	result, err := provider.GetSelectionRanges(context.Background(), &lsp.SelectionRangeRequest{Document: lsp.NewTextDocument("file:///x.js", "x", 1), SelectionRangeParams: &protocol.SelectionRangeParams{Positions: []protocol.Position{{}}}})
	require.NoError(t, err)
	assert.Empty(t, result)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = provider.GetSelectionRanges(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
}
