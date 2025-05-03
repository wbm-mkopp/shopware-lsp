package completion

import (
	"context"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/systemconfig"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type SystemConfigCompletionProvider struct {
	indexer *systemconfig.SystemConfigIndexer
}

func (s *SystemConfigCompletionProvider) GetCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if params.Node == nil {
		return nil
	}

	if treesitterhelper.TwigStringInFunctionPattern("config").Matches(params.Node, params.DocumentContent) {
		completions, err := s.indexer.GetAllSystemConfigEntries()
		if err != nil {
			return nil
		}

		var completionItems []protocol.CompletionItem
		for _, completion := range completions {
			completionItems = append(completionItems, protocol.CompletionItem{
				Label: completion.Name,
			})
		}

		return completionItems
	}

	return nil
}

func (s *SystemConfigCompletionProvider) GetTriggerCharacters() []string {
	return []string{}
}

func NewSystemConfigCompletion(lspServer *lsp.Server) *SystemConfigCompletionProvider {
	indexer, _ := lspServer.GetIndexer("systemconfig.indexer")
	return &SystemConfigCompletionProvider{
		indexer: indexer.(*systemconfig.SystemConfigIndexer),
	}
}
