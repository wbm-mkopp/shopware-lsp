package lsp

import (
	"context"
	"path/filepath"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
)

// hover handles textDocument/hover requests
func (s *Server) hover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	node, docText, ok := s.documentManager.GetNodeAtPosition(params.TextDocument.URI, params.Position.Line, params.Position.Character)
	if ok {
		params.Node = node
		params.DocumentContent = docText.Text

		if filepath.Ext(params.TextDocument.URI) == ".php" {
			phpIndex, _ := s.GetIndexer("php.index")
			ctx = phpIndex.(*php.PHPIndex).AddContext(ctx, node, docText.Text)
		}
	}

	// Try each hover provider until one returns a result
	for _, provider := range s.hoverProviders {
		hover, err := provider.GetHover(ctx, params)
		if err != nil {
			continue
		}
		if hover != nil {
			return hover, nil
		}
	}

	// No hover information available
	return nil, nil
}
