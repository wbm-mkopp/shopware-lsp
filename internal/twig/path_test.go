package twig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertToRelativePath(t *testing.T) {
	assert.Equal(t, "", ConvertToRelativePath(""))
	assert.Equal(t, "", ConvertToRelativePath("/"))
	assert.Equal(t, "", ConvertToRelativePath("/Resources/views"))
	assert.Equal(t, "", ConvertToRelativePath("/Resources/views/"))
	assert.Equal(t, "@Storefront/storefront/base.html.twig", ConvertToRelativePath("/Resources/views/storefront/base.html.twig"))
	assert.Equal(
		t,
		"@Storefront/storefront/base.html.twig",
		ConvertToRelativePath("/project/vendor/shopware/storefront/Resources/views/storefront/base.html.twig"),
	)
	assert.Equal(
		t,
		"@MyFoo/storefront/page/foo.html.twig",
		ConvertToRelativePath("/project/vendor/store.shopware.com/MyFoo/src/Resources/views/storefront/page/foo.html.twig"),
	)
}

func TestConvertToRelativePath_storePluginUsesFolderFallbackWithoutComposer(t *testing.T) {
	tempDir := t.TempDir()
	pluginRoot := filepath.Join(tempDir, "vendor/store.shopware.com/MyFoo")
	twigPath := filepath.Join(pluginRoot, "src/Resources/views/storefront/page/foo.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(twigPath), 0755))
	_, err := os.Stat(filepath.Join(pluginRoot, "composer.json"))
	require.ErrorIs(t, err, os.ErrNotExist)
	assert.Equal(t, "@MyFoo/storefront/page/foo.html.twig", ConvertToRelativePath(twigPath))
}

func TestConvertToRelativePath_storePluginUsesComposerClassName(t *testing.T) {
	tempDir := t.TempDir()
	pluginRoot := filepath.Join(tempDir, "vendor/store.shopware.com/swagcustomizedproducts")
	require.NoError(t, os.MkdirAll(filepath.Join(pluginRoot, "src/Resources/views/storefront/page"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(pluginRoot, "composer.json"), []byte(`{
		"extra": {
			"shopware-plugin-class": "Swag\\CustomizedProducts\\SwagCustomizedProducts"
		}
	}`), 0644))

	twigPath := filepath.Join(pluginRoot, "src/Resources/views/storefront/page/foo.html.twig")
	assert.Equal(t, "@SwagCustomizedProducts/storefront/page/foo.html.twig", ConvertToRelativePath(twigPath))
}

func TestTwigRelPathsEquivalent(t *testing.T) {
	assert.True(t, twigRelPathsEquivalent(
		"@SwagCustomizedProducts/storefront/page/foo.html.twig",
		"@swagcustomizedproducts/storefront/page/foo.html.twig",
	))
	assert.False(t, twigRelPathsEquivalent(
		"@SwagCustomizedProducts/storefront/page/foo.html.twig",
		"@OtherPlugin/storefront/page/foo.html.twig",
	))
}

func TestTwigViewPathsMatch(t *testing.T) {
	assert.True(t, twigViewPathsMatch(
		"storefront/component/buy-widget/buy-widget-form-customized-products.html.twig",
		"storefront/component/buy-widget/buy-widget-form-customized-products.html.twig",
	))
	assert.True(t, twigViewPathsMatch(
		"storefront/component/buy-widget/buy-widget-form-customized-products.html.twig",
		"storefront/COMPONENT/buy-widget/buy-widget-form-customized-products.html.twig",
	))
	assert.False(t, twigViewPathsMatch(
		"storefront/page/foo.html.twig",
		"storefront/page/bar.html.twig",
	))
}

func TestTwigRelPathLookupKeys(t *testing.T) {
	relPath := "@SwagCustomizedProducts/storefront/page/foo.html.twig"
	assert.Equal(t, "@swagcustomizedproducts/storefront/page/foo.html.twig", twigRelPathLookupKey(relPath))
	assert.Equal(t, "view:storefront/page/foo.html.twig", twigRelPathViewLookupKey(relPath))
}

func TestGetBundleNameByPath(t *testing.T) {
	assert.Equal(t, "foo", getBundleNameByPath("foo/Resources/views/storefront/base.html.twig"))
	assert.Equal(t, "storefront", getBundleNameByPath("vendor/shopware/storefront/Resources/views/storefront/base.html.twig"))
	assert.Equal(t, "MyFoo", getBundleNameByPath("vendor/store.shopware.com/MyFoo/src/Resources/views/storefront/base.html.twig"))
}

func TestIsOriginalTemplateSource(t *testing.T) {
	assert.True(t, IsOriginalTemplateSource("/project/vendor/shopware/storefront/Resources/views/storefront/base.html.twig"))
	assert.True(t, IsOriginalTemplateSource("/project/src/Storefront/Resources/views/storefront/base.html.twig"))
	assert.True(t, IsOriginalTemplateSource("/project/vendor/store.shopware.com/MyFoo/src/Resources/views/storefront/page/foo.html.twig"))
	assert.True(t, IsOriginalTemplateSource("/project/vendor/store.shopware.com/swagcustomizedproducts/src/Resources/views/storefront/component/buy-widget/buy-widget-form-customized-products.html.twig"))
	assert.False(t, IsOriginalTemplateSource("/project/custom/plugins/MyPlugin/src/Resources/views/storefront/page/foo.html.twig"))
	assert.False(t, IsOriginalTemplateSource("/project/src/MyPlugin/Resources/views/storefront/page/foo.html.twig"))
	assert.False(t, IsOriginalTemplateSource("/project/src/WbmAidaCore/Resources/views/storefront/component/buy-widget/buy-widget-form-customized-products.html.twig"))
}

func TestBundleNamespaceFromPluginClass(t *testing.T) {
	assert.Equal(t, "SwagCustomizedProducts", bundleNamespaceFromPluginClass("Swag\\CustomizedProducts\\SwagCustomizedProducts"))
}

func TestInvalidateStorePluginBundleNamespaceCache_onComposerReindex(t *testing.T) {
	tempDir := t.TempDir()
	idx, err := NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer idx.Close()

	pluginRoot := filepath.Join(tempDir, "vendor/store.shopware.com/myplugin")
	twigPath := filepath.Join(pluginRoot, "src/Resources/views/storefront/page/foo.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(twigPath), 0755))

	assert.Equal(t, "@myplugin/storefront/page/foo.html.twig", ConvertToRelativePath(twigPath))

	composerPath := filepath.Join(pluginRoot, "composer.json")
	require.NoError(t, os.WriteFile(composerPath, []byte(`{
		"extra": {
			"shopware-plugin-class": "Swag\\CustomizedProducts\\SwagCustomizedProducts"
		}
	}`), 0644))

	require.NoError(t, idx.Index(composerPath, nil, nil))
	assert.Equal(t, "@SwagCustomizedProducts/storefront/page/foo.html.twig", ConvertToRelativePath(twigPath))
}
