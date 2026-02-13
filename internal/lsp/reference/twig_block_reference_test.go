package reference

import (
	"context"
	"fmt"
	"sort"
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

func TestTwigBlockReference_FindsAllParentBlocks(t *testing.T) {
	tempDir := t.TempDir()

	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	// Index the Storefront vendor template
	storefrontPath := "/project/vendor/shopware/storefront/Resources/views/storefront/page/content/index.html.twig"
	storefrontContent := []byte("{% block content %}original storefront content{% endblock %}")
	storefrontTree := parser.Parse(storefrontContent, nil)
	defer storefrontTree.Close()
	require.NoError(t, twigIndexer.Index(storefrontPath, storefrontTree.RootNode(), storefrontContent))

	// Index PluginB template
	pluginBPath := "/project/custom/plugins/PluginB/Resources/views/storefront/page/content/index.html.twig"
	pluginBContent := []byte("{% sw_extends '@Storefront/storefront/page/content/index.html.twig' %}\n{% block content %}plugin B content{% endblock %}")
	pluginBTree := parser.Parse(pluginBContent, nil)
	defer pluginBTree.Close()
	require.NoError(t, twigIndexer.Index(pluginBPath, pluginBTree.RootNode(), pluginBContent))

	// Index PluginC template
	pluginCPath := "/project/custom/plugins/PluginC/Resources/views/storefront/page/content/index.html.twig"
	pluginCContent := []byte("{% sw_extends '@Storefront/storefront/page/content/index.html.twig' %}\n{% block content %}plugin C content{% endblock %}")
	pluginCTree := parser.Parse(pluginCContent, nil)
	defer pluginCTree.Close()
	require.NoError(t, twigIndexer.Index(pluginCPath, pluginCTree.RootNode(), pluginCContent))

	// Current file: PluginA extending the same template
	pluginAPath := "/project/custom/plugins/PluginA/Resources/views/storefront/page/content/index.html.twig"
	pluginAContent := []byte("{% sw_extends '@Storefront/storefront/page/content/index.html.twig' %}\n{% block content %}{{ parent() }}plugin A content{% endblock %}")
	pluginATree := parser.Parse(pluginAContent, nil)
	defer pluginATree.Close()

	contentNode := treesitterhelper.FindIdentifierNode(pluginATree.RootNode(), pluginAContent, "content")
	require.NotNil(t, contentNode)

	provider := &TwigBlockReferenceProvider{
		twigIndexer: twigIndexer,
	}

	params := &protocol.ReferenceParams{
		DocumentContent: pluginAContent,
		Node:            contentNode,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, pluginAPath)

	locations := provider.GetReferences(context.Background(), params)

	// Should find Storefront, PluginB, and PluginC (not PluginA itself)
	require.Len(t, locations, 3, "Should find all parent blocks except the current file")

	// Collect URIs for assertion
	uris := make([]string, len(locations))
	for i, loc := range locations {
		uris[i] = loc.URI
	}
	sort.Strings(uris)

	assert.Contains(t, uris, fmt.Sprintf(lsp.FileURIFormat, storefrontPath), "Should include Storefront vendor template")
	assert.Contains(t, uris, fmt.Sprintf(lsp.FileURIFormat, pluginBPath), "Should include PluginB")
	assert.Contains(t, uris, fmt.Sprintf(lsp.FileURIFormat, pluginCPath), "Should include PluginC")
	assert.NotContains(t, uris, fmt.Sprintf(lsp.FileURIFormat, pluginAPath), "Should not include the current file")
}

func TestTwigBlockReference_NoExtends(t *testing.T) {
	tempDir := t.TempDir()

	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()

	// Standalone template with no extends
	content := []byte("{% block content %}standalone content{% endblock %}")

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	tree := parser.Parse(content, nil)
	defer tree.Close()

	contentNode := treesitterhelper.FindIdentifierNode(tree.RootNode(), content, "content")
	require.NotNil(t, contentNode)

	provider := &TwigBlockReferenceProvider{
		twigIndexer: twigIndexer,
	}

	params := &protocol.ReferenceParams{
		DocumentContent: content,
		Node:            contentNode,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, "/project/Storefront/Resources/views/storefront/page/foo.html.twig")

	locations := provider.GetReferences(context.Background(), params)

	assert.Empty(t, locations, "Should return no references for a standalone template with no other files")
}

func TestTwigBlockReference_BlockNotInOtherFiles(t *testing.T) {
	tempDir := t.TempDir()

	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	// Parent template has only "sidebar" block
	parentPath := "/project/vendor/shopware/storefront/Resources/views/storefront/page/checkout/cart.html.twig"
	parentContent := []byte("{% block sidebar %}sidebar content{% endblock %}")
	parentTree := parser.Parse(parentContent, nil)
	defer parentTree.Close()
	require.NoError(t, twigIndexer.Index(parentPath, parentTree.RootNode(), parentContent))

	// Child has a custom block that doesn't exist in parent
	childPath := "/project/custom/plugins/PluginA/Resources/views/storefront/page/checkout/cart.html.twig"
	childContent := []byte("{% sw_extends '@Storefront/storefront/page/checkout/cart.html.twig' %}\n{% block my_custom_block %}custom content{% endblock %}")
	childTree := parser.Parse(childContent, nil)
	defer childTree.Close()

	customNode := treesitterhelper.FindIdentifierNode(childTree.RootNode(), childContent, "my_custom_block")
	require.NotNil(t, customNode)

	provider := &TwigBlockReferenceProvider{
		twigIndexer: twigIndexer,
	}

	params := &protocol.ReferenceParams{
		DocumentContent: childContent,
		Node:            customNode,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, childPath)

	locations := provider.GetReferences(context.Background(), params)

	assert.Empty(t, locations, "Should return no references when block is unique to current file")
}

func TestTwigBlockReference_NilNodeReturnsNil(t *testing.T) {
	tempDir := t.TempDir()

	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()

	provider := &TwigBlockReferenceProvider{
		twigIndexer: twigIndexer,
	}

	// params.Node is nil when no symbol is under the cursor
	params := &protocol.ReferenceParams{}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, "/project/src/Controller/MyController.php")

	locations := provider.GetReferences(context.Background(), params)
	assert.Nil(t, locations, "Should return nil when params.Node is nil")
}

// TestTwigBlockReference_CrossTemplateStorefrontBlock verifies that Find All
// References includes the original Storefront template where a block is defined,
// even when it lives under a different relPath than the ExtendsFile.
func TestTwigBlockReference_CrossTemplateStorefrontBlock(t *testing.T) {
	tempDir := t.TempDir()

	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	// Storefront template where the block is ACTUALLY defined (different template)
	boxStandardPath := "/project/vendor/shopware/storefront/Resources/views/storefront/component/product/card/box-standard.html.twig"
	boxStandardContent := []byte("{% block component_product_box_price %}original price{% endblock %}")
	boxStandardTree := parser.Parse(boxStandardContent, nil)
	defer boxStandardTree.Close()
	require.NoError(t, twigIndexer.Index(boxStandardPath, boxStandardTree.RootNode(), boxStandardContent))

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

	provider := &TwigBlockReferenceProvider{
		twigIndexer: twigIndexer,
	}

	params := &protocol.ReferenceParams{
		DocumentContent: themeContent,
		Node:            blockNode,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, themePath)

	locations := provider.GetReferences(context.Background(), params)

	// Should find both the merchant plugin AND the original Storefront template
	require.Len(t, locations, 2, "Should find merchant plugin and original Storefront template")

	uris := make([]string, len(locations))
	for i, loc := range locations {
		uris[i] = loc.URI
	}
	sort.Strings(uris)

	assert.Contains(t, uris, fmt.Sprintf(lsp.FileURIFormat, boxStandardPath),
		"Should include the Storefront template where the block is actually defined")
	assert.Contains(t, uris, fmt.Sprintf(lsp.FileURIFormat, merchantPath),
		"Should include other plugins that override the block")
	assert.NotContains(t, uris, fmt.Sprintf(lsp.FileURIFormat, themePath),
		"Should not include the current file")
}

func TestTwigBlockReference_OriginalTemplateFindsExtensions(t *testing.T) {
	tempDir := t.TempDir()

	twigIndexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	// Index the Storefront vendor template (no extends — this IS the original)
	storefrontPath := "/project/vendor/shopware/storefront/Resources/views/storefront/page/content/index.html.twig"
	storefrontContent := []byte("{% block content %}original content{% endblock %}")
	storefrontTree := parser.Parse(storefrontContent, nil)
	defer storefrontTree.Close()
	require.NoError(t, twigIndexer.Index(storefrontPath, storefrontTree.RootNode(), storefrontContent))

	// Index a plugin extension
	pluginPath := "/project/custom/plugins/PluginA/Resources/views/storefront/page/content/index.html.twig"
	pluginContent := []byte("{% sw_extends '@Storefront/storefront/page/content/index.html.twig' %}\n{% block content %}plugin content{% endblock %}")
	pluginTree := parser.Parse(pluginContent, nil)
	defer pluginTree.Close()
	require.NoError(t, twigIndexer.Index(pluginPath, pluginTree.RootNode(), pluginContent))

	// Find references from the Storefront template's block
	contentNode := treesitterhelper.FindIdentifierNode(storefrontTree.RootNode(), storefrontContent, "content")
	require.NotNil(t, contentNode)

	provider := &TwigBlockReferenceProvider{
		twigIndexer: twigIndexer,
	}

	params := &protocol.ReferenceParams{
		DocumentContent: storefrontContent,
		Node:            contentNode,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, storefrontPath)

	locations := provider.GetReferences(context.Background(), params)

	// From the original, we should find the plugin extension
	require.Len(t, locations, 1, "Should find plugin extension from original template")
	assert.Equal(t, fmt.Sprintf(lsp.FileURIFormat, pluginPath), locations[0].URI)
}
