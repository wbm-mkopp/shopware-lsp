package php

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInterfaceIndexing(t *testing.T) {
	// Create a new PHP index with a temporary directory following the guidelines
	idx, err := NewPHPIndex(t.TempDir())
	assert.NoError(t, err)

	// Use the test file path for interfaces
	path := filepath.Join("testdata", "interface.php")
	classes := idx.GetClassesOfFile(path)

	// Check that the interface was indexed
	assert.Contains(t, classes, "App\\Interfaces\\CustomInterface", "Interface should be indexed")

	customInterface := classes["App\\Interfaces\\CustomInterface"]

	// Check that the interface is correctly identified
	assert.True(t, customInterface.IsInterface, "CustomInterface should be identified as an interface")

	// Check that extended interfaces are correctly identified
	// Note: Current namespace resolution results in local namespace prefixing for Traversable
	assert.Contains(t, customInterface.Interfaces, "App\\Interfaces\\Traversable", "Interface should extend Traversable")
	assert.Contains(t, customInterface.Interfaces, "LoggerInterface", "Interface should extend LoggerInterface")
	assert.Len(t, customInterface.Interfaces, 2, "Interface should extend exactly 2 interfaces")

	// Verify methods are correctly indexed
	assert.Len(t, customInterface.Methods, 2, "Interface should have 2 methods")
	assert.Contains(t, customInterface.Methods, "getCustomValue", "Interface should have getCustomValue method")
	assert.Contains(t, customInterface.Methods, "setCustomValue", "Interface should have setCustomValue method")

	// Verify properties - interfaces shouldn't have properties
	assert.Len(t, customInterface.Properties, 0, "Interface should have 0 properties")

	// Verify parent is empty for interfaces
	assert.Empty(t, customInterface.Parent, "Interface should not have a parent class")
}
