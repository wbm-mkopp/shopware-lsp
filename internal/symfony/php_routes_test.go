package symfony

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

func TestExtractRoutesFromFile(t *testing.T) {
	// Extract routes from the test file
	filePath := "testdata/controller.php"
	node, content := parsePHPFile(filePath)

	routes := extractRoutes(filePath, node, content)

	// Verify we found only the method route
	assert.Len(t, routes, 1)

	// Verify method route data
	expectedRouteMethod := Route{
		Name:       "frontend.account.address.create",
		Path:       "/account/address/create", // Combined path
		FilePath:   filePath,
		Line:       14, // Line number of the Route attribute in the test file
		Controller: "App\\Controller\\Frontend\\Account\\AddressController::createAddress",
	}

	assert.Equal(t, expectedRouteMethod, routes[0])
}

func TestExtractRoutesWithBasePathFromFile(t *testing.T) {
	// Extract routes from the test file with base path
	filePath := "testdata/controller_base.php"
	node, content := parsePHPFile(filePath)

	routes := extractRoutes(filePath, node, content)

	// Verify we found only the method route
	assert.Len(t, routes, 1)

	// Verify method route data with combined path
	expectedRouteMethod := Route{
		Name:       "foo",
		Path:       "/api/foo", // Base path + route path
		FilePath:   filePath,
		Line:       11, // Line number of the Route attribute in the test file
		Controller: "Shopware\\Core\\Api\\ApiController::foo",
	}

	assert.Equal(t, expectedRouteMethod, routes[0])
}

func parsePHPFile(filePath string) (*tree_sitter.Node, []byte) {
	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())); err != nil {
		panic(err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	defer parser.Close()

	tree := parser.Parse(content, nil)
	return tree.RootNode(), content
}
