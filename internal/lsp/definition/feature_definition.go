package definition

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/feature"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type FeatureDefinitionProvider struct {
	featureIndex *feature.FeatureIndexer
}

func NewFeatureDefinitionProvider(lspServer *lsp.Server) *FeatureDefinitionProvider {
	featureIndexer, _ := lspServer.GetIndexer("feature.indexer")
	return &FeatureDefinitionProvider{
		featureIndex: featureIndexer.(*feature.FeatureIndexer),
	}
}

func (p *FeatureDefinitionProvider) GetDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if params.Node == nil {
		return []protocol.Location{}
	}

	switch strings.ToLower(filepath.Ext(params.TextDocument.URI)) {
	case ".twig":
		return p.twigDefinition(ctx, params)
	case ".php":
		return p.phpDefinition(ctx, params)
	case ".scss":
		return p.scssDefinition(ctx, params)
	default:
		return []protocol.Location{}
	}
}

func (p *FeatureDefinitionProvider) twigDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if treesitterhelper.TwigStringInFunctionPattern("feature").Matches(params.Node, params.DocumentContent) {
		featureName := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)
		features, _ := p.featureIndex.GetFeatureByName(featureName)

		var locations []protocol.Location
		for _, feature := range features {
			locations = append(locations, protocol.Location{
				URI: fmt.Sprintf("file://%s", feature.File),
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      feature.Line - 1,
						Character: 0,
					},
					End: protocol.Position{
						Line:      feature.Line - 1,
						Character: 0,
					},
				},
			})
		}

		return locations
	}

	return []protocol.Location{}
}

func (p *FeatureDefinitionProvider) phpDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	// Check for PHP feature flag usage patterns
	if treesitterhelper.IsStaticPHPMethodCall("Feature", "isActive").Matches(params.Node, params.DocumentContent) {
		featureName := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)
		features, _ := p.featureIndex.GetFeatureByName(featureName)

		var locations []protocol.Location
		for _, feature := range features {
			locations = append(locations, protocol.Location{
				URI: fmt.Sprintf("file://%s", feature.File),
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      feature.Line - 1,
						Character: 0,
					},
					End: protocol.Position{
						Line:      feature.Line - 1,
						Character: 0,
					},
				},
			})
		}

		return locations
	}

	return []protocol.Location{}
}

func (p *FeatureDefinitionProvider) scssDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if treesitterhelper.IsSCSSFunctionPattern("feature").Matches(params.Node, params.DocumentContent) {
		featureName := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)
		features, _ := p.featureIndex.GetFeatureByName(featureName)

		var locations []protocol.Location
		for _, feature := range features {
			locations = append(locations, protocol.Location{
				URI: fmt.Sprintf("file://%s", feature.File),
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      feature.Line - 1,
						Character: 0,
					},
					End: protocol.Position{
						Line:      feature.Line - 1,
						Character: 0,
					},
				},
			})
		}

		return locations
	}

	return []protocol.Location{}
}
