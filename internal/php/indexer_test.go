package php

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetClassesOfFile(t *testing.T) {
	index, err := NewPHPIndex(t.TempDir())
	assert.NoError(t, err)

	classes := index.GetClassesOfFile("testdata/01.php")

	assert.Len(t, classes, 1)

	expectedClassName := "Shopware\\Core\\Content\\Category\\Service\\NavigationLoader"

	assert.Contains(t, classes, expectedClassName)

	assert.Equal(t, expectedClassName, classes[expectedClassName].Name)
	assert.Equal(t, "testdata/01.php", classes[expectedClassName].Path)
	assert.Equal(t, 20, classes[expectedClassName].Line)

	// Check that the constructor method was found
	assert.Contains(t, classes[expectedClassName].Methods, "__construct")
	assert.Equal(t, "__construct", classes[expectedClassName].Methods["__construct"].Name)
	assert.Equal(t, 27, classes[expectedClassName].Methods["__construct"].Line)

	// Check that the property was found
	assert.Contains(t, classes[expectedClassName].Properties, "treeItem")
	assert.Equal(t, "treeItem", classes[expectedClassName].Properties["treeItem"].Name)
	assert.Equal(t, 22, classes[expectedClassName].Properties["treeItem"].Line)
	assert.Equal(t, Private, classes[expectedClassName].Properties["treeItem"].Visibility)
}

func TestGetClassesWithMethodsAndProperties(t *testing.T) {
	// Create a new context for the test
	index, err := NewPHPIndex(t.TempDir())
	assert.NoError(t, err)

	// Parse the test file with multiple methods
	classes := index.GetClassesOfFile("testdata/02.php")

	// Verify we found the class
	assert.Len(t, classes, 1)

	expectedClassName := "Shopware\\Core\\Content\\Product\\Service\\ProductLoader"
	assert.Contains(t, classes, expectedClassName)

	// Verify the class properties
	assert.Equal(t, expectedClassName, classes[expectedClassName].Name)
	assert.Equal(t, "testdata/02.php", classes[expectedClassName].Path)
	assert.Equal(t, 9, classes[expectedClassName].Line)

	// Verify the methods were extracted correctly
	methods := classes[expectedClassName].Methods
	assert.Len(t, methods, 4)

	// Check constructor
	assert.Contains(t, methods, "__construct")
	assert.Equal(t, "__construct", methods["__construct"].Name)
	assert.Equal(t, 16, methods["__construct"].Line)
	assert.Equal(t, Public, methods["__construct"].Visibility)

	// Check public method
	assert.Contains(t, methods, "load")
	assert.Equal(t, "load", methods["load"].Name)
	assert.Equal(t, 22, methods["load"].Line)
	assert.Equal(t, Public, methods["load"].Visibility)

	// Check protected method
	assert.Contains(t, methods, "validateId")
	assert.Equal(t, "validateId", methods["validateId"].Name)
	assert.Equal(t, 27, methods["validateId"].Line)
	assert.Equal(t, Protected, methods["validateId"].Visibility)

	// Check private method
	assert.Contains(t, methods, "getRepository")
	assert.Equal(t, "getRepository", methods["getRepository"].Name)
	assert.Equal(t, 32, methods["getRepository"].Line)
	assert.Equal(t, Private, methods["getRepository"].Visibility)

	// Verify the properties were extracted correctly
	properties := classes[expectedClassName].Properties
	assert.Len(t, properties, 2)

	// Check readonly property
	assert.Contains(t, properties, "request")
	assert.Equal(t, "request", properties["request"].Name)
	assert.Equal(t, 11, properties["request"].Line)
	assert.Equal(t, Private, properties["request"].Visibility)

	// Check property from constructor
	assert.Contains(t, properties, "productRepository")
	assert.Equal(t, "productRepository", properties["productRepository"].Name)
	assert.Equal(t, 17, properties["productRepository"].Line)
	assert.Equal(t, Private, properties["productRepository"].Visibility)
}
