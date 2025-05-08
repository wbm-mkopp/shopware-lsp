package twig

import (
	"os"
	"path/filepath"
	"testing"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseTwigExtension(t *testing.T) {
	// Read test file
	filePath := filepath.Join("testdata", "extension.php")
	content, err := os.ReadFile(filePath)
	require.NoError(t, err)

	// Parse the file with tree-sitter
	parser := tree_sitter.NewParser()
	err = parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP()))
	require.NoError(t, err)
	tree := parser.Parse(content, nil)
	defer tree.Close()
	rootNode := tree.RootNode()

	// Parse Twig extension
	extension, err := ParseTwigExtension(filePath, rootNode, content)
	require.NoError(t, err)
	require.NotNil(t, extension)


	// Verify class name
	assert.Equal(t, "App\\Twig\\TwigExt", extension.ClassName)

	// Verify functions
	require.Len(t, extension.Functions, 1)
	assert.Equal(t, "test", extension.Functions[0].Name)
	assert.Equal(t, "$this->test", extension.Functions[0].Method)

	// Verify function parameters
	require.Len(t, extension.Functions[0].Parameters, 1)
	assert.Equal(t, "$test", extension.Functions[0].Parameters[0].Name)
	assert.Equal(t, "string", extension.Functions[0].Parameters[0].Type)
	assert.False(t, extension.Functions[0].Parameters[0].Optional)

	// Verify filters
	require.Len(t, extension.Filters, 1)
	assert.Equal(t, "abs", extension.Filters[0].Name)
	assert.Equal(t, "abs", extension.Filters[0].Method)
}

func TestParseTwigExtensionNotExtending(t *testing.T) {
	// Create a temporary file that doesn't extend AbstractExtension
	tmpFile := filepath.Join(t.TempDir(), "not_extension.php")
	content := []byte(`<?php
namespace App\Twig;

class NotExtension
{
    public function test() {}
}`)

	err := os.WriteFile(tmpFile, content, 0644)
	require.NoError(t, err)

	// Parse the file with tree-sitter
	parser := tree_sitter.NewParser()
	err = parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP()))
	require.NoError(t, err)
	tree := parser.Parse(content, nil)
	defer tree.Close()
	rootNode := tree.RootNode()

	// Parse Twig extension
	extension, err := ParseTwigExtension(tmpFile, rootNode, content)
	require.NoError(t, err)
	assert.Nil(t, extension, "Should return nil for non-extension classes")
}
