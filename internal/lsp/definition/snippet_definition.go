package definition

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/snippet"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type SnippetDefinitionProvider struct {
	snippetIndexer *snippet.SnippetIndexer
}

func NewSnippetDefinitionProvider(lspServer *lsp.Server) *SnippetDefinitionProvider {
	snippetIndexer, _ := lspServer.GetIndexer("snippet.indexer")
	return &SnippetDefinitionProvider{
		snippetIndexer: snippetIndexer.(*snippet.SnippetIndexer),
	}
}

func (s *SnippetDefinitionProvider) GetDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if params.Node == nil {
		return []protocol.Location{}
	}

	switch strings.ToLower(filepath.Ext(params.TextDocument.URI)) {
	case ".twig":
		return s.twigDefinitions(ctx, params)
	case ".php":
		return s.phpDefinitions(ctx, params)
	default:
		return []protocol.Location{}
	}
}

func (s *SnippetDefinitionProvider) twigDefinitions(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if treesitterhelper.TwigTransPattern().Matches(params.Node, params.DocumentContent) {
		snippets, _ := s.snippetIndexer.GetFrontendSnippet(treesitterhelper.GetNodeText(params.Node, params.DocumentContent))

		var locations []protocol.Location

		for _, snippet := range snippets {
			locations = append(locations, protocol.Location{
				URI: fmt.Sprintf("file://%s", snippet.File),
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      snippet.Line - 1,
						Character: 0,
					},
					End: protocol.Position{
						Line:      snippet.Line - 1,
						Character: 0,
					},
				},
			})
		}

		return locations
	}

	return []protocol.Location{}
}

func (s *SnippetDefinitionProvider) phpDefinitions(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if treesitterhelper.IsPHPThisMethodCall("trans").Matches(params.Node, params.DocumentContent) {
		value := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)
		snippets, _ := s.snippetIndexer.GetFrontendSnippet(value)

		var locations []protocol.Location
		for _, snippet := range snippets {
			locations = append(locations, protocol.Location{
				URI: fmt.Sprintf("file://%s", snippet.File),
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      snippet.Line - 1,
						Character: 0,
					},
					End: protocol.Position{
						Line:      snippet.Line - 1,
						Character: 0,
					},
				},
			})
		}

		return locations
	}

	return []protocol.Location{}
}
