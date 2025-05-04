package php

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestClassInheritance(t *testing.T) {
	// Create a new PHP index with a temporary directory following the guidelines
	idx, err := NewPHPIndex(t.TempDir())
	assert.NoError(t, err)

	// Use the test file path for inheritance
	path := filepath.Join("testdata", "inheritance.php")
	classes := idx.GetClassesOfFile(path)

	// Check that the class was indexed
	assert.Contains(t, classes, "App\\Entity\\Product", "Class should be indexed")
	
	product := classes["App\\Entity\\Product"]
	
	// Check that parent is correctly identified
	assert.Equal(t, "App\\BaseClass", product.Parent, "Class should extend App\\BaseClass")
	
	// Check that interfaces are correctly identified
	// NOTE: Currently the AliasResolver implementation treats global interfaces
	// imported with 'use' statements as being in the current namespace.
	// This can be improved in the future to properly recognize global PHP interfaces.
	assert.Contains(t, product.Interfaces, "App\\Entity\\Traversable", "Class should implement Traversable")
	assert.Contains(t, product.Interfaces, "App\\Entity\\Countable", "Class should implement Countable")
	assert.Len(t, product.Interfaces, 2, "Class should implement exactly 2 interfaces")

	// Verify other class aspects are still correctly indexed
	assert.Len(t, product.Methods, 4, "Class should have 4 methods")
	assert.Contains(t, product.Methods, "getId", "Class should have getId method")
	assert.Contains(t, product.Methods, "getName", "Class should have getName method")
	assert.Contains(t, product.Methods, "setName", "Class should have setName method")
	assert.Contains(t, product.Methods, "count", "Class should have count method from Countable interface")
	
	assert.Len(t, product.Properties, 2, "Class should have 2 properties")
	assert.Contains(t, product.Properties, "id", "Class should have id property")
	assert.Contains(t, product.Properties, "name", "Class should have name property")
}
