package completion

import (
	"context"
	"path/filepath"
	"slices"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/shopware/shopware-lsp/internal/systemconfig"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type SystemConfigCompletionProvider struct {
	indexer  *systemconfig.SystemConfigIndexer
	phpIndex *php.PHPIndex
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

	if filepath.Ext(params.TextDocument.URI) == ".php" {
		return s.phpCompletion(ctx, params)
	}

	return nil
}

func (s *SystemConfigCompletionProvider) phpCompletion(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if s.phpIndex.IsMethodCalledOnClass(ctx, params.Node, params.DocumentContent, "Shopware\\Core\\System\\SystemConfig\\SystemConfigService") {
		if s.phpIndex.IsMethodCalledName(ctx, params.Node, params.DocumentContent, "get", "getInt", "getString", "getFloat", "getBool", "set") {
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

		if s.phpIndex.IsMethodCalledName(ctx, params.Node, params.DocumentContent, "getDomain") {
			completions, err := s.indexer.GetAllSystemConfigEntries()
			if err != nil {
				return nil
			}

			var uniqueDomains []string
			for _, completion := range completions {
				if !slices.Contains(uniqueDomains, completion.Namespace) {
					uniqueDomains = append(uniqueDomains, completion.Namespace)
				}
			}

			var completionItems []protocol.CompletionItem
			for _, domain := range uniqueDomains {
				completionItems = append(completionItems, protocol.CompletionItem{
					Label: domain,
				})
			}

			return completionItems
		}
	}

	return nil
}

func (s *SystemConfigCompletionProvider) GetTriggerCharacters() []string {
	return []string{}
}

func NewSystemConfigCompletion(lspServer *lsp.Server) *SystemConfigCompletionProvider {
	indexer, _ := lspServer.GetIndexer("systemconfig.indexer")
	phpIndexer, _ := lspServer.GetIndexer("php.index")
	return &SystemConfigCompletionProvider{
		indexer:  indexer.(*systemconfig.SystemConfigIndexer),
		phpIndex: phpIndexer.(*php.PHPIndex),
	}
}
