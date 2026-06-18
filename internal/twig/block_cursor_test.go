package twig

import (
	"os"
	"testing"

	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestBlockNameAtNode_errorWrappedBlockTag(t *testing.T) {
	content := []byte(`{% block outer_block %}
    <div>content</div>
{% endblock %}`)

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	tree := parser.Parse(content, nil)
	defer tree.Close()

	node := treesitterhelper.FindIdentifierNode(tree.RootNode(), content, "outer_block")
	require.NotNil(t, node)

	name, ok := BlockNameAtNode(node, content)
	assert.True(t, ok)
	assert.Equal(t, "outer_block", name)
}

func TestBlockNameAtCursor_swagTitlePartial(t *testing.T) {
	path := "/Users/mkopp/Projects/Customers/aida/vendor/store.shopware.com/swagcustomizedproducts/src/Resources/views/storefront/component/customized-products/_include/title.html.twig"
	content, err := os.ReadFile(path)
	if err != nil {
		t.Skip("aida project template not available")
	}

	name, ok := BlockNameAtCursor(content, 11)
	assert.True(t, ok)
	assert.Equal(t, "swag_customized_products_option_type_template_label_container", name)

	name, ok = BlockNameAtCursor(content, 2)
	assert.True(t, ok)
	assert.Equal(t, "swag_customized_products_option_type_template_label", name)
}

func TestBlockNameAtNode_swagCustomizedProductsTitlePartial(t *testing.T) {
	path := "/Users/mkopp/Projects/Customers/aida/vendor/store.shopware.com/swagcustomizedproducts/src/Resources/views/storefront/component/customized-products/_include/title.html.twig"
	content, err := os.ReadFile(path)
	if err != nil {
		t.Skip("aida project template not available locally")
	}

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	tree := parser.Parse(content, nil)
	defer tree.Close()

	tests := []struct {
		blockName string
	}{
		{"swag_customized_products_option_type_template_label"},
		{"swag_customized_products_option_type_template_label_container"},
		{"swag_customized_products_option_type_template_label_content"},
		{"swag_customized_products_option_type_template_label_content_text"},
		{"swag_customized_products_option_type_template_label_toggle_icon"},
		{"swag_customized_products_option_type_template_label_toggle_icon_plus"},
	}

	for _, tt := range tests {
		t.Run(tt.blockName, func(t *testing.T) {
			node := treesitterhelper.FindIdentifierNode(tree.RootNode(), content, tt.blockName)
			require.NotNil(t, node, "identifier node for %s", tt.blockName)

			name, ok := BlockNameAtNode(node, content)
			assert.True(t, ok)
			assert.Equal(t, tt.blockName, name)
		})
	}
}

func TestIsBlockTagLinePrefix(t *testing.T) {
	assert.True(t, isBlockTagLinePrefix("{% block "))
	assert.True(t, isBlockTagLinePrefix("    {% block "))
	assert.False(t, isBlockTagLinePrefix("{% sw_include "))
}
