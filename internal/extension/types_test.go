package extension

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsLocalExtensionPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/project/custom/plugins/MyPlugin/MyPlugin.php", true},
		{"/project/custom/static-plugins/MyPlugin/src/MyPlugin.php", true},
		{"/project/src/MyBundle/MyBundle.php", true},
		{"/project/vendor/store.shopware.com/MyPlugin/src/MyPlugin.php", false},
		{"/project/vendor/shopware/storefront/Storefront.php", false},
		{"/project/vendor/acme/plugin/src/Plugin.php", false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, IsLocalExtensionPath(tt.path), tt.path)
	}
}
