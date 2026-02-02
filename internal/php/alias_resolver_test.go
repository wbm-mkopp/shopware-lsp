package php

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAliasResolver(t *testing.T) {
	// Test cases for different namespace and alias scenarios
	tests := []struct {
		name           string
		namespace      string
		useStatements  map[string]string
		aliases        map[string]string
		typeName       string
		expectedResult string
	}{
		{
			name:           "Primitive type",
			namespace:      "Shopware\\Core\\Content\\Product",
			useStatements:  map[string]string{},
			aliases:        map[string]string{},
			typeName:       "string",
			expectedResult: "string",
		},
		{
			name:           "Special type",
			namespace:      "Shopware\\Core\\Content\\Product",
			useStatements:  map[string]string{},
			aliases:        map[string]string{},
			typeName:       "self",
			expectedResult: "self",
		},
		{
			name:      "Use statement",
			namespace: "Shopware\\Core\\Content\\Product",
			useStatements: map[string]string{
				"Request": "Symfony\\Component\\HttpFoundation\\Request",
			},
			aliases:        map[string]string{},
			typeName:       "Request",
			expectedResult: "Symfony\\Component\\HttpFoundation\\Request",
		},
		{
			name:      "Alias",
			namespace: "Shopware\\Core\\Content\\Product",
			useStatements: map[string]string{
				"Request": "Symfony\\Component\\HttpFoundation\\Request",
			},
			aliases: map[string]string{
				"SymfonyRequest": "Symfony\\Component\\HttpFoundation\\Request",
			},
			typeName:       "SymfonyRequest",
			expectedResult: "Symfony\\Component\\HttpFoundation\\Request",
		},
		{
			name:           "Current namespace",
			namespace:      "Shopware\\Core\\Content\\Product",
			useStatements:  map[string]string{},
			aliases:        map[string]string{},
			typeName:       "ProductEntity",
			expectedResult: "Shopware\\Core\\Content\\Product\\ProductEntity",
		},
		{
			name:           "Fully qualified name",
			namespace:      "Shopware\\Core\\Content\\Product",
			useStatements:  map[string]string{},
			aliases:        map[string]string{},
			typeName:       "Shopware\\Core\\Content\\Category\\CategoryEntity",
			expectedResult: "Shopware\\Core\\Content\\Category\\CategoryEntity",
		},
		{
			name:           "No namespace",
			namespace:      "",
			useStatements:  map[string]string{},
			aliases:        map[string]string{},
			typeName:       "ProductEntity",
			expectedResult: "ProductEntity",
		},
	}

	// Run all test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewAliasResolver(tt.namespace, tt.useStatements, tt.aliases)
			result := resolver.ResolveType(tt.typeName)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}

func TestIsPrimitiveType(t *testing.T) {
	// Test primitive types
	primitiveTypes := []string{
		"string", "int", "integer", "float", "double", "bool", "boolean",
		"array", "object", "callable", "iterable", "void", "null",
		"mixed", "never", "resource", "false", "true", "number",
	}

	for _, typeName := range primitiveTypes {
		t.Run(typeName, func(t *testing.T) {
			assert.True(t, isPrimitiveType(typeName))
		})
	}

	// Test non-primitive types
	nonPrimitiveTypes := []string{
		"Request", "ProductEntity", "Shopware\\Core\\Content\\Product\\ProductEntity",
		"self", "static", "parent", "$this",
	}

	for _, typeName := range nonPrimitiveTypes {
		t.Run(typeName, func(t *testing.T) {
			assert.False(t, isPrimitiveType(typeName))
		})
	}
}

func TestIsSpecialType(t *testing.T) {
	// Test special types
	specialTypes := []string{
		"self", "static", "parent", "$this", "class-string", "array-key",
	}

	for _, typeName := range specialTypes {
		t.Run(typeName, func(t *testing.T) {
			assert.True(t, isSpecialType(typeName))
		})
	}

	// Test non-special types
	nonSpecialTypes := []string{
		"string", "int", "array", "Request", "ProductEntity",
		"Shopware\\Core\\Content\\Product\\ProductEntity",
	}

	for _, typeName := range nonSpecialTypes {
		t.Run(typeName, func(t *testing.T) {
			assert.False(t, isSpecialType(typeName))
		})
	}
}

// BenchmarkResolveType benchmarks the ResolveType function with caching
func BenchmarkResolveType(b *testing.B) {
	namespace := "Shopware\\Core\\Content\\Product\\Aggregate\\ProductManufacturer"
	useStatements := map[string]string{
		"Request":          "Symfony\\Component\\HttpFoundation\\Request",
		"Response":         "Symfony\\Component\\HttpFoundation\\Response",
		"EntityRepository": "Shopware\\Core\\Framework\\DataAbstractionLayer\\EntityRepository",
		"EntityDefinition": "Shopware\\Core\\Framework\\DataAbstractionLayer\\EntityDefinition",
		"EventDispatcher":  "Symfony\\Component\\EventDispatcher\\EventDispatcher",
	}
	aliases := map[string]string{
		"HttpRequest":  "Symfony\\Component\\HttpFoundation\\Request",
		"HttpResponse": "Symfony\\Component\\HttpFoundation\\Response",
	}

	resolver := NewAliasResolver(namespace, useStatements, aliases)

	// Types to resolve - mix of cached hits and namespace concatenation
	types := []string{
		"Request", "Response", "EntityRepository", "EntityDefinition",
		"ProductManufacturerEntity", "ProductManufacturerCollection",
		"HttpRequest", "HttpResponse", "EventDispatcher",
		"string", "int", "array", "bool", // primitives (early return)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, typeName := range types {
			_ = resolver.ResolveType(typeName)
		}
	}
}

// BenchmarkResolveTypeRepeated benchmarks resolving the same type repeatedly (cache hits)
func BenchmarkResolveTypeRepeated(b *testing.B) {
	namespace := "Shopware\\Core\\Content\\Product\\Aggregate\\ProductManufacturer"
	useStatements := map[string]string{}
	aliases := map[string]string{}

	resolver := NewAliasResolver(namespace, useStatements, aliases)

	// Warm up the cache
	_ = resolver.ResolveType("ProductManufacturerEntity")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// This should hit the cache every time
		_ = resolver.ResolveType("ProductManufacturerEntity")
	}
}

func TestComplexAliasScenarios(t *testing.T) {
	// Test cases for more complex alias scenarios
	tests := []struct {
		name           string
		namespace      string
		useStatements  map[string]string
		aliases        map[string]string
		typeName       string
		expectedResult string
	}{
		{
			name:      "Multiple use statements with same class name",
			namespace: "App\\Controller",
			useStatements: map[string]string{
				"Request":        "Symfony\\Component\\HttpFoundation\\Request",
				"EntityRequest":  "App\\Entity\\Request",
				"ServiceRequest": "App\\Service\\Request",
			},
			aliases: map[string]string{
				"HttpRequest": "Symfony\\Component\\HttpFoundation\\Request",
			},
			typeName:       "Request",
			expectedResult: "Symfony\\Component\\HttpFoundation\\Request",
		},
		{
			name:      "Nested namespaces",
			namespace: "App\\Controller\\Admin",
			useStatements: map[string]string{
				"User": "App\\Entity\\User",
			},
			aliases:        map[string]string{},
			typeName:       "UserController",
			expectedResult: "App\\Controller\\Admin\\UserController",
		},
		{
			name:      "Special alias handling",
			namespace: "Shopware\\Core\\Content\\Product\\Test",
			useStatements: map[string]string{
				"Request": "Symfony\\Component\\HttpFoundation\\Request",
			},
			aliases: map[string]string{
				"SymfonyRequest": "Symfony\\Component\\HttpFoundation\\Request",
			},
			typeName:       "SymfonyRequest",
			expectedResult: "Symfony\\Component\\HttpFoundation\\Request",
		},
	}

	// Run all test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := NewAliasResolver(tt.namespace, tt.useStatements, tt.aliases)
			result := resolver.ResolveType(tt.typeName)
			assert.Equal(t, tt.expectedResult, result)
		})
	}
}
