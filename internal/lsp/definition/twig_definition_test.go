package definition

import (
	"context"
	"fmt"
	"testing"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	"github.com/shopware/shopware-lsp/internal/twig"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestTwigBlockDefinition_GotoParentBlock(t *testing.T) {
	tempDir := t.TempDir()

	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()

	// Parent template with proper block structure (no HTML to avoid tree-sitter ERROR nodes)
	parentPath := "/project/Storefront/Resources/views/storefront/page/content/index.html.twig"
	parentContent := []byte("{% block content %}parent content{% endblock %}\n{% block sidebar %}sidebar content{% endblock %}")

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	parentTree := parser.Parse(parentContent, nil)
	defer parentTree.Close()

	require.NoError(t, twigIndexer.Index(parentPath, parentTree.RootNode(), parentContent))

	// Child template that extends the parent
	childPath := "/project/MyExtension/Resources/views/storefront/page/content/index.html.twig"
	childContent := []byte("{% sw_extends '@Storefront/storefront/page/content/index.html.twig' %}\n{% block content %}{{ parent() }}overridden{% endblock %}")

	childTree := parser.Parse(childContent, nil)
	defer childTree.Close()

	// Find the "content" identifier node inside the block tag
	contentNode := treesitterhelper.FindIdentifierNode(childTree.RootNode(), childContent, "content")
	require.NotNil(t, contentNode, "Could not find identifier node 'content'")
	assert.Equal(t, "block", contentNode.Parent().Kind(), "Parent of identifier should be a block node")

	provider := &TwigDefinitionProvider{
		twigIndexer: twigIndexer,
	}

	params := &protocol.DefinitionParams{
		DocumentContent: childContent,
		Node:            contentNode,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, childPath)

	locations := provider.GetDefinition(context.Background(), params)

	require.Len(t, locations, 1, "Should return exactly one location for the parent block")
	assert.Equal(t, fmt.Sprintf(lsp.FileURIFormat, parentPath), locations[0].URI)
	// Parent block "content" is on line 1 (1-indexed), so LSP line is 0
	assert.Equal(t, 0, locations[0].Range.Start.Line)
}

func TestTwigBlockDefinition_GotoParentBlock_MultilineBlocks(t *testing.T) {
	tempDir := t.TempDir()

	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()

	// Parent template with multiline blocks (text-only content for valid parsing)
	parentPath := "/project/Storefront/Resources/views/storefront/page/checkout/cart.html.twig"
	parentContent := []byte(`{% block page_content %}
    some page content
{% endblock %}

{% block page_sidebar %}
    sidebar content
{% endblock %}`)

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	parentTree := parser.Parse(parentContent, nil)
	defer parentTree.Close()

	require.NoError(t, twigIndexer.Index(parentPath, parentTree.RootNode(), parentContent))

	// Child extends parent and overrides page_sidebar
	childPath := "/project/MyExtension/Resources/views/storefront/page/checkout/cart.html.twig"
	childContent := []byte(`{% sw_extends '@Storefront/storefront/page/checkout/cart.html.twig' %}

{% block page_sidebar %}
    {{ parent() }}
    extra sidebar content
{% endblock %}`)

	childTree := parser.Parse(childContent, nil)
	defer childTree.Close()

	sidebarNode := treesitterhelper.FindIdentifierNode(childTree.RootNode(), childContent, "page_sidebar")
	require.NotNil(t, sidebarNode, "Could not find identifier node 'page_sidebar'")
	assert.Equal(t, "block", sidebarNode.Parent().Kind())

	provider := &TwigDefinitionProvider{
		twigIndexer: twigIndexer,
	}

	params := &protocol.DefinitionParams{
		DocumentContent: childContent,
		Node:            sidebarNode,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, childPath)

	locations := provider.GetDefinition(context.Background(), params)

	require.Len(t, locations, 1, "Should return exactly one location for the parent block")
	assert.Equal(t, fmt.Sprintf(lsp.FileURIFormat, parentPath), locations[0].URI)
	// Parent block "page_sidebar" is on line 5 (1-indexed), so LSP line is 4
	assert.Equal(t, 4, locations[0].Range.Start.Line)
}

func TestTwigBlockDefinition_ErrorNodeWithHTML(t *testing.T) {
	tempDir := t.TempDir()

	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()

	// Parent template (indexed with single-line block to ensure proper indexing)
	parentPath := "/project/Storefront/Resources/views/storefront/page/product/index.html.twig"
	parentContent := []byte("{% block product_detail %}product detail content{% endblock %}")

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	parentTree := parser.Parse(parentContent, nil)
	defer parentTree.Close()

	require.NoError(t, twigIndexer.Index(parentPath, parentTree.RootNode(), parentContent))

	// Child template with HTML inside block — tree-sitter produces ERROR node
	childPath := "/project/MyExtension/Resources/views/storefront/page/product/index.html.twig"
	childContent := []byte("{% sw_extends '@Storefront/storefront/page/product/index.html.twig' %}\n{% block product_detail %}\n    <div>custom detail</div>\n{% endblock %}")

	childTree := parser.Parse(childContent, nil)
	defer childTree.Close()

	productNode := treesitterhelper.FindIdentifierNode(childTree.RootNode(), childContent, "product_detail")
	require.NotNil(t, productNode, "Could not find identifier node 'product_detail'")
	// With HTML inside, tree-sitter produces an ERROR node instead of a block node
	assert.Equal(t, "ERROR", productNode.Parent().Kind(), "Parent should be ERROR due to HTML content")

	provider := &TwigDefinitionProvider{
		twigIndexer: twigIndexer,
	}

	params := &protocol.DefinitionParams{
		DocumentContent: childContent,
		Node:            productNode,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, childPath)

	locations := provider.GetDefinition(context.Background(), params)

	require.Len(t, locations, 1, "Should still resolve parent block even with ERROR node")
	assert.Equal(t, fmt.Sprintf(lsp.FileURIFormat, parentPath), locations[0].URI)
	assert.Equal(t, 0, locations[0].Range.Start.Line)
}

func TestTwigBlockDefinition_PrefersStorefrontVendor(t *testing.T) {
	tempDir := t.TempDir()

	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	// Index a plugin template that overrides the same block
	pluginPath := "/project/custom/plugins/PluginB/Resources/views/storefront/page/content/index.html.twig"
	pluginContent := []byte("{% sw_extends '@Storefront/storefront/page/content/index.html.twig' %}\n{% block content %}plugin B content{% endblock %}")
	pluginTree := parser.Parse(pluginContent, nil)
	defer pluginTree.Close()
	require.NoError(t, twigIndexer.Index(pluginPath, pluginTree.RootNode(), pluginContent))

	// Index the Storefront vendor template (the canonical original)
	storefrontPath := "/project/vendor/shopware/storefront/Resources/views/storefront/page/content/index.html.twig"
	storefrontContent := []byte("{% block content %}original storefront content{% endblock %}")
	storefrontTree := parser.Parse(storefrontContent, nil)
	defer storefrontTree.Close()
	require.NoError(t, twigIndexer.Index(storefrontPath, storefrontTree.RootNode(), storefrontContent))

	// Child template from PluginA extending the same relPath
	childPath := "/project/custom/plugins/PluginA/Resources/views/storefront/page/content/index.html.twig"
	childContent := []byte("{% sw_extends '@Storefront/storefront/page/content/index.html.twig' %}\n{% block content %}{{ parent() }}plugin A content{% endblock %}")
	childTree := parser.Parse(childContent, nil)
	defer childTree.Close()

	contentNode := treesitterhelper.FindIdentifierNode(childTree.RootNode(), childContent, "content")
	require.NotNil(t, contentNode)

	provider := &TwigDefinitionProvider{
		twigIndexer: twigIndexer,
	}

	params := &protocol.DefinitionParams{
		DocumentContent: childContent,
		Node:            contentNode,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, childPath)

	locations := provider.GetDefinition(context.Background(), params)

	require.Len(t, locations, 1, "Should return exactly one location")
	// Must prefer the Storefront vendor template, not PluginB
	assert.Equal(t, fmt.Sprintf(lsp.FileURIFormat, storefrontPath), locations[0].URI,
		"Go-to-Definition should navigate to the Storefront vendor template, not another plugin")
}

// TestTwigBlockDefinition_CrossTemplateStorefrontBlock reproduces the real-world
// scenario where a block is DEFINED in one Storefront template (e.g.
// box-standard.html.twig) but OVERRIDDEN in a plugin file that extends a
// different template (e.g. price-unit.html.twig). Go-to-Definition must navigate
// to the Storefront template where the block is actually defined, using the block
// hash index.
func TestTwigBlockDefinition_CrossTemplateStorefrontBlock(t *testing.T) {
	tempDir := t.TempDir()

	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	// The Storefront vendor template where the block is ACTUALLY defined.
	// This is a DIFFERENT template from what the plugin files extend.
	boxStandardPath := "/project/vendor/shopware/storefront/Resources/views/storefront/component/product/card/box-standard.html.twig"
	boxStandardContent := []byte("{% block component_product_box_price %}original price{% endblock %}")
	boxStandardTree := parser.Parse(boxStandardContent, nil)
	defer boxStandardTree.Close()
	require.NoError(t, twigIndexer.Index(boxStandardPath, boxStandardTree.RootNode(), boxStandardContent))

	// The Storefront's price-unit.html.twig — exists but does NOT have this block.
	storefrontPriceUnitPath := "/project/vendor/shopware/storefront/Resources/views/storefront/component/product/card/price-unit.html.twig"
	storefrontPriceUnitContent := []byte("{% block component_product_box_price_unit %}unit price{% endblock %}")
	storefrontPriceUnitTree := parser.Parse(storefrontPriceUnitContent, nil)
	defer storefrontPriceUnitTree.Close()
	require.NoError(t, twigIndexer.Index(storefrontPriceUnitPath, storefrontPriceUnitTree.RootNode(), storefrontPriceUnitContent))

	// Another plugin that also overrides this block in price-unit.html.twig
	merchantPath := "/project/custom/static-plugins/MerchantPlugin/src/Resources/views/storefront/component/product/card/price-unit.html.twig"
	merchantContent := []byte("{% sw_extends '@Storefront/storefront/component/product/card/price-unit.html.twig' %}\n{% block component_product_box_price %}merchant content{% endblock %}")
	merchantTree := parser.Parse(merchantContent, nil)
	defer merchantTree.Close()
	require.NoError(t, twigIndexer.Index(merchantPath, merchantTree.RootNode(), merchantContent))

	// Current file: theme plugin extending price-unit.html.twig
	themePath := "/project/custom/static-plugins/ThemePlugin/src/Resources/views/storefront/component/product/card/price-unit.html.twig"
	themeContent := []byte("{% sw_extends '@Storefront/storefront/component/product/card/price-unit.html.twig' %}\n{% block component_product_box_price %}theme content{% endblock %}")
	themeTree := parser.Parse(themeContent, nil)
	defer themeTree.Close()

	blockNode := treesitterhelper.FindIdentifierNode(themeTree.RootNode(), themeContent, "component_product_box_price")
	require.NotNil(t, blockNode)

	provider := &TwigDefinitionProvider{
		twigIndexer: twigIndexer,
	}

	params := &protocol.DefinitionParams{
		DocumentContent: themeContent,
		Node:            blockNode,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, themePath)

	locations := provider.GetDefinition(context.Background(), params)

	require.Len(t, locations, 1, "Should return exactly one location")
	// Must navigate to box-standard.html.twig (where block is DEFINED),
	// not to merchant's price-unit.html.twig or storefront's price-unit.html.twig.
	assert.Equal(t, fmt.Sprintf(lsp.FileURIFormat, boxStandardPath), locations[0].URI,
		"Go-to-Definition should navigate to the Storefront template where the block is actually defined")
	assert.Equal(t, 0, locations[0].Range.Start.Line)
}

func TestTwigBlockDefinition_NoExtends(t *testing.T) {
	tempDir := t.TempDir()

	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()

	// Template without extends — just standalone blocks
	content := []byte("{% block content %}standalone content{% endblock %}")

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	tree := parser.Parse(content, nil)
	defer tree.Close()

	contentNode := treesitterhelper.FindIdentifierNode(tree.RootNode(), content, "content")
	require.NotNil(t, contentNode)

	provider := &TwigDefinitionProvider{
		twigIndexer: twigIndexer,
	}

	params := &protocol.DefinitionParams{
		DocumentContent: content,
		Node:            contentNode,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, "/project/Storefront/Resources/views/storefront/page/foo.html.twig")

	locations := provider.GetDefinition(context.Background(), params)

	assert.Empty(t, locations, "Should return no locations when file has no extends")
}

func TestTwigBlockDefinition_BlockNotInParent(t *testing.T) {
	tempDir := t.TempDir()

	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()

	// Parent template with only "sidebar" block
	parentPath := "/project/Storefront/Resources/views/storefront/page/checkout/cart.html.twig"
	parentContent := []byte("{% block sidebar %}sidebar content{% endblock %}")

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	parentTree := parser.Parse(parentContent, nil)
	defer parentTree.Close()

	require.NoError(t, twigIndexer.Index(parentPath, parentTree.RootNode(), parentContent))

	// Child template that extends parent but has a block not present in parent
	childPath := "/project/MyExtension/Resources/views/storefront/page/checkout/cart.html.twig"
	childContent := []byte("{% sw_extends '@Storefront/storefront/page/checkout/cart.html.twig' %}\n{% block new_custom_block %}custom content{% endblock %}")

	childTree := parser.Parse(childContent, nil)
	defer childTree.Close()

	customBlockNode := treesitterhelper.FindIdentifierNode(childTree.RootNode(), childContent, "new_custom_block")
	require.NotNil(t, customBlockNode)

	provider := &TwigDefinitionProvider{
		twigIndexer: twigIndexer,
	}

	params := &protocol.DefinitionParams{
		DocumentContent: childContent,
		Node:            customBlockNode,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, childPath)

	locations := provider.GetDefinition(context.Background(), params)

	assert.Empty(t, locations, "Should return no locations when block does not exist in parent")
}
