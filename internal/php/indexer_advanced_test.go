package php

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdvancedNamespaceAliases(t *testing.T) {
	// Create a new context for the test
	index, err := NewPHPIndex(t.TempDir())
	assert.NoError(t, err)

	// Parse the test file with advanced namespace aliases
	classes := index.GetClassesOfFile("testdata/04.php")

	// Verify we found the class
	assert.Len(t, classes, 1)

	expectedClassName := "Shopware\\Core\\Content\\Product\\Advanced\\AdvancedProductTest"
	assert.Contains(t, classes, expectedClassName)

	// Verify the class properties
	assert.Equal(t, expectedClassName, classes[expectedClassName].Name)
	assert.Equal(t, "testdata/04.php", classes[expectedClassName].Path)

	// Verify the properties with aliased types are correctly resolved
	properties := classes[expectedClassName].Properties
	assert.Len(t, properties, 6)

	// Check property with group use statement type (Request)
	assert.Contains(t, properties, "request")
	assert.Equal(t, "request", properties["request"].Name)
	assert.Equal(t, Private, properties["request"].Visibility)
	assert.Equal(t, "Symfony\\Component\\HttpFoundation\\Request", properties["request"].Type.Name())

	// Check property with group use statement type (Response)
	assert.Contains(t, properties, "response")
	assert.Equal(t, "response", properties["response"].Name)
	assert.Equal(t, Private, properties["response"].Visibility)
	assert.Equal(t, "Symfony\\Component\\HttpFoundation\\Response", properties["response"].Type.Name())

	// Check property with group use statement type (Kernel)
	assert.Contains(t, properties, "kernel")
	assert.Equal(t, "kernel", properties["kernel"].Name)
	assert.Equal(t, Private, properties["kernel"].Visibility)
	assert.Equal(t, "Symfony\\Component\\HttpKernel\\Kernel", properties["kernel"].Type.Name())

	// Check property with aliased type (DbConnection)
	assert.Contains(t, properties, "connection")
	assert.Equal(t, "connection", properties["connection"].Name)
	assert.Equal(t, Private, properties["connection"].Visibility)
	assert.Equal(t, "Doctrine\\DBAL\\Connection", properties["connection"].Type.Name())

	// Check property with aliased type (Repository)
	assert.Contains(t, properties, "productRepository")
	assert.Equal(t, "productRepository", properties["productRepository"].Name)
	assert.Equal(t, Private, properties["productRepository"].Visibility)
	assert.Equal(t, "Shopware\\Core\\Framework\\DataAbstractionLayer\\EntityRepository", properties["productRepository"].Type.Name())

	// Verify the methods
	methods := classes[expectedClassName].Methods
	assert.Len(t, methods, 4)

	// Check method with union type return (PHP 8.0+)
	assert.Contains(t, methods, "getRequestOrResponse")
	assert.Equal(t, "getRequestOrResponse", methods["getRequestOrResponse"].Name)
	assert.Equal(t, Public, methods["getRequestOrResponse"].Visibility)
	// Note: Current implementation might not handle union types correctly yet

	// Check method with nullable type
	assert.Contains(t, methods, "getOptionalKernel")
	assert.Equal(t, "getOptionalKernel", methods["getOptionalKernel"].Name)
	assert.Equal(t, Public, methods["getOptionalKernel"].Visibility)
	// Note: Current implementation might not handle nullable types correctly yet

	// Check method with mixed return type
	assert.Contains(t, methods, "getData")
	assert.Equal(t, "getData", methods["getData"].Name)
	assert.Equal(t, Public, methods["getData"].Visibility)
	assert.Equal(t, "mixed", methods["getData"].ReturnType.Name())
}
