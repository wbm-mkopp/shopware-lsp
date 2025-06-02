package definition

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/shopware/shopware-lsp/internal/systemconfig"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type SystemConfigDefinitionProvider struct {
	indexer  *systemconfig.SystemConfigIndexer
	phpIndex *php.PHPIndex
}

func NewSystemConfigDefinitionProvider(lspServer *lsp.Server) *SystemConfigDefinitionProvider {
	indexer, _ := lspServer.GetIndexer("systemconfig.indexer")
	phpIndex, _ := lspServer.GetIndexer("php.index")
	return &SystemConfigDefinitionProvider{
		indexer:  indexer.(*systemconfig.SystemConfigIndexer),
		phpIndex: phpIndex.(*php.PHPIndex),
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
				URI: fmt.Sprintf("file://%s", entry.FilePath),
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

	if filepath.Ext(params.TextDocument.URI) == ".php" {
		return s.phpDefinition(ctx, params)
	}

	return nil
}

func (s *SystemConfigDefinitionProvider) phpDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if s.phpIndex.IsMethodCalledOnClass(ctx, params.Node, params.DocumentContent, "Shopware\\Core\\System\\SystemConfig\\SystemConfigService") {
		if s.phpIndex.IsMethodCalledName(ctx, params.Node, params.DocumentContent, "get", "getInt", "getString", "getFloat", "getBool", "set") {
			value := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)

			entries, err := s.indexer.GetSystemConfigEntry(value)
			if err != nil {
				return nil
			}

			locations := make([]protocol.Location, 0, len(entries))
			for _, entry := range entries {
				locations = append(locations, protocol.Location{
					URI: fmt.Sprintf("file://%s", entry.FilePath),
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

		if s.phpIndex.IsMethodCalledName(ctx, params.Node, params.DocumentContent, "getDomain") {
			value := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)

			entries, err := s.indexer.GetAllSystemConfigEntries()
			if err != nil {
				return nil
			}

			locations := make([]protocol.Location, 0, len(entries))
			for _, entry := range entries {
				if entry.Namespace != value {
					continue
				}

				locations = append(locations, protocol.Location{
					URI: fmt.Sprintf("file://%s", entry.FilePath),
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

				break
			}

			return locations
		}
	}

	return nil
}
