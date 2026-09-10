package twig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertToRelativePath(t *testing.T) {
	assert.Equal(t, "", ConvertToRelativePath(""))
	assert.Equal(t, "", ConvertToRelativePath("/"))
	assert.Equal(t, "", ConvertToRelativePath("/Resources/views"))
	assert.Equal(t, "", ConvertToRelativePath("/Resources/views/"))
	assert.Equal(t, "@Storefront/storefront/base.html.twig", ConvertToRelativePath("/Resources/views/storefront/base.html.twig"))
}

func TestGetBundleNameByPath(t *testing.T) {
	assert.Equal(t, "foo", getBundleNameByPath("foo/Resources/views/storefront/base.html.twig"))
	assert.Equal(t, "storefront", getBundleNameByPath("vendor/shopware/storefront/Resources/views/storefront/base.html.twig"))
	assert.Equal(t, "MyFoo", getBundleNameByPath("vendor/store.shopware.com/MyFoo/src/Resources/views/storefront/base.html.twig"))
}

func TestTemplateNames(t *testing.T) {
	assert.Equal(t, []string{"base.html.twig"}, TemplateNames("/project/templates/base.html.twig"))
	assert.Equal(
		t,
		[]string{
			"card.html.twig",
			"@Storefront/card.html.twig",
			"@MyBundle/card.html.twig",
			"MyBundle::card.html.twig",
		},
		TemplateNames("/project/MyBundle/src/Resources/views/card.html.twig"),
	)
}

func TestIsTemplateAssetPath(t *testing.T) {
	assert.True(t, IsTemplateAssetPath(
		"/project/src/Core/Profiling/Resources/views/Collector/checkmark.svg",
	))
	assert.True(t, IsTemplateAssetPath("/project/templates/mail/logo.svg"))
	// Twig files take the full indexing path instead.
	assert.False(t, IsTemplateAssetPath(
		"/project/src/Storefront/Resources/views/storefront/base.html.twig",
	))
	// Not below a template root, so no loader can address it.
	assert.False(t, IsTemplateAssetPath("/project/public/bundles/storefront/logo.svg"))
	assert.False(t, IsTemplateAssetPath(
		"/project/src/Administration/Resources/app/administration/src/icon.svg",
	))
	assert.False(t, IsTemplateAssetPath("/project/templates/.gitignore"))
	// Shopware symlinks node_modules into a template root, and os.ReadDir
	// reports the link as a file.
	assert.False(t, IsTemplateAssetPath(
		"/project/src/Storefront/Resources/views/components/node_modules",
	))
}
