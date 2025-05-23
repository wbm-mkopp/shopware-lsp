package completion

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/extension"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/theme"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	"github.com/shopware/shopware-lsp/internal/twig"
)

type TwigCompletionProvider struct {
	twigIndexer  *twig.TwigIndexer
	iconProvider *theme.IconProvider
}

func NewTwigCompletionProvider(projectRoot string, lspServer *lsp.Server) *TwigCompletionProvider {
	twigIndexer, _ := lspServer.GetIndexer("twig.indexer")
	extensionIndexer, _ := lspServer.GetIndexer("extension.indexer")

	iconProvider := theme.NewIconProvider(projectRoot, extensionIndexer.(*extension.ExtensionIndexer))

	return &TwigCompletionProvider{
		twigIndexer:  twigIndexer.(*twig.TwigIndexer),
		iconProvider: iconProvider,
	}
}

func (p *TwigCompletionProvider) GetCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if params.Node == nil {
		return []protocol.CompletionItem{}
	}

	switch strings.ToLower(filepath.Ext(params.TextDocument.URI)) {
	case ".php":
		return p.phpCompletions(ctx, params)
	case ".twig":
		return p.twigCompletions(ctx, params)
	default:
		return []protocol.CompletionItem{}
	}
}

func (p *TwigCompletionProvider) twigCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {

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

	if treesitterhelper.TwigAutocompleteFilterPattern().Matches(params.Node, params.DocumentContent) {
		filters, _ := p.twigIndexer.GetAllTwigFilters()
		uniqueFilters := make(map[string]struct{})

		var completionItems []protocol.CompletionItem
		for _, filter := range filters {
			if strings.Contains(filter.Name, "*") {
				continue
			}

			if _, ok := uniqueFilters[filter.Name]; ok {
				continue
			}
			uniqueFilters[filter.Name] = struct{}{}

			completionItems = append(completionItems, protocol.CompletionItem{
				Label:            filter.Usage,
				InsertText:       filter.Name + "($0)",
				InsertTextFormat: int(protocol.SnippetTextFormat),
			})
		}
		return completionItems
	}

	if treesitterhelper.TwigStringInTagPattern("sw_icon").Matches(params.Node, params.DocumentContent) {
		cfg := treesitterhelper.ExtractSwIconObjectToMap(params.Node.Parent(), params.DocumentContent)

		pack, ok := cfg["pack"]
		if !ok {
			pack = "default"
		}

		icons := p.iconProvider.GetIcons(pack)

		var completionItems []protocol.CompletionItem
		for _, icon := range icons {
			completionItems = append(completionItems, protocol.CompletionItem{
				Label: icon,
			})
		}
		return completionItems
	}

	if treesitterhelper.TwigSwIconInPackPattern().Matches(params.Node, params.DocumentContent) {
		packs := p.iconProvider.GetIconPacks()

		var completionItems []protocol.CompletionItem
		for _, pack := range packs {
			completionItems = append(completionItems, protocol.CompletionItem{
				Label: pack,
			})
		}
		return completionItems
	}

	if params.Node.Kind() == "template" {
		functions, _ := p.twigIndexer.GetAllTwigFunctions()
		uniqueFunctions := make(map[string]struct{})

		var completionItems []protocol.CompletionItem
		for _, function := range functions {
			if strings.Contains(function.Name, "*") {
				continue
			}

			if _, ok := uniqueFunctions[function.Name]; ok {
				continue
			}
			uniqueFunctions[function.Name] = struct{}{}

			completionItems = append(completionItems, protocol.CompletionItem{
				Label:            function.Usage,
				InsertText:       function.Name + "($0)",
				InsertTextFormat: int(protocol.SnippetTextFormat),
			})
		}

		return completionItems
	}

	return []protocol.CompletionItem{}
}

func (p *TwigCompletionProvider) phpCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if treesitterhelper.IsPHPThisMethodCall("renderStorefront").Matches(params.Node, params.DocumentContent) {
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
	return []string{"\"", "'", "|"}
}
