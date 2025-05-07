package theme

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
)

func TestThemeConfigIndexer(t *testing.T) {
	tempDir := t.TempDir()

	// Create a new indexer
	indexer, err := NewThemeConfigIndexer(tempDir)
	require.NoError(t, err)
	defer func() { _ = indexer.Close() }()

	// Load test theme.json file
	bytes, err := os.ReadFile("testdata/theme.json")
	require.NoError(t, err)

	// Create parser
	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_json.Language())))

	// Parse file
	tree := parser.Parse(bytes, nil)
	require.NotNil(t, tree)
	defer tree.Close()

	// Index the file
	filePath := "testdata/theme.json"
	err = indexer.Index(filePath, tree.RootNode(), bytes)
	require.NoError(t, err)

	// Test GetThemeConfigFields
	keys, err := indexer.GetThemeConfigFields()
	require.NoError(t, err)
	assert.NotEmpty(t, keys)

	// Test GetThemeConfigField for a specific key
	fields, err := indexer.GetThemeConfigField("sw-color-brand-primary")
	require.NoError(t, err)
	assert.NotEmpty(t, fields)
	assert.Equal(t, "Primary colour", fields[0].Label["en-GB"])
	assert.Equal(t, "color", fields[0].Type)

	// Test GetAllThemeConfigFields
	allFields, err := indexer.GetAllThemeConfigFields()
	require.NoError(t, err)
	assert.NotEmpty(t, allFields)

	// Test removing a file
	err = indexer.RemovedFiles([]string{filePath})
	require.NoError(t, err)

	// Verify the file was removed
	emptyKeys, err := indexer.GetThemeConfigFields()
	require.NoError(t, err)
	assert.Empty(t, emptyKeys)
}
