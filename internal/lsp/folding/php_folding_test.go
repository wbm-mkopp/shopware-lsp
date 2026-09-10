package folding

import (
	"context"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestPHPFoldingUsesCurrentSyntax(t *testing.T) {
	source := `<?php
use First\Thing;
use Second\Thing as Other;
/**
 * Documentation 😀
 */
class Example {
    public function run() {
        $items = [
            1,
            2,
        ];
        if ($items):
            echo 'yes';
        endif;
    }
}
namespace Separate;
use Third\Thing;
use Fourth\Thing as Last;
`
	result := phpFolds(t, source)
	for _, expected := range []protocol.FoldingRange{
		{StartLine: 1, EndLine: 2, Kind: protocol.FoldingRangeKindImports},
		{StartLine: 3, EndLine: 5, Kind: protocol.FoldingRangeKindComment},
		{StartLine: 6, EndLine: 15}, {StartLine: 7, EndLine: 14},
		{StartLine: 8, EndLine: 10}, {StartLine: 12, EndLine: 13},
		{StartLine: 18, EndLine: 19, Kind: protocol.FoldingRangeKindImports},
	} {
		assert.Contains(t, result, expected)
	}
	for _, fold := range result {
		assert.Nil(t, fold.StartCharacter)
		assert.Nil(t, fold.EndCharacter)
		assert.Greater(t, fold.EndLine, fold.StartLine)
	}
}

func TestPHPFoldingIncompleteBlocksCommentsAndStrings(t *testing.T) {
	source := "<?php\r\n// one\r\n// two\r\n\r\n// separate\r\nclass Live {\r\n function unfinished() {\r\n  echo 1;\r\n  echo 2;"
	result := phpFolds(t, source)
	assert.Contains(t, result, protocol.FoldingRange{StartLine: 1, EndLine: 2, Kind: protocol.FoldingRangeKindComment})
	assert.NotContains(t, result, protocol.FoldingRange{StartLine: 1, EndLine: 4, Kind: protocol.FoldingRangeKindComment})
	assert.Contains(t, result, protocol.FoldingRange{StartLine: 5, EndLine: 8})
	assert.Contains(t, result, protocol.FoldingRange{StartLine: 6, EndLine: 8})
	strings := phpFolds(t, "<?php\n$value = <<<'TEXT'\nfirst\nsecond\nTEXT;\n")
	assert.Contains(t, strings, protocol.FoldingRange{StartLine: 1, EndLine: 4})
	assert.Empty(t, phpFolds(t, "<?php class Tiny {}"))
}

func TestPHPFoldingUnsupportedAndCancellation(t *testing.T) {
	provider := NewPHPFoldingProvider()
	result, err := provider.GetFoldingRanges(context.Background(), &lsp.FoldingRangeRequest{Document: lsp.NewTextDocument("file:///test.js", "{\n x\n}", 1)})
	require.NoError(t, err)
	assert.Empty(t, result)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = provider.GetFoldingRanges(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
}

func phpFolds(t *testing.T, source string) []protocol.FoldingRange {
	t.Helper()
	result, err := NewPHPFoldingProvider().GetFoldingRanges(context.Background(), &lsp.FoldingRangeRequest{Document: lsp.NewTextDocument("file:///live.php", source, 7)})
	require.NoError(t, err)
	return result
}

func TestPHPFoldingSwitchMatchAndPropertyHooks(t *testing.T) {
	source := `<?php
class Example {
 public string $label {
  get {
   return 'label';
  }
 }
 function run($value) {
  switch ($value) {
   case 1:
    return match ($value) {
     1 => 'one',
     default => 'other',
    };
  }
 }
}`
	result := phpFolds(t, source)
	for _, fold := range []protocol.FoldingRange{
		{StartLine: 2, EndLine: 5}, {StartLine: 3, EndLine: 4},
		{StartLine: 8, EndLine: 13}, {StartLine: 10, EndLine: 12},
	} {
		assert.Contains(t, result, fold)
	}
}
