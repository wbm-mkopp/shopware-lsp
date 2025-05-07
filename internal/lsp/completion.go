package lsp

import (
	"context"
	"path/filepath"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
)

// completion handles textDocument/completion requests
func (s *Server) completion(ctx context.Context, params *protocol.CompletionParams) *protocol.CompletionList {
	node, docText, ok := s.documentManager.GetNodeAtPosition(params.TextDocument.URI, params.Position.Line, params.Position.Character)
	if ok {
		params.Node = node
		params.DocumentContent = docText.Text

		if filepath.Ext(params.TextDocument.URI) == ".php" {
			phpIndex, _ := s.GetIndexer("php.index")
			ctx = phpIndex.(*php.PHPIndex).AddContext(ctx, node, docText.Text)
		}
	}

	// Collect completion items from all providers
	var items []protocol.CompletionItem
	for _, provider := range s.completionProviders {
		providerItems := provider.GetCompletions(ctx, params)
		items = append(items, providerItems...)
	}

	// Return the completion list
	return &protocol.CompletionList{
		IsIncomplete: false,
		Items:        items,
	}
}
