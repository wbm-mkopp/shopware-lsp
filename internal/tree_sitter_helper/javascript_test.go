package treesitterhelper

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

func findJSStringNode(root *tree_sitter.Node, content []byte) *tree_sitter.Node {
	var result *tree_sitter.Node
	var visit func(node *tree_sitter.Node)
	visit = func(node *tree_sitter.Node) {
		// Match string_fragment which contains the actual text
		// or string node (for empty strings that have no string_fragment)
		if node.Kind() == "string_fragment" {
			result = node
			return
		}
		if node.Kind() == "string" && result == nil {
			// For empty strings, use the string node itself
			// Check if it has no string_fragment child
			hasFragment := false
			for i := uint(0); i < node.NamedChildCount(); i++ {
				if node.NamedChild(i).Kind() == "string_fragment" {
					hasFragment = true
					break
				}
			}
			if !hasFragment {
				result = node
				return
			}
		}
		for i := uint(0); i < node.NamedChildCount(); i++ {
			visit(node.NamedChild(i))
			if result != nil {
				return
			}
		}
	}
	visit(root)
	return result
}

func TestJSAdminSnippetPattern(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected bool
	}{
		{
			name:     "this.$tc call",
			code:     `this.$tc('my.snippet.key');`,
			expected: true,
		},
		{
			name:     "this.$t call",
			code:     `this.$t('my.snippet.key');`,
			expected: true,
		},
		{
			name:     "this.$tc with parameters",
			code:     `this.$tc('my.snippet.key', { count: 5 });`,
			expected: true,
		},
		{
			name:     "regular method call should not match",
			code:     `this.someMethod('my.snippet.key');`,
			expected: false,
		},
		{
			name:     "non-this call should not match",
			code:     `$tc('my.snippet.key');`,
			expected: false,
		},
		{
			name:     "object method call should not match",
			code:     `obj.$tc('my.snippet.key');`,
			expected: false,
		},
		{
			name:     "this.$tc in return statement",
			code:     `return this.$tc('my.snippet.key');`,
			expected: true,
		},
		{
			name:     "this.$tc in variable assignment",
			code:     `const label = this.$tc('my.snippet.key');`,
			expected: true,
		},
		{
			name:     "this.$tc with empty string",
			code:     `this.$tc('');`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseJS(t, tt.code)
			defer tree.Close()
			defer parser.Close()

			stringNode := findJSStringNode(tree.RootNode(), []byte(tt.code))
			if stringNode == nil {
				if tt.expected {
					t.Fatal("Could not find string node in code")
				}
				return
			}

			result := JSAdminSnippetPattern().Matches(stringNode, []byte(tt.code))
			assert.Equal(t, tt.expected, result, "Pattern match for: %s", tt.code)
		})
	}
}

func TestJSAdminSnippetPatternEmptyString(t *testing.T) {
	// Test that the pattern matches when cursor is on quote character (empty string case)
	code := `this.$tc('');`

	tree, parser := parseJS(t, code)
	defer tree.Close()
	defer parser.Close()

	// Find the string node
	var stringNode *tree_sitter.Node
	var visit func(node *tree_sitter.Node)
	visit = func(node *tree_sitter.Node) {
		if node.Kind() == "string" {
			stringNode = node
			return
		}
		for i := uint(0); i < node.NamedChildCount(); i++ {
			visit(node.NamedChild(i))
			if stringNode != nil {
				return
			}
		}
	}
	visit(tree.RootNode())

	assert.NotNil(t, stringNode, "Should find string node")

	// The string node itself should match
	assert.True(t, JSAdminSnippetPattern().Matches(stringNode, []byte(code)),
		"Pattern should match string node")

	// Simulate cursor on quote - get first child (the quote character)
	// In tree-sitter, unnamed children include the quotes
	quoteNode := stringNode.Child(0)
	assert.NotNil(t, quoteNode, "Should have child node (quote)")
	assert.Equal(t, "'", quoteNode.Kind(), "First child should be quote")

	// The quote node should also match (because parent is string in right context)
	assert.True(t, JSAdminSnippetPattern().Matches(quoteNode, []byte(code)),
		"Pattern should match quote node inside this.$tc")
}

func TestJSThisMethodCallPattern(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		methodNames []string
		expected    bool
	}{
		{
			name:        "match single method",
			code:        `this.trans('key');`,
			methodNames: []string{"trans"},
			expected:    true,
		},
		{
			name:        "match one of multiple methods",
			code:        `this.$tc('key');`,
			methodNames: []string{"$tc", "$t", "trans"},
			expected:    true,
		},
		{
			name:        "no match when method not in list",
			code:        `this.other('key');`,
			methodNames: []string{"$tc", "$t"},
			expected:    false,
		},
		{
			name:        "empty method list never matches",
			code:        `this.$tc('key');`,
			methodNames: []string{},
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseJS(t, tt.code)
			defer tree.Close()
			defer parser.Close()

			stringNode := findJSStringNode(tree.RootNode(), []byte(tt.code))
			if stringNode == nil {
				if tt.expected {
					t.Fatal("Could not find string node in code")
				}
				return
			}

			result := JSThisMethodCallPattern(tt.methodNames...).Matches(stringNode, []byte(tt.code))
			assert.Equal(t, tt.expected, result, "Pattern match for: %s", tt.code)
		})
	}
}
