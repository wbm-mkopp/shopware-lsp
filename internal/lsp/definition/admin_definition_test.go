package definition

import (
	"testing"

	"github.com/shopware/shopware-lsp/internal/admin"
	"github.com/stretchr/testify/assert"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

func parseJS(t *testing.T, code string) (*tree_sitter.Tree, *tree_sitter.Parser) {
	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse([]byte(code), nil)
	return tree, parser
}

func findNodeAtPosition(root *tree_sitter.Node, line, col uint) *tree_sitter.Node {
	var result *tree_sitter.Node
	var visit func(node *tree_sitter.Node)
	visit = func(node *tree_sitter.Node) {
		startPoint := node.Range().StartPoint
		endPoint := node.Range().EndPoint

		if (startPoint.Row < line || (startPoint.Row == line && startPoint.Column <= col)) &&
			(endPoint.Row > line || (endPoint.Row == line && endPoint.Column >= col)) {
			result = node
			for i := uint(0); i < node.ChildCount(); i++ {
				visit(node.Child(i))
			}
		}
	}
	visit(root)
	return result
}

func TestIsInComponentCall(t *testing.T) {
	provider := &AdminDefinitionProvider{}

	tests := []struct {
		name     string
		code     string
		line     uint
		col      uint
		expected bool
	}{
		{
			name:     "Component.register first arg",
			code:     `Component.register('my-component', () => import('./index'));`,
			line:     0,
			col:      22,
			expected: true,
		},
		{
			name:     "Component.extend first arg",
			code:     `Component.extend('my-component', 'parent', () => import('./index'));`,
			line:     0,
			col:      22,
			expected: true,
		},
		{
			name:     "Component.extend second arg (parent)",
			code:     `Component.extend('my-component', 'parent', () => import('./index'));`,
			line:     0,
			col:      36,
			expected: true,
		},
		{
			name:     "Shopware.Component.register",
			code:     `Shopware.Component.register('my-component', () => import('./index'));`,
			line:     0,
			col:      32,
			expected: true,
		},
		{
			name:     "Shopware.Component.extend",
			code:     `Shopware.Component.extend('my-component', 'parent', () => import('./index'));`,
			line:     0,
			col:      45,
			expected: true,
		},
		{
			name:     "not in component call",
			code:     `const name = 'my-component';`,
			line:     0,
			col:      16,
			expected: false,
		},
		{
			name:     "different function call",
			code:     `someFunc('my-component');`,
			line:     0,
			col:      12,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseJS(t, tt.code)
			defer tree.Close()
			defer parser.Close()

			node := findNodeAtPosition(tree.RootNode(), tt.line, tt.col)
			if node == nil {
				t.Fatalf("Could not find node at position %d:%d", tt.line, tt.col)
			}

			result := provider.isInComponentCall(node, []byte(tt.code))
			assert.Equal(t, tt.expected, result, "Node kind: %s, text: %s", node.Kind(), string(node.Utf8Text([]byte(tt.code))))
		})
	}
}

func TestExtractComponentName(t *testing.T) {
	provider := &AdminDefinitionProvider{}

	tests := []struct {
		name     string
		code     string
		line     uint
		col      uint
		expected string
	}{
		{
			name:     "single quoted string",
			code:     `Component.register('my-component', () => {});`,
			line:     0,
			col:      22,
			expected: "my-component",
		},
		{
			name:     "double quoted string",
			code:     `Component.register("my-component", () => {});`,
			line:     0,
			col:      22,
			expected: "my-component",
		},
		{
			name:     "parent component name",
			code:     `Component.extend('child', 'sw-base-component', () => {});`,
			line:     0,
			col:      30,
			expected: "sw-base-component",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseJS(t, tt.code)
			defer tree.Close()
			defer parser.Close()

			node := findNodeAtPosition(tree.RootNode(), tt.line, tt.col)
			if node == nil {
				t.Fatalf("Could not find node at position %d:%d", tt.line, tt.col)
			}

			result := provider.extractComponentName(node, []byte(tt.code))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestNormalizePropName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"label", "label"},
		{"position-identifier", "positionIdentifier"},
		{":label", "label"},
		{":position-identifier", "positionIdentifier"},
		{"v-bind:label", "label"},
		{"v-bind:position-identifier", "positionIdentifier"},
		{"@click", ""},     // event handler
		{"v-on:click", ""}, // event handler
		{"v-if", ""},       // directive
		{"v-for", ""},      // directive
		{"v-model", ""},    // directive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := admin.NormalizePropName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestKebabToCamel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"label", "label"},
		{"position-identifier", "positionIdentifier"},
		{"my-prop-name", "myPropName"},
		{"a-b-c", "aBC"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := kebabToCamel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
