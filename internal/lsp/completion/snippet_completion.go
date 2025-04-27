package completion

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/snippet"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type SnippetCompletionProvider struct {
	snippetIndexer *snippet.SnippetIndexer
}

func NewSnippetCompletionProvider(lsp *lsp.Server) *SnippetCompletionProvider {
	snippetIndexer, _ := lsp.GetIndexer("snippet.indexer")

	return &SnippetCompletionProvider{
		snippetIndexer: snippetIndexer.(*snippet.SnippetIndexer),
	}
}

func (s *SnippetCompletionProvider) GetCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if params.Node == nil {
		return []protocol.CompletionItem{}
	}

	switch strings.ToLower(filepath.Ext(params.TextDocument.URI)) {
	case ".twig":
		return s.twigCompletion(ctx, params)
	case ".php":
		return s.phpCompletion(ctx, params)
	default:
		return []protocol.CompletionItem{}
	}
}

func (s *SnippetCompletionProvider) twigCompletion(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if treesitterhelper.TwigTransPattern().Matches(params.Node, params.DocumentContent) {
		snippets, _ := s.snippetIndexer.GetFrontendSnippets()

		var completionItems []protocol.CompletionItem
		for _, snippet := range snippets {
			completionItems = append(completionItems, protocol.CompletionItem{
				Label: snippet,
			})
		}

		return completionItems
	}

	return []protocol.CompletionItem{}
}

func (s *SnippetCompletionProvider) phpCompletion(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if treesitterhelper.IsPHPThisMethodCall(params.Node, params.DocumentContent, "trans").Matches(params.Node, params.DocumentContent) {
		snippets, _ := s.snippetIndexer.GetFrontendSnippets()

		var completionItems []protocol.CompletionItem
		for _, snippet := range snippets {
			completionItems = append(completionItems, protocol.CompletionItem{
				Label: snippet,
			})
		}

		return completionItems
	}

	return []protocol.CompletionItem{}
}

func (s *SnippetCompletionProvider) GetTriggerCharacters() []string {
	return []string{}
}
