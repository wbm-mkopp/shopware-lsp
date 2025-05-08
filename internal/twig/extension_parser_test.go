package twig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
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
	functions, filters, err := ParseTwigExtension(filePath, rootNode, content)
	require.NoError(t, err)

	// Verify functions
	require.Len(t, functions, 2)
	assert.Equal(t, "test", functions[0].Name)
	assert.Equal(t, filePath, functions[0].FilePath)

	assert.Equal(t, "test2", functions[1].Name)
	assert.Equal(t, filePath, functions[1].FilePath)

	// Verify function parameters
	require.Len(t, functions[0].Parameters, 1)
	assert.Equal(t, "$test", functions[0].Parameters[0].Name)
	assert.Equal(t, "string", functions[0].Parameters[0].Type)
	assert.False(t, functions[0].Parameters[0].Optional)

	// Verify filters
	require.Len(t, filters, 3)
	assert.Equal(t, "abs", filters[0].Name)
	assert.Equal(t, filePath, filters[0].FilePath)

	assert.Equal(t, "test", filters[1].Name)
	assert.Equal(t, filePath, filters[1].FilePath)

	assert.Equal(t, "test2", filters[2].Name)
	assert.Equal(t, filePath, filters[2].FilePath)
}

func TestParseTwigExtension2(t *testing.T) {
	// Read test file
	filePath := filepath.Join("testdata", "extension2.php")
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
	functions, _, err := ParseTwigExtension(filePath, rootNode, content)
	require.NoError(t, err)

	// Verify functions
	require.Len(t, functions, 2)
	assert.Equal(t, "inAppPurchase", functions[0].Name)
	assert.Equal(t, filePath, functions[0].FilePath)

	assert.Equal(t, "allInAppPurchases", functions[1].Name)
	assert.Equal(t, filePath, functions[1].FilePath)
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
	functions, filters, err := ParseTwigExtension(tmpFile, rootNode, content)
	require.NoError(t, err)
	assert.Nil(t, functions, "Should return nil functions for non-extension classes")
	assert.Nil(t, filters, "Should return nil filters for non-extension classes")
}
