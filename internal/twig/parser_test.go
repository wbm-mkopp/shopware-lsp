package twig

import (
	"testing"

	tree_sitter_twig "github.com/kaermorchen/tree-sitter-twig/bindings/go"
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
	assert.Equal(t, map[string]TwigBlock{"foo": {Name: "foo", Line: 1}}, file.Blocks)
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
	assert.Equal(t, "'@Storefront/storefront/base.html.twig'", file.ExtendsFile)
}
