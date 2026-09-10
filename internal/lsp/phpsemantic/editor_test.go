package phpsemantic

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shopware/shopware-lsp/internal/indexer"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/shopware/shopware-lsp/internal/uriutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPHPDocumentOutline(t *testing.T) {
	source := `<?php
namespace First;
use Other\Imported;
interface Contract { public function run(); }
trait Shared { public string $label; }
enum State: string { case Ready = 'ready'; }
class Service implements Contract {
    const A = Imported::A, B = OTHER;
    private string $first, $second;
    public function __construct(private readonly Contract $inner) {}
    public function run(): void { $local = 1; }
}
function helper() {}
namespace Second;
const C = First\Service::A;
class Next {}
`
	document := lsp.NewTextDocument("file:///workspace/Example.php", source, 1)
	result, err := New(nil).GetDocumentSymbols(context.Background(), &lsp.DocumentSymbolRequest{Document: document})
	require.NoError(t, err)
	require.Len(t, result, 2)
	assert.Equal(t, []string{"First", "Second"}, outlineNames(result))
	assert.Equal(t, []string{"Contract", "Shared", "State", "Service", "helper"}, outlineNames(result[0].Children))
	assert.Equal(t, []string{"C", "Next"}, outlineNames(result[1].Children))
	service := result[0].Children[3]
	assert.Equal(t, []string{"A", "B", "first", "second", "__construct", "inner", "run"}, outlineNames(service.Children))
	assert.Equal(t, protocol.SymbolConstructor, service.Children[4].Kind)
	assert.Equal(t, protocol.SymbolProperty, service.Children[5].Kind)
	assert.Empty(t, service.Children[6].Children)
	assert.Equal(t, protocol.SymbolEnum, result[0].Children[2].Kind)
	assert.Equal(t, protocol.SymbolEnumMember, result[0].Children[2].Children[0].Kind)
	assertOutlineRanges(t, document, result)
}

func TestPHPOutlineBracedNamespacesAndIncompleteSource(t *testing.T) {
	for _, source := range []string{
		"<?php namespace One { class First {} } namespace Two { function second() {} }",
		"<?php namespace One { class First {} } namespace Two { function second(",
	} {
		document := lsp.NewTextDocument("file:///workspace/live.php", source, 9)
		result, err := New(nil).GetDocumentSymbols(context.Background(), &lsp.DocumentSymbolRequest{Document: document})
		require.NoError(t, err)
		require.Len(t, result, 2)
		assert.Equal(t, []string{"First"}, outlineNames(result[0].Children))
		assert.Equal(t, []string{"second"}, outlineNames(result[1].Children))
		assertOutlineRanges(t, document, result)
	}
	source := "<?php /* 😀 */ class Visible {}"
	document := lsp.NewTextDocument("file:///workspace/live.php", source, 10)
	result, err := New(nil).GetDocumentSymbols(context.Background(), &lsp.DocumentSymbolRequest{Document: document})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, 21, result[0].SelectionRange.Start.Character)
	assertOutlineRanges(t, document, result)
}

func TestPHPDocumentHighlightsUseLiveSemanticIdentity(t *testing.T) {
	idx, err := php.NewPHPIndex(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, idx.Close()) })
	path := filepath.Join(t.TempDir(), "live.php")
	require.NoError(t, idx.Index(indexer.NewParsedFile(path, []byte("<?php function old($value) { return $value; }"))))
	source := `<?php
function first($value) {
    $value = '😀'; echo $value;
    return $value;
}
function second($value) { return $value; }
`
	document := lsp.NewTextDocument(uriutil.FileURI(path), source, 5)
	offset := uint32(strings.Index(source, "echo $value") + len("echo $"))
	syntax := syntaxContext(document, offset)
	result, err := New(idx).GetDocumentHighlights(context.Background(), &lsp.DocumentHighlightRequest{
		DocumentHighlightParams: &protocol.DocumentHighlightParams{Position: positionAt(document, offset)}, SyntaxContext: syntax,
	})
	require.NoError(t, err)
	require.Len(t, result, 4)
	assert.Equal(t, []protocol.DocumentHighlightKind{protocol.DocumentHighlightWrite, protocol.DocumentHighlightWrite, protocol.DocumentHighlightRead, protocol.DocumentHighlightRead}, highlightKinds(result))
	for _, item := range result {
		assert.Equal(t, "$value", sourceRange(document, item.Range))
		assert.Less(t, item.Range.Start.Line, 5)
	}
	assert.Equal(t, 24, result[2].Range.Start.Character)
}

func TestPHPDocumentHighlightsMembersAndUnresolvedNames(t *testing.T) {
	idx, err := php.NewPHPIndex(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, idx.Close()) })
	source := `<?php
class First { public int $value; function get() { $this->value = 1; return $this->value; } }
class Second { public int $value; }
function run(First $first, Second $second) { echo $first->value; echo $second->value; missing(); }
`
	document := lsp.NewTextDocument("file:///workspace/members.php", source, 1)
	for _, tc := range []struct {
		needle string
		count  int
	}{{"$first->value", 4}, {"missing", 0}} {
		offset := uint32(strings.Index(source, tc.needle))
		if tc.count != 0 {
			offset += uint32(len("$first->"))
		}
		result, err := New(idx).GetDocumentHighlights(context.Background(), &lsp.DocumentHighlightRequest{
			DocumentHighlightParams: &protocol.DocumentHighlightParams{Position: positionAt(document, offset)}, SyntaxContext: syntaxContext(document, offset),
		})
		require.NoError(t, err)
		assert.Len(t, result, tc.count)
	}
}

func TestPHPEditorProvidersRejectUnsupportedAndCancelledRequests(t *testing.T) {
	p := New(nil)
	result, err := p.GetDocumentSymbols(context.Background(), &lsp.DocumentSymbolRequest{Document: lsp.NewTextDocument("file:///test.js", "class Other {}", 1)})
	require.NoError(t, err)
	assert.Empty(t, result)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = p.GetDocumentSymbols(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
	_, err = p.GetDocumentHighlights(ctx, nil)
	require.ErrorIs(t, err, context.Canceled)
}

func outlineNames(symbols []protocol.DocumentSymbol) []string {
	result := make([]string, 0, len(symbols))
	for _, symbol := range symbols {
		result = append(result, symbol.Name)
	}
	return result
}
func highlightKinds(items []protocol.DocumentHighlight) []protocol.DocumentHighlightKind {
	result := make([]protocol.DocumentHighlightKind, 0, len(items))
	for _, item := range items {
		result = append(result, item.Kind)
	}
	return result
}
func positionAt(document *lsp.TextDocument, offset uint32) protocol.Position {
	line, character := document.LineIndex.PositionUTF16(offset)
	return protocol.Position{Line: int(line), Character: int(character)}
}
func sourceRange(document *lsp.TextDocument, rng protocol.Range) string {
	start := document.LineIndex.OffsetUTF16(uint32(rng.Start.Line), uint32(rng.Start.Character))
	end := document.LineIndex.OffsetUTF16(uint32(rng.End.Line), uint32(rng.End.Character))
	return document.SourceString()[start:end]
}
func assertOutlineRanges(t *testing.T, document *lsp.TextDocument, symbols []protocol.DocumentSymbol) {
	t.Helper()
	for _, symbol := range symbols {
		assert.Equal(t, symbol.Name, strings.TrimPrefix(sourceRange(document, symbol.SelectionRange), "$"))
		full := sourceRange(document, symbol.Range)
		assert.Contains(t, full, sourceRange(document, symbol.SelectionRange))
		for _, child := range symbol.Children {
			assert.Contains(t, full, sourceRange(document, child.Range))
		}
		assertOutlineRanges(t, document, symbol.Children)
	}
}

func TestPHPOutlineAnonymousClassKeepsMembersInTheirContainer(t *testing.T) {
	source := "<?php namespace { $value = new class { public function run() {} }; }"
	document := lsp.NewTextDocument("file:///live.php", source, 1)
	result, err := New(nil).GetDocumentSymbols(context.Background(), &lsp.DocumentSymbolRequest{Document: document})
	require.NoError(t, err)
	require.Len(t, result, 1)
	assert.Equal(t, "(global)", result[0].Name)
	assert.Equal(t, "namespace", sourceRange(document, result[0].SelectionRange))
	require.Len(t, result[0].Children, 1)
	anonymous := result[0].Children[0]
	assert.Equal(t, "(anonymous class)", anonymous.Name)
	assert.Equal(t, "class", sourceRange(document, anonymous.SelectionRange))
	assert.Equal(t, []string{"run"}, outlineNames(anonymous.Children))
}
