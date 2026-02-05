package definition

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

func findStringNodeWithText(root *tree_sitter.Node, content []byte, targetText string) *tree_sitter.Node {
	var result *tree_sitter.Node
	var visit func(node *tree_sitter.Node)
	visit = func(node *tree_sitter.Node) {
		if node.Kind() == "string" {
			text := string(node.Utf8Text(content))
			if text == targetText || text == "'"+targetText+"'" || text == "\""+targetText+"\"" {
				result = node
				return
			}
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

func TestSnippetDefinitionPatternMatching(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		snippetKey     string
		expectFrontend bool
		expectAdmin    bool
	}{
		{
			name:           "frontend trans filter",
			code:           `{{ 'checkout.cart.title'|trans }}`,
			snippetKey:     "checkout.cart.title",
			expectFrontend: true,
			expectAdmin:    false,
		},
		{
			name:           "admin $tc function",
			code:           `{{ $tc('sw-settings.index.title') }}`,
			snippetKey:     "sw-settings.index.title",
			expectFrontend: false,
			expectAdmin:    true,
		},
		{
			name:           "admin $t function",
			code:           `{{ $t('global.actions.save') }}`,
			snippetKey:     "global.actions.save",
			expectFrontend: false,
			expectAdmin:    true,
		},
		{
			name:           "admin $tc with parameters",
			code:           `{{ $tc('sw-product.list.count', items.length) }}`,
			snippetKey:     "sw-product.list.count",
			expectFrontend: false,
			expectAdmin:    true,
		},
		{
			name:           "trans filter with parameters",
			code:           `{{ 'checkout.cart.items'|trans({'%count%': count}) }}`,
			snippetKey:     "checkout.cart.items",
			expectFrontend: true,
			expectAdmin:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseTwig(t, tt.code)
			defer tree.Close()
			defer parser.Close()

			stringNode := findStringNodeWithText(tree.RootNode(), []byte(tt.code), tt.snippetKey)
			if stringNode == nil {
				t.Fatalf("Could not find string node with text '%s'", tt.snippetKey)
			}

			frontendMatch := treesitterhelper.TwigTransPattern().Matches(stringNode, []byte(tt.code))
			adminMatch := treesitterhelper.TwigAdminSnippetPattern().Matches(stringNode, []byte(tt.code))

			assert.Equal(t, tt.expectFrontend, frontendMatch, "Frontend pattern match for: %s", tt.code)
			assert.Equal(t, tt.expectAdmin, adminMatch, "Admin pattern match for: %s", tt.code)

			// Also verify we can extract the snippet key correctly
			extractedKey := treesitterhelper.GetNodeText(stringNode, []byte(tt.code))
			assert.Equal(t, tt.snippetKey, extractedKey, "Extracted snippet key for: %s", tt.code)
		})
	}
}

func TestSnippetDefinitionEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		code           string
		expectFrontend bool
		expectAdmin    bool
	}{
		{
			name:           "nested in block",
			code:           `{% block content %}{{ $tc('my.key') }}{% endblock %}`,
			expectFrontend: false,
			expectAdmin:    true,
		},
		{
			name:           "in if condition",
			code:           `{% if $tc('condition.key') %}yes{% endif %}`,
			expectFrontend: false,
			expectAdmin:    true,
		},
		{
			name:           "in set statement",
			code:           `{% set label = 'my.label'|trans %}`,
			expectFrontend: true,
			expectAdmin:    false,
		},
		{
			name:           "multiple on same line",
			code:           `{{ $tc('first.key') }} - {{ $tc('second.key') }}`,
			expectFrontend: false,
			expectAdmin:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseTwig(t, tt.code)
			defer tree.Close()
			defer parser.Close()

			// Find the first string node
			var stringNode *tree_sitter.Node
			var visit func(node *tree_sitter.Node)
			visit = func(node *tree_sitter.Node) {
				if node.Kind() == "string" && stringNode == nil {
					stringNode = node
					return
				}
				for i := uint(0); i < node.ChildCount(); i++ {
					visit(node.Child(i))
					if stringNode != nil {
						return
					}
				}
			}
			visit(tree.RootNode())

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
