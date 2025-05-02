package completion

import (
	"context"

	"github.com/shopware/shopware-lsp/internal/feature"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type FeatureCompletionProvider struct {
	featureIndex *feature.FeatureIndexer
}

func NewFeatureCompletionProvider(lspServer *lsp.Server) *FeatureCompletionProvider {
	featureIndexer, _ := lspServer.GetIndexer("feature.indexer")
	return &FeatureCompletionProvider{
		featureIndex: featureIndexer.(*feature.FeatureIndexer),
	}
}

func (p *FeatureCompletionProvider) GetCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if params.Node == nil {
		return nil
	}

	if treesitterhelper.TwigStringInFunctionPattern("feature").Matches(params.Node, params.DocumentContent) || treesitterhelper.IsStaticPHPMethodCall("Feature", "isActive").Matches(params.Node, params.DocumentContent) || treesitterhelper.IsSCSSFunctionPattern("feature").Matches(params.Node, params.DocumentContent) {
		completionItems := []protocol.CompletionItem{}
		features, _ := p.featureIndex.GetAllFeatures()
		for _, feature := range features {
			completionItems = append(completionItems, protocol.CompletionItem{
				Label: feature.Name,
				Kind:  int(protocol.FunctionCompletion),
			})
		}

		return completionItems
	}

	return nil
}

func (p *FeatureCompletionProvider) GetTriggerCharacters() []string {
	return []string{}
}
