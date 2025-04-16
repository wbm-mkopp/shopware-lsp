package lsp

import (
	"context"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
)

// definition handles textDocument/definition requests
func (s *Server) definition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	node, docText, ok := s.documentManager.GetNodeAtPosition(params.TextDocument.URI, params.Position.Line, params.Position.Character)
	if ok {
		params.Node = node
		params.DocumentContent = docText.Text
	}

	// Collect definition locations from all providers
	var locations []protocol.Location
	for _, provider := range s.definitionProviders {
		providerLocations := provider.GetDefinition(ctx, params)
		locations = append(locations, providerLocations...)
	}

	return locations
}
