package phpsemantic

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/folding"
	"github.com/shopware/shopware-lsp/internal/lsp/phpanalysis"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/lsp/selection"
	"github.com/shopware/shopware-lsp/internal/php"
)

func BenchmarkPHPEditorBasics(b *testing.B) {
	var source strings.Builder
	source.WriteString("<?php\nclass Large {\n")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&source, " public function method%d($value): int {\n  $value += 1;\n  return $value;\n }\n", i)
	}
	source.WriteString("}\n")
	document := lsp.NewTextDocument("file:///workspace/Large.php", source.String(), 1)
	ctx := context.Background()
	p := New(nil)
	b.Run("outline", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := p.GetDocumentSymbols(ctx, &lsp.DocumentSymbolRequest{Document: document}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("folding", func(b *testing.B) {
		provider := folding.NewPHPFoldingProvider()
		b.ReportAllocs()
		for b.Loop() {
			if _, err := provider.GetFoldingRanges(ctx, &lsp.FoldingRangeRequest{Document: document}); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("selection", func(b *testing.B) {
		provider := selection.NewPHPSelectionRangeProvider()
		request := &lsp.SelectionRangeRequest{Document: document, SelectionRangeParams: &protocol.SelectionRangeParams{Positions: []protocol.Position{{Line: 3, Character: 8}}}}
		b.ReportAllocs()
		for b.Loop() {
			if _, err := provider.GetSelectionRanges(ctx, request); err != nil {
				b.Fatal(err)
			}
		}
	})
	index, err := php.NewPHPIndex(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() {
		if err := index.Close(); err != nil {
			b.Error(err)
		}
	})
	p = New(index)
	syntax := syntaxContext(document, uint32(strings.Index(source.String(), "return $value")+len("return $")))
	request := &lsp.DocumentHighlightRequest{SyntaxContext: syntax, DocumentHighlightParams: &protocol.DocumentHighlightParams{Position: protocol.Position{Line: 4, Character: 10}}}
	if _, err := phpanalysis.ForDocument(index, document); err != nil {
		b.Fatal(err)
	}
	b.Run("highlights_cached", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if _, err := p.GetDocumentHighlights(ctx, request); err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("highlights_after_edit", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			live := lsp.NewTextDocument(document.URI, source.String(), 2)
			request.Document = live
			request.LineIndex = live.LineIndex
			request.Root = live.SyntaxTree.Root
			if _, err := p.GetDocumentHighlights(ctx, request); err != nil {
				b.Fatal(err)
			}
		}
	})
}
