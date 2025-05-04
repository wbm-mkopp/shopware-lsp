package php

import (
	"sort"
	"strings"
)

// PHPType represents a PHP type with methods to compare and match against other types
type PHPType interface {
	// Name returns the string representation of the type
	Name() string
	
	// Matches determines if this type matches another type
	// For example, 'int' would match 'integer' or 'mixed' would match any type
	Matches(other PHPType) bool
}

// BaseType provides common functionality for PHP types
type BaseType struct {
	name string
}

// Name returns the string name of the type
func (t *BaseType) Name() string {
	return t.name
}

// NewPHPType creates a new PHPType based on the provided type name
func NewPHPType(typeName string) PHPType {
	// Handle nullable types (e.g., ?string, ?int, ?string|int)
	isNullable := false
	if strings.HasPrefix(typeName, "?") {
		isNullable = true
		typeName = typeName[1:]
	}

	// Handle union types (e.g., string|int, Foo|Bar)
	if strings.Contains(typeName, "|") {
		typeNames := strings.Split(typeName, "|")
		types := make([]PHPType, 0, len(typeNames))
		
		// Add all types from the union
		for _, name := range typeNames {
			types = append(types, NewPHPType(name))
		}
		
		// If the entire union is nullable, add null type
		if isNullable {
			hasNullType := false
			
			// Check if null type already exists in the union
			for _, t := range types {
				if _, ok := t.(*NullType); ok {
					hasNullType = true
					break
				}
			}
			
			// Add null type if not already present
			if !hasNullType {
				types = append(types, NewNullType())
			}
		}
		
		return NewUnionType(types)
	}

	// Handle fully qualified class names
	if strings.Contains(typeName, "\\") {
		// If nullable, create a union with null
		if isNullable {
			return NewUnionType([]PHPType{NewObjectType(typeName, false), NewNullType()})
		} else {
			return NewObjectType(typeName, false)
		}
	}

	// Handle array types (e.g., string[], int[])
	if strings.HasSuffix(typeName, "[]") {
		elementTypeName := strings.TrimSuffix(typeName, "[]")
		elementType := NewPHPType(elementTypeName)
		// If nullable, create a union with null
		if isNullable {
			return NewUnionType([]PHPType{NewArrayType(elementType, false), NewNullType()})
		} else {
			return NewArrayType(elementType, false)
		}
	}

	// For all other types, construct the base type and handle nullable as a union
	var baseType PHPType
	// Create the base type (always non-nullable)
	switch strings.ToLower(typeName) {
	case "string":
		baseType = NewStringType(false)
	case "int", "integer":
		baseType = NewIntType(false)
	case "float", "double":
		baseType = NewFloatType(false)
	case "bool", "boolean":
		baseType = NewBoolType(false)
	case "array":
		baseType = NewArrayType(nil, false)
	case "object":
		baseType = NewObjectType("object", false)
	case "callable":
		baseType = NewCallableType(false)
	case "iterable":
		baseType = NewIterableType(false)
	case "void":
		// void can't be nullable
		return NewVoidType()
	case "null":
		// null is already null
		return NewNullType()
	case "mixed":
		// mixed already includes null
		return NewMixedType()
	case "never":
		// never can't be nullable
		return NewNeverType()
	case "self", "static", "parent", "$this":
		baseType = NewSpecialType(typeName)
	default:
		// If not a recognized primitive type, assume it's a class/interface
		baseType = NewObjectType(typeName, false)
	}
	
	// If nullable, create a union with null
	if isNullable {
		return NewUnionType([]PHPType{baseType, NewNullType()})
	}
	
	return baseType
}

// StringType represents the PHP string type
type StringType struct {
	BaseType
	nullable bool
}

// NewStringType creates a new string type
func NewStringType(nullable bool) *StringType {
	name := "string"
	if nullable {
		name = "?" + name
	}
	return &StringType{
		BaseType: BaseType{name: name},
		nullable: nullable,
	}
}

// Matches checks if this type matches another type
func (t *StringType) Matches(other PHPType) bool {
	switch o := other.(type) {
	case *StringType:
		// A nullable string matches a non-nullable string
		return !o.nullable || t.nullable
	case *MixedType:
		return true
	case *NullType:
		return t.nullable
	default:
		return false
	}
}

// IntType represents the PHP integer type
type IntType struct {
	BaseType
	nullable bool
}

// NewIntType creates a new integer type
func NewIntType(nullable bool) *IntType {
	name := "int"
	if nullable {
		name = "?" + name
	}
	return &IntType{
		BaseType: BaseType{name: name},
		nullable: nullable,
	}
}

// Matches checks if this type matches another type
func (t *IntType) Matches(other PHPType) bool {
	// Special case for union types
	if unionType, ok := other.(*UnionType); ok {
		// Check if any of the union's types match our int type
		for _, typ := range unionType.types {
			if t.Matches(typ) {
				return true
			}
		}
		return false
	}

	switch o := other.(type) {
	case *IntType:
		// int matches int
		return !o.nullable || t.nullable
	case *FloatType:
		// int matches float (but not the other way around)
		return !o.nullable || t.nullable
	case *MixedType:
		// int matches mixed
		return true
	case *NullType:
		// int doesn't match null unless nullable
		return t.nullable
	default:
		return false
	}
}

// FloatType represents the PHP float type
type FloatType struct {
	BaseType
	nullable bool
}

// NewFloatType creates a new float type
func NewFloatType(nullable bool) *FloatType {
	name := "float"
	if nullable {
		name = "?" + name
	}
	return &FloatType{
		BaseType: BaseType{name: name},
		nullable: nullable,
	}
}

// Matches checks if this type matches another type
func (t *FloatType) Matches(other PHPType) bool {
	switch o := other.(type) {
	case *FloatType:
		return !o.nullable || t.nullable
	case *IntType:
		// Float doesn't match int (no implicit downcast)
		return false
	case *MixedType:
		return true
	case *NullType:
		return t.nullable
	default:
		return false
	}
}

// BoolType represents the PHP boolean type
type BoolType struct {
	BaseType
	nullable bool
}

// NewBoolType creates a new boolean type
func NewBoolType(nullable bool) *BoolType {
	name := "bool"
	if nullable {
		name = "?" + name
	}
	return &BoolType{
		BaseType: BaseType{name: name},
		nullable: nullable,
	}
}

// Matches checks if this type matches another type
func (t *BoolType) Matches(other PHPType) bool {
	switch o := other.(type) {
	case *BoolType:
		return !o.nullable || t.nullable
	case *MixedType:
		return true
	case *NullType:
		return t.nullable
	default:
		return false
	}
}

// ArrayType represents the PHP array type
type ArrayType struct {
	BaseType
	elementType PHPType // Can be nil for generic arrays
	nullable    bool
}

// NewArrayType creates a new array type
func NewArrayType(elementType PHPType, nullable bool) *ArrayType {
	var name string
	if elementType != nil {
		name = elementType.Name() + "[]"
	} else {
		name = "array"
	}
	if nullable {
		name = "?" + name
	}
	return &ArrayType{
		BaseType:    BaseType{name: name},
		elementType: elementType,
		nullable:    nullable,
	}
}

// Matches checks if this type matches another type
func (t *ArrayType) Matches(other PHPType) bool {
	switch o := other.(type) {
	case *ArrayType:
		// If either array doesn't specify an element type, or the element types match
		if t.elementType == nil || o.elementType == nil {
			return !o.nullable || t.nullable
		}
		return t.elementType.Matches(o.elementType) && (!o.nullable || t.nullable)
	case *IterableType:
		return true // Arrays are iterable
	case *MixedType:
		return true
	case *NullType:
		return t.nullable
	default:
		return false
	}
}

// ObjectType represents a PHP class/interface type
type ObjectType struct {
	BaseType
	className string
	nullable  bool
}

// NewObjectType creates a new object type
func NewObjectType(className string, nullable bool) *ObjectType {
	name := className
	if nullable {
		name = "?" + name
	}
	return &ObjectType{
		BaseType:  BaseType{name: name},
		className: className,
		nullable:  nullable,
	}
}

// Matches checks if this type matches another type
func (t *ObjectType) Matches(other PHPType) bool {
	switch o := other.(type) {
	case *ObjectType:
		// For now, simply check if the class names match
		// TODO: Add proper inheritance checks when class hierarchy is available
		return strings.EqualFold(t.className, o.className) && (!o.nullable || t.nullable)
	case *MixedType:
		return true
	case *NullType:
		return t.nullable
	default:
		return false
	}
}

// CallableType represents the PHP callable type
type CallableType struct {
	BaseType
	nullable bool
}

// NewCallableType creates a new callable type
func NewCallableType(nullable bool) *CallableType {
	name := "callable"
	if nullable {
		name = "?" + name
	}
	return &CallableType{
		BaseType: BaseType{name: name},
		nullable: nullable,
	}
}

// Matches checks if this type matches another type
func (t *CallableType) Matches(other PHPType) bool {
	switch o := other.(type) {
	case *CallableType:
		return !o.nullable || t.nullable
	case *MixedType:
		return true
	case *NullType:
		return t.nullable
	default:
		return false
	}
}

// IterableType represents the PHP iterable type
type IterableType struct {
	BaseType
	nullable bool
}

// NewIterableType creates a new iterable type
func NewIterableType(nullable bool) *IterableType {
	name := "iterable"
	if nullable {
		name = "?" + name
	}
	return &IterableType{
		BaseType: BaseType{name: name},
		nullable: nullable,
	}
}

// Matches checks if this type matches another type
func (t *IterableType) Matches(other PHPType) bool {
	switch o := other.(type) {
	case *IterableType:
		return !o.nullable || t.nullable
	case *ArrayType:
		// Arrays are iterable, but iterable is not necessarily an array
		return t.nullable || !o.nullable
	case *MixedType:
		return true
	case *NullType:
		return t.nullable
	default:
		return false
	}
}

// VoidType represents the PHP void type
type VoidType struct {
	BaseType
}

// NewVoidType creates a new void type
func NewVoidType() *VoidType {
	return &VoidType{
		BaseType: BaseType{name: "void"},
	}
}

// Matches checks if this type matches another type
func (t *VoidType) Matches(other PHPType) bool {
	switch other.(type) {
	case *VoidType:
		return true
	case *MixedType:
		return true
	default:
		return false
	}
}

// NullType represents the PHP null type
type NullType struct {
	BaseType
}

// NewNullType creates a new null type
func NewNullType() *NullType {
	return &NullType{
		BaseType: BaseType{name: "null"},
	}
}

// Matches checks if this type matches another type
func (t *NullType) Matches(other PHPType) bool {
	switch o := other.(type) {
	case *NullType:
		return true
	case *StringType, *IntType, *FloatType, *BoolType, *ArrayType, *ObjectType, *CallableType, *IterableType:
		// null matches any nullable type
		return getTypeNullability(o)
	case *UnionType:
		// Check if any of the types in the union is a null type
		for _, unionType := range o.types {
			if _, ok := unionType.(*NullType); ok {
				return true
			}
		}
		return false
	case *MixedType:
		return true
	default:
		return false
	}
}

// MixedType represents the PHP mixed type
type MixedType struct {
	BaseType
}

// NewMixedType creates a new mixed type
func NewMixedType() *MixedType {
	return &MixedType{
		BaseType: BaseType{name: "mixed"},
	}
}

// Matches checks if this type matches another type
func (t *MixedType) Matches(other PHPType) bool {
	// mixed matches any type
	return true
}

// NeverType represents the never return type (PHP 8.1+)
type NeverType struct {
	BaseType
}

// NewNeverType creates a new never type
func NewNeverType() *NeverType {
	return &NeverType{
		BaseType: BaseType{name: "never"},
	}
}

// Matches checks if this type matches another type
func (t *NeverType) Matches(other PHPType) bool {
	_, ok := other.(*NeverType)
	return ok
}

// UnionType represents a PHP union type (e.g., string|int)
type UnionType struct {
	BaseType
	types []PHPType
}

// NewUnionType creates a new union type
func NewUnionType(types []PHPType) *UnionType {
	// Build a sorted list of type names to ensure consistent naming
	names := make([]string, 0, len(types))
	for _, t := range types {
		names = append(names, t.Name())
	}
	sort.Strings(names)
	
	// Join the names with | as per PHP syntax
	name := strings.Join(names, "|")
	
	return &UnionType{
		BaseType: BaseType{name: name},
		types: types,
	}
}

// Matches checks if this type matches another type
func (t *UnionType) Matches(other PHPType) bool {
	// Special case: mixed always matches everything
	if _, ok := other.(*MixedType); ok {
		return true
	}

	// If other is also a union type, check if there's any overlap between types
	if otherUnion, ok := other.(*UnionType); ok {
		// Check if types like string|bool and int|float have no overlap
		// For union types to match, at least one of our types needs to match
		// with one of their types
		
		// Special case for int|float: there is a special matching rule where
		// int matches float but float doesn't match int
		hasOverlap := false
		for _, ourType := range t.types {
			for _, theirType := range otherUnion.types {
				// Check both directions to handle special cases like int/float
				if ourType.Matches(theirType) {
					hasOverlap = true
					break
				}
			}
			if hasOverlap {
				break
			}
		}
		return hasOverlap
	}
	
	// If other is a simple type, check if any of our types match it
	for _, typ := range t.types {
		if typ.Matches(other) {
			return true
		}
	}
	
	return false
}

// SpecialType represents special PHP types like self, static, parent, $this
type SpecialType struct {
	BaseType
}

// NewSpecialType creates a new special type
func NewSpecialType(typeName string) *SpecialType {
	return &SpecialType{
		BaseType: BaseType{name: typeName},
	}
}

// Matches checks if this type matches another type
func (t *SpecialType) Matches(other PHPType) bool {
	switch o := other.(type) {
	case *SpecialType:
		return t.Name() == o.Name()
	case *MixedType:
		return true
	default:
		return false
	}
}

// Helper function to get nullability from different type implementations
func getTypeNullability(t PHPType) bool {
	switch o := t.(type) {
	case *StringType:
		return o.nullable
	case *IntType:
		return o.nullable
	case *FloatType:
		return o.nullable
	case *BoolType:
		return o.nullable
	case *ArrayType:
		return o.nullable
	case *ObjectType:
		return o.nullable
	case *CallableType:
		return o.nullable
	case *IterableType:
		return o.nullable
	default:
		return false
	}
}
