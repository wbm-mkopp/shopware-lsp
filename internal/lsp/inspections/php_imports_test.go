package inspections

import (
	"context"
	"testing"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUnusedPHPImportInspectionAndLazyFix(t *testing.T) {
	index, err := php.NewPHPIndex(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })
	inspection := NewPHPImports(index)
	source := "<?php /* 😀 */ use Vendor\\{Used, Unused as Alias}; new Used;"
	document := lsp.NewTextDocument("file:///workspace/live.php", source, 1)
	collector := &problemCollector{}
	require.NoError(t, inspection.Inspect(context.Background(), document, collector))
	require.Len(t, collector.problems, 1)
	problem := collector.problems[0]
	assert.Equal(t, lsp.DiagnosticID("php.unusedImport"), problem.ID)
	assert.Contains(t, problem.Tags, protocol.DiagnosticTagUnnecessary)
	assert.Equal(t, "Unused", source[problem.Range.Start:problem.Range.End])
	assert.Equal(t, 33, wireRange(document.LineIndex, problem.Range).Start.Character)
	require.Len(t, problem.Fixes, 1)
	fix := quickFixWithID(t, inspection, problem.Fixes[0].ID)
	fixCtx := fixContext(t, document, problem, problem.Fixes[0], nil)
	presentation, ok, err := fix.Present(context.Background(), fixCtx)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, lsp.FixLazy, presentation.Resolution)
	assert.Equal(t, "Remove unused import 'Alias'", presentation.Title)
	plan, err := fix.Build(context.Background(), fixCtx)
	require.NoError(t, err)
	require.Len(t, plan.Documents, 1)
	result, err := plan.Documents[0].Apply()
	require.NoError(t, err)
	assert.Equal(t, "<?php /* 😀 */ use Vendor\\{Used}; new Used;", result)
	fixCtx.Document = lsp.NewTextDocument(document.URI, source+" new Alias;", 2)
	_, err = fix.Build(context.Background(), fixCtx)
	require.ErrorIs(t, err, rewrite.ErrStaleHandle)
}

func TestUnusedImportsSuppressionAndIncompleteInput(t *testing.T) {
	index, err := php.NewPHPIndex(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, index.Close()) })
	for _, source := range []string{
		"<?php\n/** @noinspection php.unusedImport */\nuse Vendor\\Unused;\n",
		"<?php\n/** @noinspection PhpUnusedAliasInspection */\nuse Vendor\\Unused;\n",
		"<?php use Vendor\\Unused; function incomplete(",
		"<?php use Vendor\\Unused; use Vendor\\",
	} {
		collector := &problemCollector{}
		require.NoError(t, NewPHPImports(index).Inspect(context.Background(), lsp.NewTextDocument("file:///workspace/live.php", source, 1), collector))
		assert.Empty(t, collector.problems, source)
	}
}
