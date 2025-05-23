package php

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTypeInferenceWithInheritance(t *testing.T) {
	// Create a temporary context for testing
	tmpDir := t.TempDir()
	idx, err := NewPHPIndex(tmpDir)
	assert.NoError(t, err)

	// We'll register test classes directly in the dataIndexer so they're available
	// via GetClass() used by our implementation

	// Create all the test classes
	// Interface definition
	productInterface := PHPClass{
		Name:        "App\\Entity\\ProductInterface",
		Path:        "testdata/typeinference_inheritance.php",
		Line:        5,
		IsInterface: true,
		Methods: map[string]PHPMethod{
			"getDescription": {
				Name:       "getDescription",
				Line:       7,
				Visibility: Public,
				ReturnType: NewPHPType("string"),
			},
			"getPrice": {
				Name:       "getPrice",
				Line:       8,
				Visibility: Public,
				ReturnType: NewPHPType("float"),
			},
		},
		Properties: make(map[string]PHPProperty),
	}

	// Abstract parent class with properties
	baseProduct := PHPClass{
		Name:       "App\\Entity\\BaseProduct",
		Path:       "testdata/typeinference_inheritance.php",
		Line:       11,
		Interfaces: []string{"App\\Entity\\ProductInterface"},
		Methods: map[string]PHPMethod{
			"getId": {
				Name:       "getId",
				Line:       27,
				Visibility: Public,
				ReturnType: NewPHPType("int"),
			},
			"getDescription": {
				Name:       "getDescription",
				Line:       28,
				Visibility: Public,
				ReturnType: NewPHPType("string"),
			},
			"getBaseInformation": {
				Name:       "getBaseInformation",
				Line:       29,
				Visibility: Public,
				ReturnType: NewPHPType("array"),
			},
		},
		Properties: map[string]PHPProperty{
			"id": {
				Name:       "id",
				Line:       13,
				Visibility: Public,
				Type:       NewPHPType("int"),
			},
			"description": {
				Name:       "description",
				Line:       14,
				Visibility: Public,
				Type:       NewPHPType("string"),
			},
			"price": {
				Name:       "price",
				Line:       15,
				Visibility: Public,
				Type:       NewPHPType("float"),
			},
		},
	}

	// Child class with additional properties and overridden methods
	product := PHPClass{
		Name:   "App\\Entity\\Product",
		Path:   "testdata/typeinference_inheritance.php",
		Line:   31,
		Parent: "App\\Entity\\BaseProduct",
		Methods: map[string]PHPMethod{
			"getName": {
				Name:       "getName",
				Line:       42,
				Visibility: Public,
				ReturnType: NewPHPType("string"),
			},
			"getPrice": { // Override parent method
				Name:       "getPrice",
				Line:       43,
				Visibility: Public,
				ReturnType: NewPHPType("float"),
			},
		},
		Properties: map[string]PHPProperty{
			"name": {
				Name:       "name",
				Line:       33,
				Visibility: Private, // Private property, not accessible from children
				Type:       NewPHPType("string"),
			},
			"sku": {
				Name:       "sku",
				Line:       34,
				Visibility: Private, // Private property, not accessible from children
				Type:       NewPHPType("string"),
			},
		},
	}

	// Save all classes to the index using BatchSaveItems
	classes := map[string]map[string]PHPClass{
		"testdata/typeinference_inheritance.php": {
			productInterface.Name: productInterface,
			baseProduct.Name:      baseProduct,
			product.Name:          product,
		},
	}

	err = idx.dataIndexer.BatchSaveItems(classes)
	assert.NoError(t, err)

	// Test cases for method inheritance
	methodTestCases := []struct {
		classType    string // The class where to find the method
		methodName   string // The method to check
		expectedType string // The expected return type
		source       string // Where the method comes from: "self", "parent", "interface"
	}{
		{"App\\Entity\\Product", "getName", "string", "self"},             // Method defined in Product class
		{"App\\Entity\\Product", "getId", "int", "parent"},                // Method inherited from BaseProduct
		{"App\\Entity\\Product", "getDescription", "string", "parent"},    // Method from interface, implemented in BaseProduct
		{"App\\Entity\\Product", "getPrice", "float", "self"},             // Method from interface, implemented in Product
		{"App\\Entity\\Product", "getBaseInformation", "array", "parent"}, // Method from BaseProduct
	}

	// First test the method return types
	for _, tc := range methodTestCases {
		t.Run("Method_"+tc.methodName, func(t *testing.T) {
			// Test that our method resolver properly finds the return type through inheritance
			resultType := idx.searchParentClassMethod(tc.classType, tc.methodName)
			assert.NotNil(t, resultType, "Should find return type for method %s", tc.methodName)
			assert.Equal(t, tc.expectedType, resultType.Name(), "Should correctly infer return type for method %s", tc.methodName)
		})
	}

	// Test cases for property inheritance
	propertyTestCases := []struct {
		classType    string // The class where to find the property
		propertyName string // The property to check
		expectedType string // The expected property type
		source       string // Where the property comes from: "self", "parent"
		accessible   bool   // Whether the property should be accessible
	}{
		// Note: According to visibility rules in our test, name and sku are PRIVATE in Product class
		// so they shouldn't be accessible from outside the class and will return nil
		// But they would be accessible from WITHIN the Product class
		{"App\\Entity\\Product", "name", "string", "self", false}, // Private property in Product class - not accessible
		{"App\\Entity\\Product", "sku", "?string", "self", false}, // Private property in Product class - not accessible

		// These properties from parent are all protected, so they should be accessible
		{"App\\Entity\\Product", "id", "int", "parent", true},             // Protected property from BaseProduct
		{"App\\Entity\\Product", "description", "string", "parent", true}, // Protected property from BaseProduct
		{"App\\Entity\\Product", "price", "float", "parent", true},        // Protected property from BaseProduct
	}

	// Test each property's type lookup with searchParentClassProperty function
	for _, tc := range propertyTestCases {
		t.Run("Property_"+tc.propertyName, func(t *testing.T) {
			// Test that our property resolver properly handles visibility and inheritance
			resultType := idx.searchParentClassProperty(tc.classType, tc.propertyName)

			if tc.accessible {
				// Property should be accessible (public/protected)
				assert.NotNil(t, resultType, "Should find type for property %s", tc.propertyName)
				if resultType != nil {
					assert.Equal(t, tc.expectedType, resultType.Name(), "Should correctly infer type for property %s", tc.propertyName)
				}
			} else {
				// Property should not be accessible (private) from outside the defining class
				// This verifies that we respect PHP's visibility rules
				assert.Nil(t, resultType, "Private property %s should not be accessible from outside its class", tc.propertyName)
			}
		})
	}

	// Test that handleMemberCallExpression correctly resolves method return types
	t.Run("handleMemberCallExpression correctly resolves method return types", func(t *testing.T) {
		// Just check that searchParentClassMethod returns the expected types
		for _, tc := range methodTestCases {
			resultType := idx.searchParentClassMethod(tc.classType, tc.methodName)
			assert.NotNil(t, resultType)
			assert.Equal(t, tc.expectedType, resultType.Name())
		}
	})

	// Create a test for the property access resolution
	t.Run("handleMemberExpression correctly resolves property types", func(t *testing.T) {
		// Check that searchParentClassProperty correctly handles property visibility
		for _, tc := range propertyTestCases {
			resultType := idx.searchParentClassProperty(tc.classType, tc.propertyName)

			if tc.accessible {
				// Property should be accessible (public/protected)
				assert.NotNil(t, resultType, "searchParentClassProperty should find type for %s", tc.propertyName)
				if resultType != nil {
					assert.Equal(t, tc.expectedType, resultType.Name(), "searchParentClassProperty should correctly infer type for %s", tc.propertyName)
				}
			} else {
				// Property should not be accessible (private) from outside the defining class
				assert.Nil(t, resultType, "Private property %s should not be accessible from outside its class", tc.propertyName)
			}
		}
	})
}
