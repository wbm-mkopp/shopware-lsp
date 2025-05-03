package php

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/shopware/shopware-lsp/internal/indexer"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

type PHPClass struct {
	Name       string                 `json:"name"`
	Path       string                 `json:"path"`
	Line       int                    `json:"line"`
	Methods    map[string]PHPMethod   `json:"methods"`
	Properties map[string]PHPProperty `json:"properties"`
}

type PHPMethod struct {
	Name       string     `json:"name"`
	Line       int        `json:"line"`
	Visibility Visibility `json:"visibility"`
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
	Name       string     `json:"name"`
	Line       int        `json:"line"`
	Visibility Visibility `json:"visibility"`
	Type       string     `json:"type"`
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
	classes := idx.GetClassesOfFileWithParser(path, node, fileContent)

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

	return idx.GetClassesOfFileWithParser(path, tree.RootNode(), fileContent)
}

func (idx *PHPIndex) GetClassesOfFileWithParser(path string, node *tree_sitter.Node, fileContent []byte) map[string]PHPClass {
	classes := make(map[string]PHPClass)

	cursor := node.Walk()

	currentNamespace := ""
	// Map to store use statements (imports)
	useStatements := make(map[string]string)

	defer cursor.Close()

	if cursor.GotoFirstChild() {
		for {
			node := cursor.Node()

			if node.Kind() == "namespace_definition" {
				nameNode := node.Child(1)

				if nameNode != nil {
					currentNamespace = string(nameNode.Utf8Text(fileContent))
				}
			}

			// Process use statements (imports)
			if node.Kind() == "namespace_use_declaration" {
				for i := uint(0); i < node.NamedChildCount(); i++ {
					useClause := node.NamedChild(i)
					if useClause != nil && useClause.Kind() == "namespace_use_clause" {
						// Get the qualified name
						qualifiedName := treesitterhelper.GetFirstNodeOfKind(useClause, "qualified_name")
						if qualifiedName != nil {
							// Get the full namespace path
							fullPath := string(qualifiedName.Utf8Text(fileContent))
							
							// Get the class name (last part of the path)
							classNameNode := qualifiedName.NamedChild(qualifiedName.NamedChildCount() - 1)
							if classNameNode != nil && classNameNode.Kind() == "name" {
								className := string(classNameNode.Utf8Text(fileContent))
								
								// Store the mapping from class name to full path
								useStatements[className] = fullPath
							}
						}
					}
				}
			}

			if node.Kind() == "class_declaration" {
				classNameNode := treesitterhelper.GetFirstNodeOfKind(node, "name")

				if classNameNode != nil {
					className := string(classNameNode.Utf8Text(fileContent))

					if currentNamespace != "" {
						className = currentNamespace + "\\" + className
					}

					// Create a new class with empty methods and properties maps
					phpClass := PHPClass{
						Name:       className,
						Path:       path,
						Line:       int(classNameNode.Range().StartPoint.Row) + 1,
						Methods:    make(map[string]PHPMethod),
						Properties: make(map[string]PHPProperty),
					}

					// Extract methods and properties from the class
					phpClass.Methods = idx.extractMethodsFromClass(node, fileContent)
					phpClass.Properties = idx.extractPropertiesFromClass(node, fileContent, currentNamespace, useStatements)

					classes[className] = phpClass
				}
			}

			if !cursor.GotoNextSibling() {
				break
			}
		}
	}

	return classes
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

// extractMethodsFromClass extracts all method definitions from a class declaration node
func (idx *PHPIndex) extractMethodsFromClass(node *tree_sitter.Node, fileContent []byte) map[string]PHPMethod {
	methods := make(map[string]PHPMethod)

	// Find the class body node
	classBodyNode := treesitterhelper.GetFirstNodeOfKind(node, "declaration_list")
	if classBodyNode == nil {
		return methods
	}

	// Iterate through all children of the class body
	for i := uint(0); i < classBodyNode.NamedChildCount(); i++ {
		child := classBodyNode.NamedChild(i)
		if child == nil {
			continue
		}

		// Check if the child is a method declaration
		if child.Kind() == "method_declaration" {
			// Get the method name
			methodNameNode := treesitterhelper.GetFirstNodeOfKind(child, "name")
			if methodNameNode != nil {
				methodName := string(methodNameNode.Utf8Text(fileContent))

				// Determine method visibility
				visibility := Public // Default visibility
				
				// Check for visibility modifiers in the method declaration
				for k := uint(0); k < child.NamedChildCount(); k++ {
					modifier := child.NamedChild(k)
					if modifier == nil {
						continue
					}
					
					modifierText := string(modifier.Utf8Text(fileContent))
					switch modifierText {
					case "private":
						visibility = Private
						break
					case "protected":
						visibility = Protected
						break
					case "public":
						visibility = Public
						break
					}
				}
				
				// Create a new method and add it to the methods map
				methods[methodName] = PHPMethod{
					Name:       methodName,
					Line:       int(methodNameNode.Range().StartPoint.Row) + 1,
					Visibility: visibility,
				}
			}
		}
	}

	return methods
}

// extractPropertiesFromClass extracts all property definitions from a class declaration node
func (idx *PHPIndex) extractPropertiesFromClass(node *tree_sitter.Node, fileContent []byte, currentNamespace string, useStatements map[string]string) map[string]PHPProperty {
	properties := make(map[string]PHPProperty)

	// Find the class body node
	classBodyNode := treesitterhelper.GetFirstNodeOfKind(node, "declaration_list")
	if classBodyNode == nil {
		return properties
	}

	// Iterate through all children of the class body
	for i := uint(0); i < classBodyNode.NamedChildCount(); i++ {
		child := classBodyNode.NamedChild(i)
		if child == nil {
			continue
		}

		// Check if the child is a property declaration
		if child.Kind() == "property_declaration" {
			// Property declarations can have multiple properties defined at once
			// We need to iterate through the declaration_list to find all property elements
			for j := uint(0); j < child.NamedChildCount(); j++ {
				propElement := child.NamedChild(j)
				if propElement == nil {
					continue
				}

				// Check if this is a property element
				if propElement.Kind() == "property_element" {
					// Get the property name (variable name without the $ prefix)
					varNode := treesitterhelper.GetFirstNodeOfKind(propElement, "variable_name")
					if varNode != nil {
						// Get the property name without the $ prefix
						propNameWithPrefix := string(varNode.Utf8Text(fileContent))
						// Remove the $ prefix
						propName := propNameWithPrefix[1:] // Skip the first character ($)

						// Determine property visibility
						visibility := Public // Default visibility
						
						// Check for visibility modifiers in the property declaration
						for k := uint(0); k < child.NamedChildCount(); k++ {
							modifier := child.NamedChild(k)
							if modifier == nil {
								continue
							}
							
							modifierText := string(modifier.Utf8Text(fileContent))
							switch modifierText {
							case "private":
								visibility = Private
								break
							case "protected":
								visibility = Protected
								break
							case "public":
								visibility = Public
								break
							}
						}
						
						// Extract property type if available
						propType := ""
						
						// Try to find named_type (class types) first
						namedTypeNode := treesitterhelper.GetFirstNodeOfKind(child, "named_type")
						if namedTypeNode != nil {
							// For named types, get the name node
							nameNode := treesitterhelper.GetFirstNodeOfKind(namedTypeNode, "name")
							if nameNode != nil {
								// Get the short class name
								shortClassName := string(nameNode.Utf8Text(fileContent))
								
								// Try to resolve the FQCN using the use statements
								if fqcn, ok := useStatements[shortClassName]; ok {
									propType = fqcn
								} else {
									// If not found in use statements, assume it's in the current namespace
									if currentNamespace != "" {
										propType = currentNamespace + "\\" + shortClassName
									} else {
										propType = shortClassName
									}
								}
							}
						} else {
							// Try primitive type (int, string, etc.)
							primitiveTypeNode := treesitterhelper.GetFirstNodeOfKind(child, "primitive_type")
							if primitiveTypeNode != nil {
								propType = string(primitiveTypeNode.Utf8Text(fileContent))
							}
						}

						// Create a new property and add it to the properties map
						properties[propName] = PHPProperty{
							Name:       propName,
							Line:       int(varNode.Range().StartPoint.Row) + 1,
							Visibility: visibility,
							Type:       propType,
						}
					}
				}
			}
		} else if child.Kind() == "method_declaration" {
			// Check if this is a constructor method
			methodNameNode := treesitterhelper.GetFirstNodeOfKind(child, "name")
			if methodNameNode != nil && string(methodNameNode.Utf8Text(fileContent)) == "__construct" {
				// Find the parameter list
				paramListNode := treesitterhelper.GetFirstNodeOfKind(child, "formal_parameters")
				if paramListNode != nil {
					// Iterate through all parameters
					for j := uint(0); j < paramListNode.NamedChildCount(); j++ {
						param := paramListNode.NamedChild(j)
						if param == nil || param.Kind() != "property_promotion_parameter" {
							continue
						}

						// Get the property name from the parameter
						varNode := treesitterhelper.GetFirstNodeOfKind(param, "variable_name")
						if varNode != nil {
							// Get the property name without the $ prefix
							propNameWithPrefix := string(varNode.Utf8Text(fileContent))
							// Remove the $ prefix
							propName := propNameWithPrefix[1:] // Skip the first character ($)

							// Determine property visibility from constructor parameter
							visibility := Public // Default visibility
							
							// Check for visibility modifiers in the parameter
							for k := uint(0); k < param.NamedChildCount(); k++ {
								modifier := param.NamedChild(k)
								if modifier == nil {
									continue
								}
								
								modifierText := string(modifier.Utf8Text(fileContent))
								switch modifierText {
								case "private":
									visibility = Private
									break
								case "protected":
									visibility = Protected
									break
								case "public":
									visibility = Public
									break
								}
							}
							
							// Extract property type if available
							propType := ""
							
							// Try to find named_type (class types) first
							namedTypeNode := treesitterhelper.GetFirstNodeOfKind(param, "named_type")
							if namedTypeNode != nil {
								// For named types, get the name node
								nameNode := treesitterhelper.GetFirstNodeOfKind(namedTypeNode, "name")
								if nameNode != nil {
									// Get the short class name
									shortClassName := string(nameNode.Utf8Text(fileContent))
									
									// Try to resolve the FQCN using the use statements
									if fqcn, ok := useStatements[shortClassName]; ok {
										propType = fqcn
									} else {
										// If not found in use statements, assume it's in the current namespace
										if currentNamespace != "" {
											propType = currentNamespace + "\\" + shortClassName
										} else {
											propType = shortClassName
										}
									}
								}
							} else {
								// Try primitive type (int, string, etc.)
								primitiveTypeNode := treesitterhelper.GetFirstNodeOfKind(param, "primitive_type")
								if primitiveTypeNode != nil {
									propType = string(primitiveTypeNode.Utf8Text(fileContent))
								}
							}

							// Create a new property and add it to the properties map
							properties[propName] = PHPProperty{
								Name:       propName,
								Line:       int(varNode.Range().StartPoint.Row) + 1,
								Visibility: visibility,
								Type:       propType,
							}
						}
					}
				}
			}
		}
	}

	return properties
}
