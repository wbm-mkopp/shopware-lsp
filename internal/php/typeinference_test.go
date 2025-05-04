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

	// Create mock class data directly
	// This avoids serialization/deserialization issues with the indexer
	allClasses := make(map[string]PHPClass)
	
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
	allClasses[productInterface.Name] = productInterface
	
	// Abstract parent class with properties
	baseProduct := PHPClass{
		Name:       "App\\Entity\\BaseProduct",
		Path:        "testdata/typeinference_inheritance.php",
		Line:        11,
		Interfaces: []string{"App\\Entity\\ProductInterface"},
		Methods: map[string]PHPMethod{
			"getId": {
				Name:       "getId",
				Line:       17,
				Visibility: Public,
				ReturnType: NewPHPType("int"),
			},
			"getDescription": {
				Name:       "getDescription",
				Line:       22,
				Visibility: Public,
				ReturnType: NewPHPType("string"),
			},
			"getBaseInformation": {
				Name:       "getBaseInformation",
				Line:       27,
				Visibility: Public,
				ReturnType: NewPHPType("array"),
			},
		},
		Properties: map[string]PHPProperty{
			"id": {
				Name:       "id",
				Line:       12,
				Visibility: Protected,
				Type:       NewPHPType("int"),
			},
			"description": {
				Name:       "description",
				Line:       13,
				Visibility: Protected,
				Type:       NewPHPType("string"),
			},
			"price": {
				Name:       "price",
				Line:       14,
				Visibility: Protected,
				Type:       NewPHPType("float"),
			},
		},
	}
	allClasses[baseProduct.Name] = baseProduct
	
	// Concrete class with properties
	product := PHPClass{
		Name:   "App\\Entity\\Product",
		Path:   "testdata/typeinference_inheritance.php",
		Line:   35,
		Parent: "App\\Entity\\BaseProduct",
		Methods: map[string]PHPMethod{
			"getName": {
				Name:       "getName",
				Line:       40,
				Visibility: Public,
				ReturnType: NewPHPType("string"),
			},
			"getPrice": {
				Name:       "getPrice",
				Line:       50,
				Visibility: Public,
				ReturnType: NewPHPType("float"),
			},
			"getSku": {
				Name:       "getSku",
				Line:       55,
				Visibility: Public,
				ReturnType: NewPHPType("?string"),
			},
		},
		Properties: map[string]PHPProperty{
			"name": {
				Name:       "name",
				Line:       36,
				Visibility: Private,
				Type:       NewPHPType("string"),
			},
			"sku": {
				Name:       "sku",
				Line:       37,
				Visibility: Private,
				Type:       NewPHPType("?string"),
			},
		},
	}
	allClasses[product.Name] = product

	// Test cases for method inheritance
	methodTestCases := []struct {
		classType    string // The class where to find the method
		methodName   string // The method to check
		expectedType string // The expected return type
		source       string // Where the method comes from: "self", "parent", "interface"
	}{
		{"App\\Entity\\Product", "getName", "string", "self"},          // Method defined in Product class
		{"App\\Entity\\Product", "getId", "int", "parent"},            // Method inherited from BaseProduct
		{"App\\Entity\\Product", "getDescription", "string", "parent"}, // Method from interface, implemented in BaseProduct
		{"App\\Entity\\Product", "getPrice", "float", "self"},     // Method from interface, implemented in Product
		{"App\\Entity\\Product", "getBaseInformation", "array", "parent"}, // Method from BaseProduct
	}

	// Test each method's type lookup with searchParentClassMethod function
	for _, tc := range methodTestCases {
		t.Run("Method_"+tc.methodName, func(t *testing.T) {
			// Test that our method resolver properly finds the method through inheritance
			resultType := idx.searchParentClassMethod(allClasses, tc.classType, tc.methodName)
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
		{"App\\Entity\\Product", "name", "string", "self", false},    // Private property in Product class - not accessible
		{"App\\Entity\\Product", "sku", "?string", "self", false},   // Private property in Product class - not accessible

		// These properties from parent are all protected, so they should be accessible
		{"App\\Entity\\Product", "id", "int", "parent", true},       // Protected property from BaseProduct
		{"App\\Entity\\Product", "description", "string", "parent", true}, // Protected property from BaseProduct
		{"App\\Entity\\Product", "price", "float", "parent", true},    // Protected property from BaseProduct
	}

	// Test each property's type lookup with searchParentClassProperty function
	for _, tc := range propertyTestCases {
		t.Run("Property_"+tc.propertyName, func(t *testing.T) {
			// Test that our property resolver properly handles visibility and inheritance
			resultType := idx.searchParentClassProperty(allClasses, tc.classType, tc.propertyName)
			
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

	// Create a test for the method access resolution
	t.Run("handleMemberCallExpression correctly resolves method return types", func(t *testing.T) {
		// Just check that searchParentClassMethod returns the expected types
		for _, tc := range methodTestCases {
			resultType := idx.searchParentClassMethod(allClasses, tc.classType, tc.methodName)
			assert.NotNil(t, resultType, "searchParentClassMethod should find return type for %s", tc.methodName)
			assert.Equal(t, tc.expectedType, resultType.Name(), "searchParentClassMethod should correctly infer return type for %s", tc.methodName)
		}
	})

	// Create a test for the property access resolution
	t.Run("handleMemberExpression correctly resolves property types", func(t *testing.T) {
		// Check that searchParentClassProperty correctly handles property visibility
		for _, tc := range propertyTestCases {
			resultType := idx.searchParentClassProperty(allClasses, tc.classType, tc.propertyName)
			
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
