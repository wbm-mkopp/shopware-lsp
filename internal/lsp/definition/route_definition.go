package definition

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/symfony"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type RouteDefinitionProvider struct {
	routeIndex *symfony.RouteIndexer
}

func NewRouteDefinitionProvider(server *lsp.Server) *RouteDefinitionProvider {
	routeIndexer, _ := server.GetIndexer("symfony.route")
	return &RouteDefinitionProvider{
		routeIndex: routeIndexer.(*symfony.RouteIndexer),
	}
}

func (p *RouteDefinitionProvider) GetDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if params.Node == nil {
		return []protocol.Location{}
	}

	switch strings.ToLower(filepath.Ext(params.TextDocument.URI)) {
	case ".php":
		return p.phpDefinition(ctx, params)
	case ".twig":
		return p.twigDefinition(ctx, params)
	default:
		return []protocol.Location{}
	}
}

func (p *RouteDefinitionProvider) phpDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if treesitterhelper.IsPHPRedirectToRoute(params.Node, params.DocumentContent) {
		currentText := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)

		locations, _ := p.routeIndex.GetRoute(currentText)

		var result []protocol.Location
		for _, location := range locations {
			result = append(result, protocol.Location{
				URI: location.FilePath,
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      location.Line - 1,
						Character: 0,
					},
					End: protocol.Position{
						Line:      location.Line - 1,
						Character: 0,
					},
				},
			})
		}

		return result
	}

	return []protocol.Location{}
}

func (p *RouteDefinitionProvider) twigDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if treesitterhelper.TwigStringInFunctionPattern("seoUrl", "url", "path").Matches(params.Node, []byte(params.DocumentContent)) {
		routes, _ := p.routeIndex.GetRoute(treesitterhelper.GetNodeText(params.Node, params.DocumentContent))

		var locations []protocol.Location
		for _, route := range routes {
			locations = append(locations, protocol.Location{
				URI: route.FilePath,
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      route.Line - 1,
						Character: 0,
					},
					End: protocol.Position{
						Line:      route.Line - 1,
						Character: 0,
					},
				},
			})
		}

		return locations
	}

	return []protocol.Location{}
}
