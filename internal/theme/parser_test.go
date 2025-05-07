package theme

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
)

func TestParseThemeConfig(t *testing.T) {
	bytes, err := os.ReadFile("testdata/theme.json")
	assert.NoError(t, err)

	parser := tree_sitter.NewParser()
	assert.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_json.Language())))

	tree := parser.Parse(bytes, nil)
	if tree == nil {
		t.Fatalf("Failed to parse JSON")
	}
	defer tree.Close()

	filePath := "testdata/theme.json"
	fields, err := ParseThemeConfig(tree.RootNode(), bytes, filePath)
	assert.NoError(t, err)

	// Verify we got fields
	assert.NotEmpty(t, fields)

	// A map to make field searching easier for tests
	fieldsMap := make(map[string]ThemeConfigField)
	for _, field := range fields {
		fieldsMap[field.Key] = field
	}

	// Check that important fields exist
	assert.Contains(t, fieldsMap, "sw-color-brand-primary")
	assert.Contains(t, fieldsMap, "sw-color-success")
	assert.Contains(t, fieldsMap, "sw-logo-desktop")

	// Check a specific field
	primaryColorField := fieldsMap["sw-color-brand-primary"]
	assert.Equal(t, "Primary colour", primaryColorField.Label["en-GB"])
	assert.Equal(t, "color", primaryColorField.Type)
	assert.Equal(t, "#0042a0", primaryColorField.Value)
	assert.True(t, primaryColorField.Editable)
	assert.Equal(t, "themeColors", primaryColorField.Block)
	assert.Equal(t, 100, primaryColorField.Order)

	// Check the Path and Line fields
	assert.Equal(t, filePath, primaryColorField.Path)
	assert.Greater(t, primaryColorField.Line, 0) // Line should be greater than 0

	// Verify we have the expected number of fields
	expectedFieldCount := 20 // Based on the theme.json file
	assert.Len(t, fields, expectedFieldCount)
}
