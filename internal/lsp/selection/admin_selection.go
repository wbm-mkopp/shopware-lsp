package selection

import (
	"context"

	"github.com/shopware/shopware-lsp/internal/language"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
)

// AdminSelectionRangeProvider exposes lossless native-CST ancestry for
// Administration JavaScript/TypeScript and Twig documents. It deliberately
// starts with the exact non-trivia token before expanding through syntax nodes
// and finally the whole document.
type AdminSelectionRangeProvider struct{}

func NewAdminSelectionRangeProvider() *AdminSelectionRangeProvider {
	return &AdminSelectionRangeProvider{}
}

func (p *AdminSelectionRangeProvider) GetSelectionRanges(
	ctx context.Context,
	request *lsp.SelectionRangeRequest,
) ([]protocol.SelectionRange, error) {
	if ctx.Err() != nil || p == nil || request == nil ||
		request.SelectionRangeParams == nil || request.Document == nil ||
		request.Document.SyntaxTree == nil || request.Document.SyntaxTree.Root == nil ||
		request.Document.LineIndex == nil {
		return nil, nil
	}
	if request.Document.SyntaxLanguage != language.JavaScript &&
		request.Document.SyntaxLanguage != language.Twig &&
		request.Document.SyntaxLanguage != language.Vue {
		return nil, nil
	}
	result := make([]protocol.SelectionRange, 0, len(request.Positions))
	for _, position := range request.Positions {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		result = append(result, selectionRangeAt(
			request.Document.SyntaxTree.Root,
			request.Document.LineIndex,
			position,
		))
	}
	return result, nil
}

var _ lsp.SelectionRangeProvider = (*AdminSelectionRangeProvider)(nil)
