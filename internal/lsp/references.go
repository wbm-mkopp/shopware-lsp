package lsp

import (
	"context"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
)

// references handles textDocument/references requests
func (s *Server) references(ctx context.Context, params *protocol.ReferenceParams) []protocol.Location {
	node, docText, ok := s.documentManager.GetNodeAtPosition(params.TextDocument.URI, params.Position.Line, params.Position.Character)
	if ok {
		params.Node = node
		params.DocumentContent = docText.Text
	}

	// Collect reference locations from all providers
	var locations []protocol.Location
	for _, provider := range s.referencesProviders {
		providerLocations := provider.GetReferences(ctx, params)
		locations = append(locations, providerLocations...)
	}

	return locations
}
