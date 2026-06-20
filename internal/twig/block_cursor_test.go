package twig

import (
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

func TestBlockNamesAtCursor_nestedReturnsInnermostFirst(t *testing.T) {
	content := []byte(`{% block outer %}
    <div>
        {% block inner %}
            text
        {% endblock %}
    </div>
{% endblock %}`)

	// Line 3 (0-based) sits inside both blocks; innermost must come first.
	names := BlockNamesAtCursor(content, 3)
	require.Equal(t, []string{"inner", "outer"}, names)

	// A line outside any block has no candidates.
	assert.Empty(t, BlockNamesAtCursor(content, 100))
}

func TestIsBlockTagLinePrefix(t *testing.T) {
	assert.True(t, isBlockTagLinePrefix("{% block "))
	assert.True(t, isBlockTagLinePrefix("    {% block "))
	assert.False(t, isBlockTagLinePrefix("{% sw_include "))
}
