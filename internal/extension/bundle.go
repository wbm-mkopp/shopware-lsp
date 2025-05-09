package extension

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/shopware/shopware-lsp/internal/php"
)

// isShopwareBundle checks if a class extends Shopware\Core\Framework\Bundle or Shopware\Core\Framework\Plugin
func isShopwareBundle(class php.PHPClass) bool {
	if class.IsInterface || class.Parent == "" {
		return false
	}

	return class.Parent == "\\Shopware\\Core\\Framework\\Bundle" ||
		class.Parent == "Shopware\\Core\\Framework\\Bundle" ||
		class.Parent == "\\Shopware\\Core\\Framework\\Plugin" ||
		class.Parent == "Shopware\\Core\\Framework\\Plugin"
}

// createBundleFromClass creates a ShopwareExtension instance from a PHP class
func createBundleFromClass(class php.PHPClass) ShopwareExtension {
	// Extract the last part of the fully qualified class name
	nameParts := strings.Split(class.Name, "\\")
	name := class.Name
	if len(nameParts) > 0 {
		name = nameParts[len(nameParts)-1]
	}

	return ShopwareExtension{
		Name: name,
		Path: class.Path,
		Type: ShopwareExtensionTypeBundle,
	}
}

var coreBundles = []string{
	"Administration.php",
	"Checkout.php",
	"DevOps.php",
	"Framework.php",
	"Plugin.php",
	"Maintenance.php",
	"Profiling.php",
	"Service.php",
	"Content.php",
	"System.php",
	"Elasticsearch.php",
	"Storefront.php",
}

// isValidForIndex checks if a file should be indexed
func isValidForIndex(filePath string) bool {
	// Handle test directories in the path
	pathParts := strings.Split(filepath.ToSlash(filePath), "/")
	for _, part := range pathParts {
		partLower := strings.ToLower(part)
		if partLower == "tests" || partLower == "test" ||
			partLower == "fixtures" || partLower == "_fixture" ||
			partLower == "_fixtures" {
			return false
		}
	}

	// Skip hidden files
	fileName := filepath.Base(filePath)
	if strings.HasPrefix(fileName, ".") {
		return false
	}

	if slices.Contains(coreBundles, fileName) {
		// Skip all core bundle files
		return false
	}

	// Handle test files but make exceptions for bundle and plugin classes
	fileNameLower := strings.ToLower(fileName)
	if strings.Contains(fileNameLower, "test") {
		// Skip all test files except TestBundle.php and TestPlugin.php (which may be valid bundle classes)
		if !strings.HasSuffix(fileNameLower, "bundle.php") && !strings.HasSuffix(fileNameLower, "plugin.php") {
			return false
		}
	}

	// If we got this far, the file should be indexed
	return true
}
