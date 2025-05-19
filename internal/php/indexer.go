package php

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/shopware/shopware-lsp/internal/indexer"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
	"github.com/vmihailenco/msgpack/v5"
)

// findChildByKind finds the first child node of the given kind
func findChildByKind(node *tree_sitter.Node, kind string) *tree_sitter.Node {
	if node == nil {
		return nil
	}

	// Check regular children
	childCount := node.ChildCount()
	for i := uint(0); i < uint(childCount); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == kind {
			return child
		}
	}

	// If not found in direct children, try to find in named children
	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Kind() == kind {
			return child
		}
	}

	return nil
}

type PHPClass struct {
	Name        string
	Path        string
	Line        int
	Methods     map[string]PHPMethod
	Properties  map[string]PHPProperty
	Parent      string   // The class this class extends from
	Interfaces  []string // Interfaces this class implements
	IsInterface bool     // Whether this is an interface or a class
}

type PHPMethod struct {
	Name       string
	Line       int
	Visibility Visibility
	ReturnType PHPType
	// Serialization helpers
	ReturnTypeName string
}

// marshalMethod creates a serializable version of PHPMethod
type marshalMethod struct {
	Name           string     `msgpack:"name"`
	Line           int        `msgpack:"line"`
	Visibility     Visibility `msgpack:"visibility"`
	ReturnTypeName string     `msgpack:"return_type_name,omitempty"`
}

// MarshalMsgpack implements msgpack.Marshaler interface
func (m PHPMethod) MarshalMsgpack() ([]byte, error) {
	mm := marshalMethod{
		Name:       m.Name,
		Line:       m.Line,
		Visibility: m.Visibility,
	}

	if m.ReturnType != nil {
		mm.ReturnTypeName = m.ReturnType.Name()
	}

	return msgpack.Marshal(mm)
}

// UnmarshalMsgpack implements msgpack.Unmarshaler interface
func (m *PHPMethod) UnmarshalMsgpack(data []byte) error {
	var mm marshalMethod
	if err := msgpack.Unmarshal(data, &mm); err != nil {
		return err
	}

	m.Name = mm.Name
	m.Line = mm.Line
	m.Visibility = mm.Visibility

	// Reconstruct the return type from the type name
	if mm.ReturnTypeName != "" {
		m.ReturnType = NewPHPType(mm.ReturnTypeName)
	}

	return nil
}

// Visibility constants for PHP properties and methods
const (
	Public Visibility = iota
	Protected
	Private
)

// Visibility represents the visibility level of a PHP element
type Visibility int

type PHPProperty struct {
	Name       string
	Line       int
	Visibility Visibility
	Type       PHPType // The PHP type of the property
	// Serialization helpers
	TypeName string
}

// marshalProperty creates a serializable version of PHPProperty
type marshalProperty struct {
	Name       string     `msgpack:"name"`
	Line       int        `msgpack:"line"`
	Visibility Visibility `msgpack:"visibility"`
	TypeName   string     `msgpack:"type_name,omitempty"`
}

// MarshalMsgpack implements msgpack.Marshaler interface
func (p PHPProperty) MarshalMsgpack() ([]byte, error) {
	mp := marshalProperty{
		Name:       p.Name,
		Line:       p.Line,
		Visibility: p.Visibility,
	}

	if p.Type != nil {
		mp.TypeName = p.Type.Name()
	}

	return msgpack.Marshal(mp)
}

// UnmarshalMsgpack implements msgpack.Unmarshaler interface
func (p *PHPProperty) UnmarshalMsgpack(data []byte) error {
	var mp marshalProperty
	if err := msgpack.Unmarshal(data, &mp); err != nil {
		return err
	}

	p.Name = mp.Name
	p.Line = mp.Line
	p.Visibility = mp.Visibility

	// Reconstruct the type from the type name
	if mp.TypeName != "" {
		p.Type = NewPHPType(mp.TypeName)
	}

	return nil
}

type PHPIndex struct {
	dataIndexer *indexer.DataIndexer[PHPClass]
}

func NewPHPIndex(configDir string) (*PHPIndex, error) {
	dataIndexer, err := indexer.NewDataIndexer[PHPClass](filepath.Join(configDir, "php.db"))
	if err != nil {
		return nil, fmt.Errorf("failed to create data indexer: %w", err)
	}

	idx := &PHPIndex{
		dataIndexer: dataIndexer,
	}

	return idx, nil
}

func (idx *PHPIndex) ID() string {
	return "php.index"
}

func (idx *PHPIndex) Index(path string, node *tree_sitter.Node, fileContent []byte) error {
	classes := GetClassesOfFileWithParser(path, node, fileContent)

	batchSave := make(map[string]map[string]PHPClass)

	for _, class := range classes {
		if _, ok := batchSave[class.Path]; !ok {
			batchSave[class.Path] = make(map[string]PHPClass)
		}
		batchSave[class.Path][class.Name] = class
	}

	return idx.dataIndexer.BatchSaveItems(batchSave)
}

func (idx *PHPIndex) GetClassesOfFile(path string) map[string]PHPClass {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())); err != nil {
		panic(err)
	}

	defer parser.Close()

	tree := parser.Parse(fileContent, nil)

	return GetClassesOfFileWithParser(path, tree.RootNode(), fileContent)
}

// GetTypeOfNode determines the PHP type of a given AST node.
// This is used for type inference in PHP code to provide accurate completions.
// The implementation is defined below as a method on PHPIndex.

// searchParentClassMethod recursively searches for a method in parent classes
// and returns the method's return type if found
func (idx *PHPIndex) searchParentClassMethod(parentClassName, methodName string) PHPType {
	if parentClassName == "" || methodName == "" {
		return nil
	}

	// Get the parent class individually - more efficient than getting all classes
	parentClass := idx.GetClass(parentClassName)
	if parentClass == nil {
		return nil
	}

	// Check if the method exists in the parent class
	method, ok := parentClass.Methods[methodName]
	if ok {
		return method.ReturnType
	}

	// If method not found in parent class, check the parent's parent
	if parentClass.Parent != "" {
		return idx.searchParentClassMethod(parentClass.Parent, methodName)
	}

	// Also check interfaces implemented by the parent class
	for _, interfaceName := range parentClass.Interfaces {
		interface_ := idx.GetClass(interfaceName)
		if interface_ == nil || !interface_.IsInterface {
			continue
		}

		method, ok := interface_.Methods[methodName]
		if ok {
			return method.ReturnType
		}
	}

	return nil
}

// searchParentClassProperty recursively searches for a property in parent classes
// and returns the property's type if found. It respects visibility rules, so private
// properties from parent classes are not accessible.
func (idx *PHPIndex) searchParentClassProperty(parentClassName, propertyName string) PHPType {
	if parentClassName == "" || propertyName == "" {
		return nil
	}

	// Get the parent class individually - more efficient than getting all classes
	parentClass := idx.GetClass(parentClassName)
	if parentClass == nil {
		return nil
	}

	// Check if the property exists in the parent class
	property, ok := parentClass.Properties[propertyName]
	if ok {
		// For the current class, we can access any property regardless of visibility
		// For parent classes, we can only access public and protected properties
		if property.Visibility != Private {
			return property.Type
		}
	}

	// If property not found or not accessible in parent class, check the parent's parent
	if parentClass.Parent != "" {
		return idx.searchParentClassProperty(parentClass.Parent, propertyName)
	}

	return nil
}

// GetClasses returns all classes indexed by name for legacy compatibility
func (idx *PHPIndex) GetClasses() map[string]PHPClass {
	allClasses := make(map[string]PHPClass)
	classValues, err := idx.dataIndexer.GetAllValues()
	if err != nil {
		log.Printf("Error fetching classes: %v", err)
		return allClasses
	}

	// Create a map of classes indexed by class name
	for _, class := range classValues {
		allClasses[class.Name] = class
	}

	return allClasses
}

// GetTypeOfNode determines the PHP type of a given AST node.
// This is used for type inference in PHP code to provide accurate completions.
// Currently supports:
// - $this->method() expressions
// - $this->property expressions
func (idx *PHPIndex) GetTypeOfNode(ctx context.Context, node *tree_sitter.Node, fileContent []byte) PHPType {
	if node == nil {
		return nil
	}

	// Get the PHP context safely
	phpCtx, ok := ctx.Value(PHPContextKey).(*PHPContext)
	if !ok || phpCtx == nil || phpCtx.InsideClass == nil {
		// If we don't have the necessary context, return a mixed type
		return NewMixedType()
	}

	nodeKind := node.Kind()

	// Handle member call expression: $this->method()
	if nodeKind == "member_call_expression" {
		return idx.handleMemberCallExpression(node, fileContent, phpCtx.InsideClass.Name)
	}

	// Default to mixed type if we can't determine a specific type
	return NewMixedType()
}

// handleMemberCallExpression processes $this->method() calls and returns the return type of that method
func (idx *PHPIndex) handleMemberCallExpression(node *tree_sitter.Node, fileContent []byte, currentClass string) PHPType {
	// Extract the object part of the expression (should be $this)
	memberAccessExpression := treesitterhelper.GetFirstNodeOfKind(node, "member_access_expression")

	if memberAccessExpression == nil {
		return NewPHPType("mixed")
	}

	variableName := treesitterhelper.GetFirstNodeOfKind(memberAccessExpression, "variable_name")

	if variableName == nil {
		return NewPHPType("mixed")
	}

	propertyName := string(treesitterhelper.GetFirstNodeOfKind(memberAccessExpression, "name").Utf8Text(fileContent))

	// Not a $this call
	if string(treesitterhelper.GetFirstNodeOfKind(variableName, "name").Utf8Text(fileContent)) != "this" {
		return NewPHPType("mixed")
	}

	property := idx.GetProperty(currentClass, propertyName)
	if property != nil {
		return property.Type
	}

	// Default to mixed if we couldn't determine the type
	return NewPHPType("mixed")
}

func (idx *PHPIndex) RemovedFiles(paths []string) error {
	return idx.dataIndexer.BatchDeleteByFilePaths(paths)
}

func (idx *PHPIndex) Close() error {
	return idx.dataIndexer.Close()
}

func (idx *PHPIndex) Clear() error {
	return idx.dataIndexer.Clear()
}

func (idx *PHPIndex) GetClass(className string) *PHPClass {
	values, err := idx.dataIndexer.GetValues(className)
	if err != nil {
		log.Printf("Error retrieving class: %v", err)
		return nil
	}

	if len(values) == 0 {
		return nil
	}

	return &values[0]
}

func (idx *PHPIndex) GetClassNames() []string {
	keys, err := idx.dataIndexer.GetAllKeys()
	if err != nil {
		log.Printf("Error retrieving class names: %v", err)
		return nil
	}

	return keys
}
