package definition

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/theme"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type ThemeDefinitionProvider struct {
	themeIndexer *theme.ThemeConfigIndexer
}

func NewThemeDefinitionProvider(lspServer *lsp.Server) *ThemeDefinitionProvider {
	themeIndexer, _ := lspServer.GetIndexer("theme.indexer")
	return &ThemeDefinitionProvider{
		themeIndexer: themeIndexer.(*theme.ThemeConfigIndexer),
	}
}

func (p *ThemeDefinitionProvider) GetDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if params.Node == nil {
		return []protocol.Location{}
	}

	switch strings.ToLower(filepath.Ext(params.TextDocument.URI)) {
	case ".scss":
		return p.scssDefinition(ctx, params)
	case ".twig":
		return p.twigDefinition(ctx, params)
	default:
		return []protocol.Location{}
	}
}

func (p *ThemeDefinitionProvider) scssDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if params.Node.Kind() == "variable" {
		nodeText := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)
		locations, _ := p.themeIndexer.GetThemeConfigField(strings.TrimPrefix(nodeText, "$"))

		var result []protocol.Location
		for _, location := range locations {
			result = append(result, protocol.Location{
				URI: fmt.Sprintf("file://%s", location.Path),
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

func (p *ThemeDefinitionProvider) twigDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {

	if treesitterhelper.TwigStringInFunctionPattern("theme_config").Matches(params.Node, params.DocumentContent) {
		nodeText := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)
		locations, _ := p.themeIndexer.GetThemeConfigField(nodeText)

		var result []protocol.Location
		for _, location := range locations {
			result = append(result, protocol.Location{
				URI: fmt.Sprintf("file://%s", location.Path),
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
