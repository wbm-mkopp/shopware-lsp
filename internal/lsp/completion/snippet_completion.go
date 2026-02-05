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
	case ".js", ".ts":
		return s.jsCompletion(ctx, params)
	default:
		return []protocol.CompletionItem{}
	}
}

func (s *SnippetCompletionProvider) twigCompletion(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	// Check for frontend snippet pattern: {{ 'key'|trans }}
	if treesitterhelper.TwigTransPattern().Matches(params.Node, params.DocumentContent) {
		return s.getFrontendSnippetCompletions()
	}

	// Check for admin snippet pattern: {{ $tc('key') }} or {{ $t('key') }}
	if treesitterhelper.TwigAdminSnippetPattern().Matches(params.Node, params.DocumentContent) {
		return s.getAdminSnippetCompletions()
	}

	return []protocol.CompletionItem{}
}

func (s *SnippetCompletionProvider) phpCompletion(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if treesitterhelper.IsPHPThisMethodCall("trans").Matches(params.Node, params.DocumentContent) {
		return s.getFrontendSnippetCompletions()
	}

	return []protocol.CompletionItem{}
}

func (s *SnippetCompletionProvider) jsCompletion(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	// Check for admin snippet pattern: this.$tc('key') or this.$t('key')
	if treesitterhelper.JSAdminSnippetPattern().Matches(params.Node, params.DocumentContent) {
		return s.getAdminSnippetCompletions()
	}

	return []protocol.CompletionItem{}
}

func (s *SnippetCompletionProvider) getFrontendSnippetCompletions() []protocol.CompletionItem {
	snippets, _ := s.snippetIndexer.GetFrontendSnippetsWithText()

	var completionItems []protocol.CompletionItem
	for key, text := range snippets {
		item := protocol.CompletionItem{
			Label:  key,
			Detail: truncateText(text, 50),
			Kind:   int(protocol.TextCompletion),
		}
		if text != "" {
			item.Documentation.Kind = "plaintext"
			item.Documentation.Value = text
		}
		completionItems = append(completionItems, item)
	}

	return completionItems
}

func (s *SnippetCompletionProvider) getAdminSnippetCompletions() []protocol.CompletionItem {
	snippets, _ := s.snippetIndexer.GetAdminSnippetsWithText()

	var completionItems []protocol.CompletionItem
	for key, text := range snippets {
		item := protocol.CompletionItem{
			Label:  key,
			Detail: truncateText(text, 50),
			Kind:   int(protocol.TextCompletion),
		}
		if text != "" {
			item.Documentation.Kind = "plaintext"
			item.Documentation.Value = text
		}
		completionItems = append(completionItems, item)
	}

	return completionItems
}

func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen-3] + "..."
}

func (s *SnippetCompletionProvider) GetTriggerCharacters() []string {
	return []string{}
}
