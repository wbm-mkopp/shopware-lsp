package diagnostics

import (
	"context"
	"fmt"
	"os"
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

func TestTwigVersioningDiagnosticsProvider_storePluginMissingVersionComment(t *testing.T) {
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

	pluginRoot := filepath.Join(tempDir, "vendor/store.shopware.com/MyPlugin")
	pluginPath := filepath.Join(pluginRoot, "src/Resources/views/storefront/page/foo.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(pluginPath), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginRoot, "composer.json"), []byte(`{
		"extra": {
			"shopware-plugin-class": "MyPlugin\\MyPlugin"
		}
	}`), 0644))
	pluginContent := []byte(`{% block content %}plugin content{% endblock %}`)
	require.NoError(t, os.WriteFile(pluginPath, pluginContent, 0644))
	pluginTree := parser.Parse(pluginContent, nil)
	defer pluginTree.Close()
	require.NoError(t, twigIndexer.Index(pluginPath, pluginTree.RootNode(), pluginContent))

	overridePath := filepath.Join(tempDir, "custom/plugins/MyOverride/src/Resources/views/storefront/page/foo.html.twig")
	overrideURI := fmt.Sprintf(lsp.FileURIFormat, overridePath)
	overrideContent := []byte(`{% sw_extends '@MyPlugin/storefront/page/foo.html.twig' %}
{% block content %}override content{% endblock %}`)
	overrideTree := parser.Parse(overrideContent, nil)
	defer overrideTree.Close()

	diagnostics, err := provider.GetDiagnostics(ctx, overrideURI, overrideTree.RootNode(), overrideContent)
	require.NoError(t, err)

	require.Len(t, diagnostics, 1)
	assert.Contains(t, diagnostics[0].Message, "does not have a versioning comment")
	assert.Equal(t, protocol.DiagnosticSeverityWarning, diagnostics[0].Severity)
	assert.Equal(t, 1, diagnostics[0].Range.Start.Line)
	assert.Equal(t, 0, diagnostics[0].Range.Start.Character)
	assert.Greater(t, diagnostics[0].Range.End.Character, 0)

	originalHash := twig.ResolveOriginalStorefrontHashForBlock(twigIndexer, "content", "@MyPlugin/storefront/page/foo.html.twig")
	require.NotNil(t, originalHash)
	assert.Equal(t, pluginPath, originalHash.AbsolutePath)
	assert.Equal(t, "@MyPlugin/storefront/page/foo.html.twig", originalHash.RelativePath)
}

func TestTwigVersioningDiagnosticsProvider_storePluginExtendsStorefrontBlock(t *testing.T) {
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

	storefrontPath := filepath.Join(tempDir, "vendor/shopware/storefront/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	pluginRoot := filepath.Join(tempDir, "vendor/store.shopware.com/swagcustomizedproducts")
	pluginPath := filepath.Join(pluginRoot, "src/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	overridePath := filepath.Join(tempDir, "src/WbmAidaCore/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")

	require.NoError(t, os.MkdirAll(filepath.Dir(storefrontPath), 0755))
	require.NoError(t, os.MkdirAll(filepath.Dir(pluginPath), 0755))
	require.NoError(t, os.MkdirAll(filepath.Dir(overridePath), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginRoot, "composer.json"), []byte(`{
		"extra": {
			"shopware-plugin-class": "Swag\\CustomizedProducts\\SwagCustomizedProducts"
		}
	}`), 0644))

	storefrontContent := []byte(`{% block buy_widget_ordernumber_container %}
    <div class="ordernumber">storefront</div>
{% endblock %}`)
	pluginContent := []byte(`{% sw_extends '@Storefront/storefront/component/buy-widget/buy-widget.html.twig' %}

{% block buy_widget_tax %}
    plugin tax
{% endblock %}`)
	overrideContent := []byte(`{% sw_extends '@SwagCustomizedProducts/storefront/component/buy-widget/buy-widget.html.twig' %}

{% block buy_widget_ordernumber_container %}
    custom ordernumber
{% endblock %}`)

	require.NoError(t, os.WriteFile(storefrontPath, storefrontContent, 0644))
	require.NoError(t, os.WriteFile(pluginPath, pluginContent, 0644))

	storefrontTree := parser.Parse(storefrontContent, nil)
	defer storefrontTree.Close()
	require.NoError(t, twigIndexer.Index(storefrontPath, storefrontTree.RootNode(), storefrontContent))

	pluginTree := parser.Parse(pluginContent, nil)
	defer pluginTree.Close()
	require.NoError(t, twigIndexer.Index(pluginPath, pluginTree.RootNode(), pluginContent))

	overrideURI := fmt.Sprintf(lsp.FileURIFormat, overridePath)
	overrideTree := parser.Parse(overrideContent, nil)
	defer overrideTree.Close()

	diagnostics, err := provider.GetDiagnostics(ctx, overrideURI, overrideTree.RootNode(), overrideContent)
	require.NoError(t, err)

	for _, diag := range diagnostics {
		assert.False(
			t,
			strings.Contains(diag.Message, "Original block not found in Storefront for block 'buy_widget_ordernumber_container'"),
			"storefront block reached via plugin extends chain should be resolvable, got: %s",
			diag.Message,
		)
	}

	require.Len(t, diagnostics, 1)
	assert.Contains(t, diagnostics[0].Message, "does not have a versioning comment")
}
