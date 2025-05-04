package php

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnionType(t *testing.T) {
	testCases := []struct{
		name string
		typeName string
		expectedName string
		expectedTypes int
	}{
		{
			name: "simple union type",
			typeName: "string|int",
			expectedName: "int|string", // Note: sorted alphabetically
			expectedTypes: 2,
		},
		{
			name: "complex union type",
			typeName: "array|bool|float|int|string",
			expectedName: "array|bool|float|int|string",
			expectedTypes: 5,
		},
		{
			name: "union with object types",
			typeName: "string|\\Foo\\Bar",
			expectedName: "\\Foo\\Bar|string", // sorted alphabetically
			expectedTypes: 2,
		},
		{
			name: "union with array type",
			typeName: "array|string[]",
			expectedName: "array|string[]",
			expectedTypes: 2,
		},
		{
			name: "nullable union type",
			typeName: "?string|int",
			expectedName: "int|null|string", // includes null type
			expectedTypes: 3,
		},
		{
			name: "nullable type normalized to union",
			typeName: "?string",
			expectedName: "null|string", // normalized to union
			expectedTypes: 2,
		},
		{
			name: "nullable array type",
			typeName: "?string[]",
			expectedName: "null|string[]", // normalized to union
			expectedTypes: 2,
		},
		{
			name: "nullable object type",
			typeName: "?\\Foo\\Bar",
			expectedName: "\\Foo\\Bar|null", // normalized to union
			expectedTypes: 2,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			phpType := NewPHPType(tc.typeName)
			
			// Check that it's a union type
			unionType, ok := phpType.(*UnionType)
			assert.True(t, ok, "Expected a UnionType, got %T", phpType)
			
			// Check the name
			assert.Equal(t, tc.expectedName, unionType.Name())
			
			// Check the number of types
			assert.Equal(t, tc.expectedTypes, len(unionType.types))
		})
	}
}

func TestNullableTypeMatching(t *testing.T) {
	testCases := []struct{
		name string
		type1 string
		type2 string
		expectedMatch bool
	}{
		{
			name: "nullable string matches string",
			type1: "?string",
			type2: "string",
			expectedMatch: true,
		},
		{
			name: "string doesn't match nullable string",
			type1: "string",
			type2: "?string",
			expectedMatch: false,
		},
		{
			name: "nullable string matches null",
			type1: "?string",
			type2: "null",
			expectedMatch: true,
		},
		{
			name: "null matches nullable string",
			type1: "null",
			type2: "?string",
			expectedMatch: true,
		},
		{
			name: "normalized nullable object type matches original object type",
			type1: "?\\Foo\\Bar",
			type2: "\\Foo\\Bar",
			expectedMatch: true,
		},
		{
			name: "nullable union type matches any of its components",
			type1: "?string|int",
			type2: "string",
			expectedMatch: true,
		},
		{
			name: "nullable union type matches null",
			type1: "?string|int",
			type2: "null",
			expectedMatch: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			type1 := NewPHPType(tc.type1)
			type2 := NewPHPType(tc.type2)
			
			result := type1.Matches(type2)
			assert.Equal(t, tc.expectedMatch, result, "Expected %s.Matches(%s) to be %v", tc.type1, tc.type2, tc.expectedMatch)
		})
	}
}

func TestUnionType_Matches(t *testing.T) {
	testCases := []struct{
		name string
		type1 string
		type2 string
		expectedMatch bool
	}{
		{
			name: "same union types match",
			type1: "string|int",
			type2: "string|int",
			expectedMatch: true,
		},
		{
			name: "different order union types match",
			type1: "string|int",
			type2: "int|string",
			expectedMatch: true,
		},
		{
			name: "union type matches non-union if one type matches",
			type1: "string|int",
			type2: "string",
			expectedMatch: true,
		},
		{
			name: "non-union type matches union if it matches one type",
			type1: "string|int", // Swapped to make union the first type
			type2: "string",
			expectedMatch: true,
		},
		{
			name: "subset union types match",
			type1: "string|int",
			type2: "string|int|float",
			expectedMatch: true,
		},
		{
			name: "union types with no overlapping types don't match",
			type1: "string|bool",  // no overlapping types with type2
			type2: "int|float",
			expectedMatch: false,
		},
		{
			name: "union types with overlapping types match",
			type1: "string|int",
			type2: "int|float",
			expectedMatch: true,
		},
		{
			name: "mixed matches any union type",
			type1: "mixed",
			type2: "string|int",
			expectedMatch: true,
		},
		{
			name: "any union type matches mixed",
			type1: "string|int",
			type2: "mixed",
			expectedMatch: true,
		},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			type1 := NewPHPType(tc.type1)
			type2 := NewPHPType(tc.type2)
			
			result := type1.Matches(type2)
			assert.Equal(t, tc.expectedMatch, result)
		})
	}
}

func TestPHPType_Matches(t *testing.T) {
	tests := []struct {
		name     string
		type1    PHPType
		type2    PHPType
		expected bool
	}{
		// String type tests
		{
			name:     "string matches string",
			type1:    NewStringType(false),
			type2:    NewStringType(false),
			expected: true,
		},
		{
			name:     "nullable string matches string",
			type1:    NewStringType(true),
			type2:    NewStringType(false),
			expected: true,
		},
		{
			name:     "non-nullable string doesn't match nullable string",
			type1:    NewStringType(false),
			type2:    NewStringType(true),
			expected: false,
		},
		{
			name:     "string doesn't match int",
			type1:    NewStringType(false),
			type2:    NewIntType(false),
			expected: false,
		},

		// Integer type tests
		{
			name:     "int matches int",
			type1:    NewIntType(false),
			type2:    NewIntType(false),
			expected: true,
		},
		{
			name:     "int matches float",
			type1:    NewIntType(false),
			type2:    NewFloatType(false),
			expected: true, // int can be implicitly converted to float
		},
		{
			name:     "float doesn't match int",
			type1:    NewFloatType(false),
			type2:    NewIntType(false),
			expected: false, // float doesn't implicitly convert to int
		},

		// Array type tests
		{
			name:     "string[] matches string[]",
			type1:    NewArrayType(NewStringType(false), false),
			type2:    NewArrayType(NewStringType(false), false),
			expected: true,
		},
		{
			name:     "string[] doesn't match int[]",
			type1:    NewArrayType(NewStringType(false), false),
			type2:    NewArrayType(NewIntType(false), false),
			expected: false,
		},
		{
			name:     "array matches any array type",
			type1:    NewArrayType(nil, false),
			type2:    NewArrayType(NewStringType(false), false),
			expected: true,
		},
		{
			name:     "array matches iterable",
			type1:    NewArrayType(nil, false),
			type2:    NewIterableType(false),
			expected: true,
		},

		// Object type tests
		{
			name:     "same class names match",
			type1:    NewObjectType("App\\Entity\\User", false),
			type2:    NewObjectType("App\\Entity\\User", false),
			expected: true,
		},
		{
			name:     "different class names don't match",
			type1:    NewObjectType("App\\Entity\\User", false),
			type2:    NewObjectType("App\\Entity\\Product", false),
			expected: false,
		},
		{
			name:     "case-insensitive class names match",
			type1:    NewObjectType("App\\Entity\\User", false),
			type2:    NewObjectType("app\\entity\\user", false),
			expected: true,
		},

		// Mixed type tests
		{
			name:     "mixed matches any type",
			type1:    NewMixedType(),
			type2:    NewStringType(false),
			expected: true,
		},
		{
			name:     "any type matches mixed",
			type1:    NewStringType(false),
			type2:    NewMixedType(),
			expected: true,
		},

		// Null and nullable tests
		{
			name:     "null matches nullable string",
			type1:    NewNullType(),
			type2:    NewStringType(true),
			expected: true,
		},
		{
			name:     "null doesn't match non-nullable string",
			type1:    NewNullType(),
			type2:    NewStringType(false),
			expected: false,
		},
		{
			name:     "nullable string matches null",
			type1:    NewStringType(true),
			type2:    NewNullType(),
			expected: true,
		},

		// Special types
		{
			name:     "self matches self",
			type1:    NewSpecialType("self"),
			type2:    NewSpecialType("self"),
			expected: true,
		},
		{
			name:     "self doesn't match static",
			type1:    NewSpecialType("self"),
			type2:    NewSpecialType("static"),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.type1.Matches(tt.type2))
		})
	}
}

func TestNewPHPType(t *testing.T) {
	tests := []struct {
		name           string
		typeName       string
		expectedType   string
		expectedStruct interface{}
	}{
		{
			name:           "string",
			typeName:       "string",
			expectedType:   "string",
			expectedStruct: &StringType{},
		},
		{
			name:           "nullable string",
			typeName:       "?string",
			expectedType:   "null|string",
			expectedStruct: &UnionType{},
		},
		{
			name:           "int",
			typeName:       "int",
			expectedType:   "int",
			expectedStruct: &IntType{},
		},
		{
			name:           "integer alias",
			typeName:       "integer",
			expectedType:   "int",
			expectedStruct: &IntType{},
		},
		{
			name:           "float",
			typeName:       "float",
			expectedType:   "float",
			expectedStruct: &FloatType{},
		},
		{
			name:           "double alias",
			typeName:       "double",
			expectedType:   "float",
			expectedStruct: &FloatType{},
		},
		{
			name:           "boolean",
			typeName:       "boolean",
			expectedType:   "bool",
			expectedStruct: &BoolType{},
		},
		{
			name:           "array",
			typeName:       "array",
			expectedType:   "array",
			expectedStruct: &ArrayType{},
		},
		{
			name:           "typed array",
			typeName:       "string[]",
			expectedType:   "string[]",
			expectedStruct: &ArrayType{},
		},
		{
			name:           "object",
			typeName:       "object",
			expectedType:   "object",
			expectedStruct: &ObjectType{},
		},
		{
			name:           "class name",
			typeName:       "App\\Entity\\User",
			expectedType:   "App\\Entity\\User",
			expectedStruct: &ObjectType{},
		},
		{
			name:           "void",
			typeName:       "void",
			expectedType:   "void",
			expectedStruct: &VoidType{},
		},
		{
			name:           "null",
			typeName:       "null",
			expectedType:   "null",
			expectedStruct: &NullType{},
		},
		{
			name:           "mixed",
			typeName:       "mixed",
			expectedType:   "mixed",
			expectedStruct: &MixedType{},
		},
		{
			name:           "special type - self",
			typeName:       "self",
			expectedType:   "self",
			expectedStruct: &SpecialType{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			phpType := NewPHPType(tt.typeName)
			assert.IsType(t, tt.expectedStruct, phpType)
			assert.Equal(t, tt.expectedType, phpType.Name())
		})
	}
}
