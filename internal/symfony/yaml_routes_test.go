package symfony

import (
	"testing"

	"github.com/stretchr/testify/assert"
	tree_sitter_yaml "github.com/tree-sitter-grammars/tree-sitter-yaml/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestParseYAMLRoutes(t *testing.T) {
	// Create a temporary test file
	yamlContent := `# Routes file
app_homepage:
    path: /
    controller: App\Controller\DefaultController::index

app_product:
    path: /product/{id}
    controller: App\Controller\ProductController::show
    methods: [GET]
    
app_product_create:
    path: /product/create
    controller: App\Controller\ProductController::create
    methods: [GET, POST]
    defaults:
        color: blue
`

	// Create a YAML parser
	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_yaml.Language())); err != nil {
		t.Fatal(err)
	}

	// Parse the YAML content
	tree := parser.Parse([]byte(yamlContent), nil)
	defer tree.Close()

	// Run the parser
	routes, err := ParseYAMLRoutes("test.yaml", tree.RootNode(), []byte(yamlContent))
	assert.NoError(t, err)

	// Debug the routes found
	t.Logf("Found %d routes", len(routes))
	for i, route := range routes {
		t.Logf("Route %d: %+v", i, route)
	}

	// Verify the parsed routes
	if assert.Len(t, routes, 3, "Expected 3 routes from tree-sitter YAML parsing") {
		// Check the first route
		assert.Equal(t, "app_homepage", routes[0].Name)
		assert.Equal(t, "/", routes[0].Path)
		assert.Equal(t, "App\\Controller\\DefaultController::index", routes[0].Controller)
		assert.Equal(t, "test.yaml", routes[0].FilePath)
		assert.Greater(t, routes[0].Line, 0)

		// Check the second route
		assert.Equal(t, "app_product", routes[1].Name)
		assert.Equal(t, "/product/{id}", routes[1].Path)
		assert.Equal(t, "App\\Controller\\ProductController::show", routes[1].Controller)
		assert.Equal(t, "test.yaml", routes[1].FilePath)
		assert.Greater(t, routes[1].Line, 0)

		// Check the third route
		assert.Equal(t, "app_product_create", routes[2].Name)
		assert.Equal(t, "/product/create", routes[2].Path)
		assert.Equal(t, "App\\Controller\\ProductController::create", routes[2].Controller)
		assert.Equal(t, "test.yaml", routes[2].FilePath)
		assert.Greater(t, routes[2].Line, 0)
	}
}
