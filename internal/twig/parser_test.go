package twig

import (
	"testing"

	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	"github.com/stretchr/testify/assert"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestTwigParse(t *testing.T) {
	parser := tree_sitter.NewParser()

	assert.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))

	content := []byte(`{% block foo %}{% endblock %}`)

	tree := parser.Parse(content, nil)
	defer tree.Close()

	file, err := ParseTwig("test", tree.RootNode(), content)
	assert.NoError(t, err)

	assert.Equal(t, "test", file.Path)
	// Check that we have the block with correct name and line
	assert.Len(t, file.Blocks, 1)
	assert.Contains(t, file.Blocks, "foo")
	block := file.Blocks["foo"]
	assert.Equal(t, "foo", block.Name)
	assert.Equal(t, 1, block.Line)
	assert.NotEmpty(t, block.Hash) // Hash should be calculated
	assert.NotEmpty(t, block.Text) // Text should be extracted
	assert.Nil(t, block.VersionComment) // No version comment in this test
}

func TestTwigParseSwExtends(t *testing.T) {
	parser := tree_sitter.NewParser()

	assert.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))

	content := []byte(`{% sw_extends '@Storefront/storefront/base.html.twig' %}`)

	tree := parser.Parse(content, nil)
	defer tree.Close()

	file, err := ParseTwig("test", tree.RootNode(), content)
	assert.NoError(t, err)

	assert.Equal(t, "test", file.Path)
	assert.Equal(t, "@Storefront/storefront/base.html.twig", file.ExtendsFile)
}

func TestNestedBlock(t *testing.T) {
	tpl := `
{% block a %}
	{% block b %}
		{% block c %}
		{% endblock %}
	{% endblock %}
{% endblock %}
`

	parser := tree_sitter.NewParser()

	assert.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))

	tree := parser.Parse([]byte(tpl), nil)
	defer tree.Close()

	file, err := ParseTwig("test", tree.RootNode(), []byte(tpl))
	assert.NoError(t, err)

	assert.Equal(t, "test", file.Path)
	// Check that we have all three blocks with correct names and lines
	assert.Len(t, file.Blocks, 3)
	assert.Contains(t, file.Blocks, "a")
	assert.Contains(t, file.Blocks, "b")
	assert.Contains(t, file.Blocks, "c")
	
	blockA := file.Blocks["a"]
	assert.Equal(t, "a", blockA.Name)
	assert.Equal(t, 2, blockA.Line)
	assert.NotEmpty(t, blockA.Hash)
	assert.NotEmpty(t, blockA.Text)
	assert.Nil(t, blockA.VersionComment)
	
	blockB := file.Blocks["b"]
	assert.Equal(t, "b", blockB.Name)
	assert.Equal(t, 3, blockB.Line)
	assert.NotEmpty(t, blockB.Hash)
	assert.NotEmpty(t, blockB.Text)
	assert.Nil(t, blockB.VersionComment)
	
	blockC := file.Blocks["c"]
	assert.Equal(t, "c", blockC.Name)
	assert.Equal(t, 4, blockC.Line)
	assert.NotEmpty(t, blockC.Hash)
	assert.NotEmpty(t, blockC.Text)
	assert.Nil(t, blockC.VersionComment)
}
