package definition

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/extension"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/theme"
	"github.com/shopware/shopware-lsp/internal/twig"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type TwigDefinitionProvider struct {
	twigIndexer  *twig.TwigIndexer
	iconProvider *theme.IconProvider
}

func NewTwigDefinitionProvider(projectRoot string, lspServer *lsp.Server) *TwigDefinitionProvider {
	twigIndexer, _ := lspServer.GetIndexer("twig.indexer")
	extensionIndexer, _ := lspServer.GetIndexer("extension.indexer")

	iconProvider := theme.NewIconProvider(projectRoot, extensionIndexer.(*extension.ExtensionIndexer))

	return &TwigDefinitionProvider{
		twigIndexer:  twigIndexer.(*twig.TwigIndexer),
		iconProvider: iconProvider,
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
				URI: fmt.Sprintf("file://%s", file.Path),
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
					URI: fmt.Sprintf("file://%s", filter.FilePath),
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
					URI: fmt.Sprintf("file://%s", function.FilePath),
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

	if treesitterhelper.TwigStringInTagPattern("sw_icon").Matches(params.Node, []byte(params.DocumentContent)) {
		text := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)

		cfg := treesitterhelper.ExtractSwIconObjectToMap(params.Node.Parent(), params.DocumentContent)

		pack, ok := cfg["pack"]
		if !ok {
			pack = "default"
		}

		icon := p.iconProvider.GetIcon(pack, text)

		if icon != "" {
			locations := []protocol.Location{
				{
					URI: fmt.Sprintf("file://%s", icon),
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
				},
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
				URI: fmt.Sprintf("file://%s", file.Path),
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
