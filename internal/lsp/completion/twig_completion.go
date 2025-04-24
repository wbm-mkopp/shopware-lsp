package completion

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/symfony"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	"github.com/shopware/shopware-lsp/internal/twig"
)

type TwigCompletionProvider struct {
	twigIndexer  *twig.TwigIndexer
	routeIndexer *symfony.RouteIndexer
}

func NewTwigCompletionProvider(lspServer *lsp.Server) *TwigCompletionProvider {
	twigIndexer, _ := lspServer.GetIndexer("twig.indexer")
	routeIndexer, _ := lspServer.GetIndexer("symfony.route")
	return &TwigCompletionProvider{
		twigIndexer:  twigIndexer.(*twig.TwigIndexer),
		routeIndexer: routeIndexer.(*symfony.RouteIndexer),
	}
}

func (p *TwigCompletionProvider) GetCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if params.Node == nil {
		return []protocol.CompletionItem{}
	}

	fileExt := strings.ToLower(filepath.Ext(params.TextDocument.URI))

	if fileExt == ".php" {
		return p.phpCompletions(ctx, params)
	}

	if fileExt != ".twig" {
		return []protocol.CompletionItem{}
	}

	if treesitterhelper.TwigStringInTagPattern("extends", "sw_extends", "include", "sw_include").Matches(params.Node, params.DocumentContent) {
		files, _ := p.twigIndexer.GetAllTemplateFiles()

		var completionItems []protocol.CompletionItem
		for _, file := range files {
			completionItems = append(completionItems, protocol.CompletionItem{
				Label: file,
			})
		}

		return completionItems
	}

	if treesitterhelper.TwigStringInFunctionPattern("seoUrl", "url", "path").Matches(params.Node, params.DocumentContent) {
		routes, _ := p.routeIndexer.GetRoutes()

		var completionItems []protocol.CompletionItem
		for _, route := range routes {
			completionItems = append(completionItems, protocol.CompletionItem{
				Label: route.Name,
			})
		}

		return completionItems
	}

	return []protocol.CompletionItem{}
}

func (p *TwigCompletionProvider) phpCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if treesitterhelper.IsPHPRenderStorefrontCallEdit(params.Node, params.DocumentContent) {
		files, _ := p.twigIndexer.GetAllTemplateFiles()

		var completionItems []protocol.CompletionItem
		for _, file := range files {
			completionItems = append(completionItems, protocol.CompletionItem{
				Label: file,
			})
		}

		return completionItems
	}

	return []protocol.CompletionItem{}
}

func (p *TwigCompletionProvider) GetTriggerCharacters() []string {
	return []string{"\"", "'"}
}
