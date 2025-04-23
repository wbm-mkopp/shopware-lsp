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
	assert.Equal(t, "storefront/base.html.twig", ConvertToRelativePath("/Resources/views/storefront/base.html.twig"))
}

func TestGetBundleNameByPath(t *testing.T) {
	assert.Equal(t, "foo", getBundleNameByPath("foo/Resources/views/storefront/base.html.twig"))
	assert.Equal(t, "storefront", getBundleNameByPath("vendor/shopware/storefront/Resources/views/storefront/base.html.twig"))
	assert.Equal(t, "MyFoo", getBundleNameByPath("vendor/store.shopware.com/MyFoo/src/Resources/views/storefront/base.html.twig"))
}
