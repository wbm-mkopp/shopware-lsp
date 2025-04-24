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

	fileExt := strings.ToLower(filepath.Ext(params.TextDocument.URI))

	if fileExt == ".php" {
		return p.phpDefinitions(ctx, params)
	}

	if fileExt != ".twig" {
		return []protocol.Location{}
	}

	if treesitterhelper.IsTwigTag(params.Node, []byte(params.DocumentContent), "extends", "sw_extends", "include", "sw_include") {
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
