package definition

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/twig"

	symfony "github.com/shopware/shopware-lsp/internal/symfony"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type TwigDefinitionProvider struct {
	twigIndexer  *twig.TwigIndexer
	routeIndexer *symfony.RouteIndexer
}

func NewTwigDefinitionProvider(lspServer *lsp.Server) *TwigDefinitionProvider {
	twigIndexer, _ := lspServer.GetIndexer("twig.indexer")
	routeIndexer, _ := lspServer.GetIndexer("symfony.route")
	return &TwigDefinitionProvider{
		twigIndexer:  twigIndexer.(*twig.TwigIndexer),
		routeIndexer: routeIndexer.(*symfony.RouteIndexer),
	}
}

func (p *TwigDefinitionProvider) GetDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if params.Node == nil {
		return []protocol.Location{}
	}

	fileExt := strings.ToLower(filepath.Ext(params.TextDocument.URI))

	if fileExt == ".php" {
		return p.phpDefinitions(ctx, params)
	}

	if fileExt != ".twig" {
		return []protocol.Location{}
	}

	if treesitterhelper.TwigStringInTagPattern("extends", "sw_extends", "include", "sw_include").Matches(params.Node, []byte(params.DocumentContent)) {
		itemValue := twig.CleanupTemplatePath(treesitterhelper.GetNodeText(params.Node, params.DocumentContent))

		files, _ := p.twigIndexer.GetTwigFilesByRelPath(itemValue)

		var locations []protocol.Location
		for _, file := range files {
			locations = append(locations, protocol.Location{
				URI: file.Path,
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      0,
						Character: 0,
					},
					End: protocol.Position{
						Line:      0,
						Character: 0,
					},
				},
			})
		}

		return locations
	}

	if treesitterhelper.TwigStringInFunctionPattern("seoUrl", "url", "path").Matches(params.Node, []byte(params.DocumentContent)) {
		routes, _ := p.routeIndexer.GetRoute(treesitterhelper.GetNodeText(params.Node, params.DocumentContent))

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

func (p *TwigDefinitionProvider) phpDefinitions(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if treesitterhelper.IsPHPRenderStorefrontCall(params.Node, params.DocumentContent) {
		files, _ := p.twigIndexer.GetTwigFilesByRelPath(twig.CleanupTemplatePath(treesitterhelper.GetNodeText(params.Node, params.DocumentContent)))

		var locations []protocol.Location
		for _, file := range files {
			locations = append(locations, protocol.Location{
				URI: file.Path,
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      0,
						Character: 0,
					},
					End: protocol.Position{
						Line:      0,
						Character: 0,
					},
				},
			})
		}

		return locations
	}

	return []protocol.Location{}
}
