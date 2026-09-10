package selection

import (
	"context"
	"github.com/shopware/shopware-lsp/internal/language"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
)

// PHPSelectionRangeProvider expands selections through the current lossless CST.
type PHPSelectionRangeProvider struct{}

func NewPHPSelectionRangeProvider() *PHPSelectionRangeProvider { return &PHPSelectionRangeProvider{} }

func (p *PHPSelectionRangeProvider) GetSelectionRanges(ctx context.Context, request *lsp.SelectionRangeRequest) ([]protocol.SelectionRange, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request == nil || request.SelectionRangeParams == nil || request.Document == nil || request.Document.SyntaxLanguage != language.PHP || request.Document.SyntaxTree == nil || request.Document.SyntaxTree.Root == nil || request.Document.LineIndex == nil {
		return nil, nil
	}
	result := make([]protocol.SelectionRange, 0, len(request.Positions))
	for _, position := range request.Positions {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		result = append(result, selectionRangeAt(request.Document.SyntaxTree.Root, request.Document.LineIndex, position))
	}
	return result, nil
}

var _ lsp.SelectionRangeProvider = (*PHPSelectionRangeProvider)(nil)
