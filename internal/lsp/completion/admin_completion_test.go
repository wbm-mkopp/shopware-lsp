package completion

import (
	"testing"

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

		// Check if position is within this node
		if (startPoint.Row < line || (startPoint.Row == line && startPoint.Column <= col)) &&
			(endPoint.Row > line || (endPoint.Row == line && endPoint.Column >= col)) {
			result = node
			// Continue to find more specific (child) nodes
			for i := uint(0); i < node.ChildCount(); i++ {
				visit(node.Child(i))
			}
		}
	}
	visit(root)
	return result
}

func TestIsInExtendParentArgument(t *testing.T) {
	provider := &AdminCompletionProvider{}

	tests := []struct {
		name     string
		code     string
		line     uint
		col      uint
		expected bool
	}{
		{
			name:     "second argument in Component.extend",
			code:     `Component.extend('my-component', 'sw-base', () => import('./index'));`,
			line:     0,
			col:      35, // Inside 'sw-base'
			expected: true,
		},
		{
			name:     "second argument in Shopware.Component.extend",
			code:     `Shopware.Component.extend('my-component', 'sw-base', () => import('./index'));`,
			line:     0,
			col:      45, // Inside 'sw-base'
			expected: true,
		},
		{
			name:     "first argument should not match",
			code:     `Component.extend('my-component', 'sw-base', () => import('./index'));`,
			line:     0,
			col:      20, // Inside 'my-component'
			expected: false,
		},
		{
			name:     "Component.register should not match",
			code:     `Component.register('my-component', () => import('./index'));`,
			line:     0,
			col:      22, // Inside 'my-component'
			expected: false,
		},
		{
			name: "second argument with destructured Component",
			code: `const { Component } = Shopware;
Component.extend('foo', 'bar', {});`,
			line:     1,
			col:      26, // Inside 'bar'
			expected: true,
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

			result := provider.isInExtendParentArgument(node, []byte(tt.code))
			assert.Equal(t, tt.expected, result, "Node kind: %s, text: %s", node.Kind(), string(node.Utf8Text([]byte(tt.code))))
		})
	}
}

func TestIsSecondStringArgument(t *testing.T) {
	provider := &AdminCompletionProvider{}

	code := `Component.extend('first', 'second', () => {});`

	tree, parser := parseJS(t, code)
	defer tree.Close()
	defer parser.Close()

	// Find the 'second' string node (around col 28)
	secondNode := findNodeAtPosition(tree.RootNode(), 0, 28)
	assert.NotNil(t, secondNode)

	result := provider.isSecondStringArgument(secondNode, []byte(code))
	assert.True(t, result)

	// Find the 'first' string node (around col 20)
	firstNode := findNodeAtPosition(tree.RootNode(), 0, 20)
	assert.NotNil(t, firstNode)

	result = provider.isSecondStringArgument(firstNode, []byte(code))
	assert.False(t, result)
}

func TestGetSlotCompletions(t *testing.T) {
	// This is a unit test for the slot completion logic
	// It tests that the completion items are correctly generated from slots

	provider := &AdminCompletionProvider{
		adminIndexer: nil, // We'll test the slot parsing logic separately
	}

	// Test that getSlotCompletions returns empty when indexer is nil
	items := provider.getSlotCompletions("sw-card")
	assert.Empty(t, items)
}

func TestGetFirstChildOfKind(t *testing.T) {
	provider := &AdminCompletionProvider{}

	code := `function test() { return 42; }`

	tree, parser := parseJS(t, code)
	defer tree.Close()
	defer parser.Close()

	root := tree.RootNode()
	assert.NotNil(t, root)

	// Find function_declaration
	funcDecl := provider.getFirstChildOfKind(root, "function_declaration")
	assert.NotNil(t, funcDecl)
	assert.Equal(t, "function_declaration", funcDecl.Kind())

	// Find non-existent kind
	nonExistent := provider.getFirstChildOfKind(root, "class_declaration")
	assert.Nil(t, nonExistent)
}
