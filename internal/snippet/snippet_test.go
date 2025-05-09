package snippet

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
)

func TestParseSnippetFile(t *testing.T) {
	bytes, err := os.ReadFile("testdata/nested.json")

	assert.NoError(t, err)

	parser := tree_sitter.NewParser()
	assert.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_json.Language())))

	tree := parser.Parse(bytes, nil)
	if tree == nil {
		t.Fatalf("Failed to parse JSON")
	}
	defer tree.Close()

	result, err := parseSnippetFile(tree.RootNode(), bytes, "testdata/nested.json")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := map[string]Snippet{
		"foo.foo.name": {
			Key:  "foo.foo.name",
			Text: "title",
			File: "testdata/nested.json",
			Line: 5,
		},
		"foo.name": {
			Key:  "foo.name",
			Text: "title",
			File: "testdata/nested.json",
			Line: 3,
		},
	}

	assert.Equal(t, expected, result)
}
