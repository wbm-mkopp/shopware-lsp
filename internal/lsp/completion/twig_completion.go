package completion

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	"github.com/shopware/shopware-lsp/internal/twig"
)

type TwigCompletionProvider struct {
	twigIndexer *twig.TwigIndexer
}

func NewTwigCompletionProvider(lspServer *lsp.Server) *TwigCompletionProvider {
	twigIndexer, _ := lspServer.GetIndexer("twig.indexer")
	return &TwigCompletionProvider{
		twigIndexer: twigIndexer.(*twig.TwigIndexer),
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
