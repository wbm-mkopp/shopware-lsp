package definition

import (
	"context"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/systemconfig"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type SystemConfigDefinitionProvider struct {
	indexer *systemconfig.SystemConfigIndexer
}

func NewSystemConfigDefinitionProvider(lspServer *lsp.Server) *SystemConfigDefinitionProvider {
	indexer, _ := lspServer.GetIndexer("systemconfig.indexer")
	return &SystemConfigDefinitionProvider{
		indexer: indexer.(*systemconfig.SystemConfigIndexer),
	}
}

func (s *SystemConfigDefinitionProvider) GetDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if params.Node == nil {
		return nil
	}

	if treesitterhelper.TwigStringInFunctionPattern("config").Matches(params.Node, params.DocumentContent) {
		value := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)

		entries, err := s.indexer.GetSystemConfigEntry(value)
		if err != nil {
			return nil
		}

		locations := make([]protocol.Location, 0, len(entries))
		for _, entry := range entries {
			locations = append(locations, protocol.Location{
				URI: "file://" + entry.FilePath,
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      entry.Line - 1, // LSP uses 0-based line numbers
						Character: 0,
					},
					End: protocol.Position{
						Line:      entry.Line - 1,
						Character: 0,
					},
				},
			})
		}

		return locations
	}

	return nil
}
