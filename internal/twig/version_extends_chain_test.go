package twig

import (
	"os"
	"path/filepath"
	"testing"

	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func indexTwigFile(t *testing.T, idx *TwigIndexer, path string, content []byte) {
	t.Helper()

	parser := tree_sitter.NewParser()
	lang := tree_sitter.NewLanguage(tree_sitter_twig.Language())
	require.NoError(t, parser.SetLanguage(lang))
	defer parser.Close()

	tree := parser.Parse(content, nil)
	defer tree.Close()
	require.NoError(t, idx.Index(path, tree.RootNode(), content))
}

func TestResolveOriginalStorefrontHashForBlock_extendsChainThroughPlugin(t *testing.T) {
	tempDir := t.TempDir()
	idx, err := NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer idx.Close()

	storefrontPath := filepath.Join(tempDir, "vendor/shopware/storefront/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	pluginRoot := filepath.Join(tempDir, "vendor/store.shopware.com/swagcustomizedproducts")
	pluginPath := filepath.Join(pluginRoot, "src/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")

	require.NoError(t, os.MkdirAll(filepath.Dir(storefrontPath), 0755))
	require.NoError(t, os.MkdirAll(filepath.Dir(pluginPath), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginRoot, "composer.json"), []byte(`{
		"extra": {
			"shopware-plugin-class": "Swag\\CustomizedProducts\\SwagCustomizedProducts"
		}
	}`), 0644))

	storefrontContent := []byte(`{% block buy_widget_ordernumber_container %}
    <div class="ordernumber">storefront</div>
{% endblock %}

{% block buy_widget_tax %}
    <div class="tax">storefront tax</div>
{% endblock %}`)
	pluginContent := []byte(`{% sw_extends '@Storefront/storefront/component/buy-widget/buy-widget.html.twig' %}

{% block buy_widget_tax %}
    <div class="tax">plugin tax</div>
{% endblock %}`)

	require.NoError(t, os.WriteFile(storefrontPath, storefrontContent, 0644))
	require.NoError(t, os.WriteFile(pluginPath, pluginContent, 0644))

	indexTwigFile(t, idx, storefrontPath, storefrontContent)
	indexTwigFile(t, idx, pluginPath, pluginContent)

	extendsFile := "@SwagCustomizedProducts/storefront/component/buy-widget/buy-widget.html.twig"

	t.Run("block only in storefront is resolved via plugin extends chain", func(t *testing.T) {
		result := ResolveOriginalStorefrontHashForBlock(idx, "buy_widget_ordernumber_container", extendsFile)
		require.NotNil(t, result)
		assert.Equal(t, storefrontPath, result.AbsolutePath)
	})

	t.Run("block redefined in plugin resolves to plugin template", func(t *testing.T) {
		result := ResolveOriginalStorefrontHashForBlock(idx, "buy_widget_tax", extendsFile)
		require.NotNil(t, result)
		assert.Equal(t, pluginPath, result.AbsolutePath)
	})
}
