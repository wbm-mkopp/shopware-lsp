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

func TestCalculateBlockHash(t *testing.T) {
	content := `{% block test %}
    <p>Hello World</p>
{% endblock %}`

	hash := calculateBlockHash(content)

	assert.NotEmpty(t, hash)
	assert.Equal(t, hash, calculateBlockHash(content))

	otherContent := `{% block test %}
    <p>Different content</p>
{% endblock %}`

	otherHash := calculateBlockHash(otherContent)
	assert.NotEqual(t, hash, otherHash)
}

func TestParseVersionComment(t *testing.T) {
	tests := []struct {
		name     string
		comment  string
		line     int
		expected *TwigVersionComment
	}{
		{
			name:    "valid version comment",
			comment: "{# shopware-block: abc123def456@6.4.15.0 #}",
			line:    10,
			expected: &TwigVersionComment{
				Hash:    "abc123def456",
				Version: "6.4.15.0",
				Line:    10,
			},
		},
		{
			name:     "invalid comment",
			comment:  "{# just a regular comment #}",
			line:     5,
			expected: nil,
		},
		{
			name:     "malformed version comment",
			comment:  "{# shopware-block: abc123 #}",
			line:     8,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseVersionComment(tt.comment, tt.line)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Hash, result.Hash)
				assert.Equal(t, tt.expected.Version, result.Version)
				assert.Equal(t, tt.expected.Line, result.Line)
			}
		})
	}
}

func TestTwigBlockHashStructure(t *testing.T) {
	blockHash := TwigBlockHash{
		Name:         "test_block",
		RelativePath: "storefront/page/checkout/cart/index.html.twig",
		AbsolutePath: "/path/to/storefront/page/checkout/cart/index.html.twig",
		Hash:         "abc123def456",
		Text:         "{% block test_block %}content{% endblock %}",
	}

	assert.Equal(t, "test_block", blockHash.Name)
	assert.Equal(t, "storefront/page/checkout/cart/index.html.twig", blockHash.RelativePath)
	assert.Equal(t, "abc123def456", blockHash.Hash)
}

func TestHashCompatibilityWithPhpStorm(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		expectedHash string
	}{
		{
			name: "simple block",
			content: `{% block test %}
    <p>Hello World</p>
{% endblock %}`,
			expectedHash: "86eec44546f994424c37efba9b8f58be1ec94259be09302891e4a681c4b02918",
		},
		{
			name:         "inline block",
			content:      `{% block test_block %}content{% endblock %}`,
			expectedHash: "851bf4d9e13400b18923d9428b8f2d681f6f50b2d2f5c9bf0944ef09aa44a8d1",
		},
		{
			name: "block with nested html",
			content: `{% block test_block %}
    <p>Hello World</p>
{% endblock %}`,
			expectedHash: "fa4dc0f67b4ec5acf55b934657a181b9de40d3b5435e5b04f8ece858a9e8bff4",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hash := calculateBlockHash(tt.content)
			assert.Equal(t, tt.expectedHash, hash, "Hash should match PHPStorm plugin output")
			assert.Len(t, hash, 64, "SHA-256 hash should be 64 hex characters")
		})
	}
}

func TestFindBlockHashInTemplateFile_PriceUnitBlockWithHTML(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "price-unit.html.twig")

	content := []byte(`{% block component_product_box_price_info %}
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
	err := os.WriteFile(filePath, content, 0644)
	assert.NoError(t, err)

	hash, err := FindBlockHashInTemplateFile(filePath, "component_product_box_price_unit")
	assert.NoError(t, err)
	assert.NotNil(t, hash)
	assert.Equal(t, "component_product_box_price_unit", hash.Name)
	assert.Equal(t, filePath, hash.AbsolutePath)
	assert.Equal(t, ConvertToRelativePath(filePath), hash.RelativePath)
	assert.NotEmpty(t, hash.Hash)
	assert.Contains(t, hash.Text, "{% block component_product_box_price_unit %}")
}

func TestFindOriginalStorefrontHashForExtends_storePlugin(t *testing.T) {
	pluginHash := TwigBlockHash{
		Name:         "content",
		RelativePath: "@MyPlugin/storefront/page/foo.html.twig",
		AbsolutePath: "/project/vendor/store.shopware.com/my-plugin/src/Resources/views/storefront/page/foo.html.twig",
		Hash:         "pluginhash123",
	}
	overrideHash := TwigBlockHash{
		Name:         "content",
		RelativePath: "@Storefront/storefront/page/foo.html.twig",
		AbsolutePath: "/project/custom/plugins/MyOverride/src/Resources/views/storefront/page/foo.html.twig",
		Hash:         "overridehash456",
	}

	hashes := []TwigBlockHash{overrideHash, pluginHash}

	t.Run("matches extends path for store plugin", func(t *testing.T) {
		result := FindOriginalStorefrontHashForExtends(hashes, "@MyPlugin/storefront/page/foo.html.twig")
		require.NotNil(t, result)
		assert.Equal(t, "pluginhash123", result.Hash)
	})

	t.Run("matches extends path case-insensitively", func(t *testing.T) {
		result := FindOriginalStorefrontHashForExtends(hashes, "@myplugin/storefront/page/foo.html.twig")
		require.NotNil(t, result)
		assert.Equal(t, "pluginhash123", result.Hash)
	})

	t.Run("prefers store plugin over local override when extends is empty", func(t *testing.T) {
		result := FindOriginalStorefrontHash(hashes)
		require.NotNil(t, result)
		assert.Equal(t, "pluginhash123", result.Hash)
	})

	t.Run("prefers core storefront over store plugin when extends is empty", func(t *testing.T) {
		coreHash := TwigBlockHash{
			Name:         "content",
			RelativePath: "@Storefront/storefront/page/foo.html.twig",
			AbsolutePath: "/project/vendor/shopware/storefront/Resources/views/storefront/page/foo.html.twig",
			Hash:         "corehash789",
		}
		result := FindOriginalStorefrontHash([]TwigBlockHash{pluginHash, coreHash})
		require.NotNil(t, result)
		assert.Equal(t, "corehash789", result.Hash)
	})

	t.Run("prefers store plugin over core for legacy rel path when extends targets plugin", func(t *testing.T) {
		coreHash := TwigBlockHash{
			Name:         "content",
			RelativePath: "@Storefront/storefront/page/foo.html.twig",
			AbsolutePath: "/project/vendor/shopware/storefront/Resources/views/storefront/page/foo.html.twig",
			Hash:         "corehash789",
		}
		legacyPluginHash := TwigBlockHash{
			Name:         "content",
			RelativePath: "@Storefront/storefront/page/foo.html.twig",
			AbsolutePath: "/project/vendor/store.shopware.com/MyPlugin/src/Resources/views/storefront/page/foo.html.twig",
			Hash:         "pluginhash123",
		}

		result := FindOriginalStorefrontHashForExtends(
			[]TwigBlockHash{coreHash, legacyPluginHash},
			"@MyPlugin/storefront/page/foo.html.twig",
		)
		require.NotNil(t, result)
		assert.Equal(t, "pluginhash123", result.Hash)
	})
}

func TestIsStorefrontTemplate_includesStorePlugins(t *testing.T) {
	path := "/project/vendor/store.shopware.com/MyFoo/src/Resources/views/storefront/page/foo.html.twig"
	assert.True(t, IsStorefrontTemplate(path))
}

func TestResolveOriginalStorefrontHashForBlock_legacyStorefrontRelPath(t *testing.T) {
	tempDir := t.TempDir()
	idx, err := NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer idx.Close()

	pluginRoot := filepath.Join(tempDir, "vendor/store.shopware.com/swagcustomizedproducts")
	pluginPath := filepath.Join(pluginRoot, "src/Resources/views/storefront/component/buy-widget/buy-widget-form-customized-products.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(pluginPath), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginRoot, "composer.json"), []byte(`{
		"extra": {
			"shopware-plugin-class": "Swag\\CustomizedProducts\\SwagCustomizedProducts"
		}
	}`), 0644))
	pluginContent := []byte(`{% block swag_customized_products_configuration_share_card %}plugin{% endblock %}`)
	require.NoError(t, os.WriteFile(pluginPath, pluginContent, 0644))

	// Simulate a stale index that still uses the old @Storefront rel path.
	require.NoError(t, idx.IndexTwigFile(TwigFile{
		Path:    pluginPath,
		RelPath: "@Storefront/storefront/component/buy-widget/buy-widget-form-customized-products.html.twig",
	}))

	extendsFile := "@SwagCustomizedProducts/storefront/component/buy-widget/buy-widget-form-customized-products.html.twig"
	result := ResolveOriginalStorefrontHashForBlock(idx, "swag_customized_products_configuration_share_card", extendsFile)
	require.NotNil(t, result)
	assert.Equal(t, pluginPath, result.AbsolutePath)
	assert.NotEmpty(t, result.Hash)
}

func TestResolveOriginalStorefrontHashForBlock_unrelatedHashesDoNotBlockFallback(t *testing.T) {
	tempDir := t.TempDir()
	idx, err := NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer idx.Close()

	pluginRoot := filepath.Join(tempDir, "vendor/store.shopware.com/MyPlugin")
	pluginPath := filepath.Join(pluginRoot, "src/Resources/views/storefront/page/foo.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(pluginPath), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginRoot, "composer.json"), []byte(`{
		"extra": {
			"shopware-plugin-class": "MyPlugin\\MyPlugin"
		}
	}`), 0644))
	pluginContent := []byte(`{% block content %}plugin{% endblock %}`)
	require.NoError(t, os.WriteFile(pluginPath, pluginContent, 0644))

	parser := tree_sitter.NewParser()
	lang := tree_sitter.NewLanguage(tree_sitter_twig.Language())
	require.NoError(t, parser.SetLanguage(lang))
	defer parser.Close()

	pluginTree := parser.Parse(pluginContent, nil)
	defer pluginTree.Close()
	require.NoError(t, idx.Index(pluginPath, pluginTree.RootNode(), pluginContent))

	overridePath := filepath.Join(tempDir, "custom/plugins/MyOverride/src/Resources/views/storefront/page/foo.html.twig")
	overrideContent := []byte(`{% block content %}override{% endblock %}`)
	require.NoError(t, os.MkdirAll(filepath.Dir(overridePath), 0755))
	require.NoError(t, os.WriteFile(overridePath, overrideContent, 0644))
	overrideTree := parser.Parse(overrideContent, nil)
	defer overrideTree.Close()
	require.NoError(t, idx.Index(overridePath, overrideTree.RootNode(), overrideContent))

	extendsFile := "@MyPlugin/storefront/page/foo.html.twig"
	result := ResolveOriginalStorefrontHashForBlock(idx, "content", extendsFile)
	require.NotNil(t, result)
	assert.Equal(t, pluginPath, result.AbsolutePath)
}

func TestResolveOriginalStorefrontHashForBlock_wrongExtendsReturnsNil(t *testing.T) {
	tempDir := t.TempDir()
	idx, err := NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer idx.Close()

	overridePath := filepath.Join(tempDir, "custom/plugins/MyOverride/src/Resources/views/storefront/page/foo.html.twig")
	overrideContent := []byte(`{% block content %}override{% endblock %}`)
	require.NoError(t, os.MkdirAll(filepath.Dir(overridePath), 0755))
	require.NoError(t, os.WriteFile(overridePath, overrideContent, 0644))

	parser := tree_sitter.NewParser()
	lang := tree_sitter.NewLanguage(tree_sitter_twig.Language())
	require.NoError(t, parser.SetLanguage(lang))
	defer parser.Close()

	overrideTree := parser.Parse(overrideContent, nil)
	defer overrideTree.Close()
	require.NoError(t, idx.Index(overridePath, overrideTree.RootNode(), overrideContent))

	result := ResolveOriginalStorefrontHashForBlock(idx, "content", "@MissingPlugin/storefront/page/foo.html.twig")
	assert.Nil(t, result)
}

func TestGetTwigFilesByRelPathViewMatch_rejectsWrongBundle(t *testing.T) {
	tempDir := t.TempDir()
	idx, err := NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer idx.Close()

	pluginA := filepath.Join(tempDir, "vendor/store.shopware.com/PluginA/src/Resources/views/storefront/page/foo.html.twig")
	pluginB := filepath.Join(tempDir, "vendor/store.shopware.com/PluginB/src/Resources/views/storefront/page/foo.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(pluginA), 0755))
	require.NoError(t, os.MkdirAll(filepath.Dir(pluginB), 0755))

	require.NoError(t, idx.IndexTwigFile(TwigFile{
		Path:    pluginA,
		RelPath: "@PluginA/storefront/page/foo.html.twig",
	}))
	require.NoError(t, idx.IndexTwigFile(TwigFile{
		Path:    pluginB,
		RelPath: "@PluginB/storefront/page/foo.html.twig",
	}))

	files, err := idx.GetTwigFilesByRelPathViewMatch("@PluginA/storefront/page/foo.html.twig")
	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Equal(t, pluginA, files[0].Path)

	files, err = idx.GetTwigFilesByRelPathViewMatch("@WrongPlugin/storefront/page/foo.html.twig")
	require.NoError(t, err)
	assert.Empty(t, files)
}
