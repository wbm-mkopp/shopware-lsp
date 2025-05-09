package definition

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/twig"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type TwigDefinitionProvider struct {
	twigIndexer *twig.TwigIndexer
}

func NewTwigDefinitionProvider(lspServer *lsp.Server) *TwigDefinitionProvider {
	twigIndexer, _ := lspServer.GetIndexer("twig.indexer")
	return &TwigDefinitionProvider{
		twigIndexer: twigIndexer.(*twig.TwigIndexer),
	}
}

func (p *TwigDefinitionProvider) GetDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if params.Node == nil {
		return []protocol.Location{}
	}

	switch strings.ToLower(filepath.Ext(params.TextDocument.URI)) {
	case ".php":
		return p.phpDefinitions(ctx, params)
	case ".twig":
		return p.twigDefinitions(ctx, params)
	default:
		return []protocol.Location{}
	}
}

func (p *TwigDefinitionProvider) twigDefinitions(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if treesitterhelper.TwigStringInTagPattern("extends", "sw_extends", "include", "sw_include").Matches(params.Node, []byte(params.DocumentContent)) {
		itemValue := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)

		files, _ := p.twigIndexer.GetTwigFilesByRelPath(itemValue)

		var locations []protocol.Location
		for _, file := range files {
			if file.Path == strings.TrimPrefix(params.TextDocument.URI, "file://") {
				continue
			}

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

	if params.Node.Kind() == "function" {
		functionName := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)
		parentNode := params.Node.Parent()

		if parentNode != nil && parentNode.Kind() == "filter_expression" {
			filters, _ := p.twigIndexer.GetTwigFilter(functionName)

			var locations []protocol.Location
			for _, filter := range filters {
				locations = append(locations, protocol.Location{
					URI: filter.FilePath,
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      int(filter.Line) - 1,
							Character: 0,
						},
						End: protocol.Position{
							Line:      int(filter.Line) - 1,
							Character: 0,
						},
					},
				})
			}

			return locations

		} else {
			functions, _ := p.twigIndexer.GetTwigFunction(functionName)

			var locations []protocol.Location
			for _, function := range functions {
				locations = append(locations, protocol.Location{
					URI: function.FilePath,
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      int(function.Line) - 1,
							Character: 0,
						},
						End: protocol.Position{
							Line:      int(function.Line) - 1,
							Character: 0,
						},
					},
				})
			}

			return locations
		}
	}

	return []protocol.Location{}
}

func (p *TwigDefinitionProvider) phpDefinitions(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if treesitterhelper.IsPHPThisMethodCall("renderStorefront").Matches(params.Node, params.DocumentContent) {
		files, _ := p.twigIndexer.GetTwigFilesByRelPath(treesitterhelper.GetNodeText(params.Node, params.DocumentContent))

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
