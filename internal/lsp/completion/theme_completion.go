package completion

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/theme"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type ThemeCompletionProvider struct {
	themeIndexer *theme.ThemeConfigIndexer
}

func NewThemeCompletionProvider(lspServer *lsp.Server) *ThemeCompletionProvider {
	themeIndexer, _ := lspServer.GetIndexer("theme.indexer")
	return &ThemeCompletionProvider{
		themeIndexer: themeIndexer.(*theme.ThemeConfigIndexer),
	}
}

func (p *ThemeCompletionProvider) GetCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if params.Node == nil {
		return []protocol.CompletionItem{}
	}

	switch strings.ToLower(filepath.Ext(params.TextDocument.URI)) {
	case ".scss":
		return p.scssCompletions(ctx, params)
	case ".twig":
		return p.twigCompletions(ctx, params)
	default:
		return []protocol.CompletionItem{}
	}
}

func (p *ThemeCompletionProvider) scssCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	var completionItems []protocol.CompletionItem

	elements, _ := p.themeIndexer.GetAllThemeConfigFields()
	uniqueElements := make(map[string]struct{})

	for _, element := range elements {
		if !element.Scss {
			continue
		}

		if _, exists := uniqueElements[element.Key]; !exists {
			uniqueElements[element.Key] = struct{}{}
			completionItems = append(completionItems, protocol.CompletionItem{
				Label: "$" + element.Key,
			})
		}
	}

	return completionItems
}

func (p *ThemeCompletionProvider) twigCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if treesitterhelper.TwigStringInFunctionPattern("theme_config").Matches(params.Node, params.DocumentContent) {
		themes, _ := p.themeIndexer.GetThemeConfigFields()

		uniqueThemes := make(map[string]struct{})
		var completionItems []protocol.CompletionItem
		for _, theme := range themes {
			if _, exists := uniqueThemes[theme]; !exists {
				uniqueThemes[theme] = struct{}{}
				completionItems = append(completionItems, protocol.CompletionItem{
					Label: theme,
				})
			}
		}

		return completionItems
	}

	return []protocol.CompletionItem{}
}

func (p *ThemeCompletionProvider) GetTriggerCharacters() []string {
	return []string{}
}
