package twig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertToRelativePath(t *testing.T) {
	assert.Equal(t, "", convertToRelativePath(""))
	assert.Equal(t, "", convertToRelativePath("/"))
	assert.Equal(t, "", convertToRelativePath("/Resources/views"))
	assert.Equal(t, "", convertToRelativePath("/Resources/views/"))
	assert.Equal(t, "storefront/base.html.twig", convertToRelativePath("/Resources/views/storefront/base.html.twig"))
}
