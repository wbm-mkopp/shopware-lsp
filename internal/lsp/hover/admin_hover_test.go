package hover

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

func TestAdminHoverIsInComponentCall(t *testing.T) {
	provider := &AdminHoverProvider{}

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

func TestAdminHoverExtractComponentName(t *testing.T) {
	provider := &AdminHoverProvider{}

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

func TestBuildHoverContent(t *testing.T) {
	provider := &AdminHoverProvider{}

	tests := []struct {
		name       string
		components []admin.VueComponent
		contains   []string
	}{
		{
			name: "component with props",
			components: []admin.VueComponent{
				{
					Name: "sw-button",
					Props: []admin.VueComponentProp{
						{Name: "label", Type: "String", Required: true},
						{Name: "disabled", Type: "Boolean", Default: "false"},
					},
				},
			},
			contains: []string{
				"## `sw-button`",
				"### Props",
				"`label`: **String** *(required)*",
				"`disabled`: **Boolean** = `false`",
			},
		},
		{
			name: "component with emits",
			components: []admin.VueComponent{
				{
					Name:  "sw-input",
					Emits: []string{"input", "change", "blur"},
				},
			},
			contains: []string{
				"## `sw-input`",
				"### Events",
				"`input`",
				"`change`",
				"`blur`",
			},
		},
		{
			name: "component with methods",
			components: []admin.VueComponent{
				{
					Name:    "sw-modal",
					Methods: []string{"open", "close", "toggle"},
				},
			},
			contains: []string{
				"## `sw-modal`",
				"### Methods",
				"`open()`",
				"`close()`",
				"`toggle()`",
			},
		},
		{
			name: "component with computed",
			components: []admin.VueComponent{
				{
					Name:     "sw-list",
					Computed: []string{"filteredItems", "totalCount"},
				},
			},
			contains: []string{
				"## `sw-list`",
				"### Computed",
				"`filteredItems`",
				"`totalCount`",
			},
		},
		{
			name: "component that extends another",
			components: []admin.VueComponent{
				{
					Name:             "sw-custom-button",
					ExtendsComponent: "sw-button",
				},
			},
			contains: []string{
				"## `sw-custom-button`",
				"**Extends**: `sw-button`",
			},
		},
		{
			name: "component with definition path",
			components: []admin.VueComponent{
				{
					Name:           "sw-card",
					DefinitionPath: "/path/to/sw-card/index.js",
				},
			},
			contains: []string{
				"## `sw-card`",
				"*Defined in*: `/path/to/sw-card/index.js`",
			},
		},
		{
			name: "full component",
			components: []admin.VueComponent{
				{
					Name:             "sw-data-grid",
					ExtendsComponent: "sw-base-grid",
					Props: []admin.VueComponentProp{
						{Name: "columns", Type: "Array", Required: true},
						{Name: "dataSource", Type: "Array"},
					},
					Emits:          []string{"selection-change", "page-change"},
					Methods:        []string{"refresh", "selectAll"},
					Computed:       []string{"selectedItems"},
					DefinitionPath: "/path/to/sw-data-grid/index.js",
				},
			},
			contains: []string{
				"## `sw-data-grid`",
				"**Extends**: `sw-base-grid`",
				"### Props",
				"`columns`: **Array** *(required)*",
				"`dataSource`: **Array**",
				"### Events",
				"`selection-change`",
				"`page-change`",
				"### Methods",
				"`refresh()`",
				"`selectAll()`",
				"### Computed",
				"`selectedItems`",
				"*Defined in*: `/path/to/sw-data-grid/index.js`",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.buildHoverContent(tt.components)

			for _, expected := range tt.contains {
				assert.Contains(t, result, expected)
			}
		})
	}
}
