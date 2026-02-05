package completion

import (
	"testing"

	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	"github.com/stretchr/testify/assert"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func parseTwig(t *testing.T, code string) (*tree_sitter.Tree, *tree_sitter.Parser) {
	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse([]byte(code), nil)
	return tree, parser
}

func findStringNode(root *tree_sitter.Node, content []byte) *tree_sitter.Node {
	var result *tree_sitter.Node
	var visit func(node *tree_sitter.Node)
	visit = func(node *tree_sitter.Node) {
		if node.Kind() == "string" {
			result = node
			return
		}
		for i := uint(0); i < node.ChildCount(); i++ {
			visit(node.Child(i))
			if result != nil {
				return
			}
		}
	}
	visit(root)
	return result
}

func TestSnippetCompletionPatternMatching(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		expectFrontend bool
		expectAdmin    bool
	}{
		{
			name:           "frontend trans filter",
			code:           `{{ 'snippet.key'|trans }}`,
			expectFrontend: true,
			expectAdmin:    false,
		},
		{
			name:           "admin $tc function",
			code:           `{{ $tc('snippet.key') }}`,
			expectFrontend: false,
			expectAdmin:    true,
		},
		{
			name:           "admin $t function",
			code:           `{{ $t('snippet.key') }}`,
			expectFrontend: false,
			expectAdmin:    true,
		},
		{
			name:           "regular string - no completion",
			code:           `{{ 'just a string' }}`,
			expectFrontend: false,
			expectAdmin:    false,
		},
		{
			name:           "regular function call - no completion",
			code:           `{{ someFunc('arg') }}`,
			expectFrontend: false,
			expectAdmin:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseTwig(t, tt.code)
			defer tree.Close()
			defer parser.Close()

			stringNode := findStringNode(tree.RootNode(), []byte(tt.code))
			if stringNode == nil {
				t.Fatal("Could not find string node")
			}

			frontendMatch := treesitterhelper.TwigTransPattern().Matches(stringNode, []byte(tt.code))
			adminMatch := treesitterhelper.TwigAdminSnippetPattern().Matches(stringNode, []byte(tt.code))

			assert.Equal(t, tt.expectFrontend, frontendMatch, "Frontend pattern match for: %s", tt.code)
			assert.Equal(t, tt.expectAdmin, adminMatch, "Admin pattern match for: %s", tt.code)
		})
	}
}
