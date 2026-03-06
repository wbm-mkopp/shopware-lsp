package diagnostics

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shopware/shopware-lsp/internal/indexer"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	"github.com/shopware/shopware-lsp/internal/twig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestTwigVersioningDiagnosticsProvider_originalNotFoundMessage(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	fileScanner, err := indexer.NewFileScanner(tempDir, filepath.Join(tempDir, "scanner.db"))
	require.NoError(t, err)

	server := lsp.NewServer(fileScanner, tempDir, "test")
	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	server.RegisterIndexer(twigIndexer, nil)

	provider := NewTwigVersioningDiagnosticsProvider(server)

	uri := "file:///tmp/myext/Resources/views/storefront/page/checkout/foo.html.twig"
	content := []byte(`{% sw_extends '@Storefront/storefront/page/checkout/foo' %}{# shopware-block: abc123def456@6.4.15.0 #}{% block content %}test{% endblock %}`)

	parser := tree_sitter.NewParser()
	lang := tree_sitter.NewLanguage(tree_sitter_twig.Language())
	require.NoError(t, parser.SetLanguage(lang))
	tree := parser.Parse(content, nil)
	defer tree.Close()

	diagnostics, err := provider.GetDiagnostics(ctx, uri, tree.RootNode(), content)
	require.NoError(t, err)

	require.Len(t, diagnostics, 1)
	assert.Contains(t, diagnostics[0].Message, "Original block not found in Storefront for block 'content'")
	assert.Equal(t, protocol.DiagnosticSeverityWarning, diagnostics[0].Severity)
	assert.Equal(t, "shopware-lsp", diagnostics[0].Source)
}

func TestTwigVersioningDiagnosticsProvider_nilIndexerNoPanic(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	fileScanner, err := indexer.NewFileScanner(tempDir, filepath.Join(tempDir, "scanner.db"))
	require.NoError(t, err)

	server := lsp.NewServer(fileScanner, tempDir, "test")

	provider := NewTwigVersioningDiagnosticsProvider(server)
	require.NotNil(t, provider)

	content := []byte(`{% block foo %}{% endblock %}`)
	uri := "file:///tmp/ext/Resources/views/storefront/page/bar.html.twig"
	parser := tree_sitter.NewParser()
	lang := tree_sitter.NewLanguage(tree_sitter_twig.Language())
	require.NoError(t, parser.SetLanguage(lang))
	tree := parser.Parse(content, nil)
	defer tree.Close()

	diagnostics, err := provider.GetDiagnostics(ctx, uri, tree.RootNode(), content)
	require.NoError(t, err)
	assert.Empty(t, diagnostics)
}

func TestTwigVersioningDiagnosticsProvider_PriceUnitParentBlockFoundViaFallback(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	fileScanner, err := indexer.NewFileScanner(tempDir, filepath.Join(tempDir, "scanner.db"))
	require.NoError(t, err)

	server := lsp.NewServer(fileScanner, tempDir, "test")
	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	server.RegisterIndexer(twigIndexer, nil)

	provider := NewTwigVersioningDiagnosticsProvider(server)

	parser := tree_sitter.NewParser()
	lang := tree_sitter.NewLanguage(tree_sitter_twig.Language())
	require.NoError(t, parser.SetLanguage(lang))
	defer parser.Close()

	// Vendor storefront template based on the reported real-world case:
	// component_product_box_price_unit may not be indexed via regular hash lookup.
	vendorPath := "/tmp/project/vendor/shopware/storefront/Resources/views/storefront/component/product/card/price-unit.html.twig"
	vendorContent := []byte(`{% block component_product_box_price_info %}
    <div class="product-price-info">
        {% block component_product_box_price_unit %}
            <p class="product-price-unit">
                {% block component_product_box_price_purchase_unit %}
                    {% if referencePrice and referencePrice.unitName %}
                        <span class="product-unit-label"></span>
                    {% endif %}
                {% endblock %}
            </p>
        {% endblock %}
    </div>
{% endblock %}`)
	vendorTree := parser.Parse(vendorContent, nil)
	defer vendorTree.Close()
	require.NoError(t, twigIndexer.Index(vendorPath, vendorTree.RootNode(), vendorContent))

	// Extension override modeled after Aida's price-unit override.
	overridePath := "/tmp/project/src/WbmAidaCore/Resources/views/storefront/component/product/card/price-unit.html.twig"
	overrideURI := fmt.Sprintf(lsp.FileURIFormat, overridePath)
	overrideContent := []byte(`{% sw_extends '@Storefront/storefront/component/product/card/price-unit.html.twig' %}

{% block component_product_box_price_info %}
    <div class="product-price-info">
        {{ block('component_product_box_price') }}
        {{ block('component_product_box_price_unit') }}
    </div>
{% endblock %}

{% block component_product_box_price_unit %}
    {% if referencePrice and referencePrice.unitName %}
        {{ parent() }}
    {% endif %}
{% endblock %}`)
	overrideTree := parser.Parse(overrideContent, nil)
	defer overrideTree.Close()

	diagnostics, err := provider.GetDiagnostics(ctx, overrideURI, overrideTree.RootNode(), overrideContent)
	require.NoError(t, err)

	for _, diag := range diagnostics {
		assert.False(
			t,
			strings.Contains(diag.Message, "Original block not found in Storefront for block 'component_product_box_price_unit'"),
			"price-unit parent block should be resolvable, got diagnostic: %s",
			diag.Message,
		)
	}
}
