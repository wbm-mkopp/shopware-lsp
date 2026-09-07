package phpsemantic

import (
	"context"
	"sort"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/phpanalysis"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/parser/cst"
	phpquery "github.com/shopware/shopware-lsp/internal/parser/php/query"
	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/shopware/shopware-lsp/internal/php/semantic"
)

// GetDocumentHighlights uses resolved identities in the current document only.
// Only document-local references are searched; no cross-file occurrence query
// is needed.
func (p *Provider) GetDocumentHighlights(ctx context.Context, request *lsp.DocumentHighlightRequest) ([]protocol.DocumentHighlight, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if p == nil || request == nil || request.DocumentHighlightParams == nil || !phpEditorDocument(request.Document) {
		return nil, nil
	}
	state, err := phpanalysis.ForDocument(p.index, request.Document)
	if err != nil || state == nil {
		return nil, err
	}
	offset := request.Document.LineIndex.OffsetUTF16(uint32(max(request.Position.Line, 0)), uint32(max(request.Position.Character, 0)))
	target, found := php.SymbolAt(state.Document, state.Snapshot, offset)
	if !found {
		return nil, nil
	}
	occurrences := make(map[cst.TextRange]protocol.DocumentHighlightKind)
	add := func(rng cst.TextRange, kind protocol.DocumentHighlightKind) {
		if rng.Len() > 0 && kind > occurrences[rng] {
			occurrences[rng] = kind
		}
	}
	for _, symbol := range state.Document.Symbols {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if symbol.ID == target.ID {
			kind := protocol.DocumentHighlightText
			switch symbol.Kind {
			case semantic.LocalSymbol, semantic.ParameterSymbol, semantic.PropertySymbol:
				kind = protocol.DocumentHighlightWrite
			}
			add(symbol.SelectionRange, kind)
		}
	}
	for _, reference := range state.Document.References {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if reference.Resolved != target.ID {
			continue
		}
		kind := protocol.DocumentHighlightRead
		if reference.Write || phpquery.VariableIsWrite(request.Document.SyntaxTree.Root.NodeAtOffset(reference.Range.Start)) {
			kind = protocol.DocumentHighlightWrite
		}
		add(reference.Range, kind)
	}
	ranges := make([]cst.TextRange, 0, len(occurrences))
	for rng := range occurrences {
		ranges = append(ranges, rng)
	}
	sort.Slice(ranges, func(i, j int) bool {
		if ranges[i].Start != ranges[j].Start {
			return ranges[i].Start < ranges[j].Start
		}
		return ranges[i].End < ranges[j].End
	})
	result := make([]protocol.DocumentHighlight, 0, len(ranges))
	for _, rng := range ranges {
		result = append(result, protocol.DocumentHighlight{Range: *rangeFromText(request.Document.LineIndex, rng), Kind: occurrences[rng]})
	}
	return result, nil
}

var _ lsp.DocumentHighlightProvider = (*Provider)(nil)
