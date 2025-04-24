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
	Name    string               `json:"name"`
	Path    string               `json:"path"`
	Line    int                  `json:"line"`
	Methods map[string]PHPMethod `json:"methods"`
}

type PHPMethod struct {
	Name string `json:"name"`
	Line int    `json:"line"`
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

			if node.Kind() == "class_declaration" {
				classNameNode := treesitterhelper.GetFirstNodeOfKind(node, "name")

				if classNameNode != nil {
					className := string(classNameNode.Utf8Text(fileContent))

					if currentNamespace != "" {
						className = currentNamespace + "\\" + className
					}

					// Create a new class with empty methods map
					phpClass := PHPClass{
						Name:    className,
						Path:    path,
						Line:    int(classNameNode.Range().StartPoint.Row) + 1,
						Methods: make(map[string]PHPMethod),
					}

					// Extract methods from the class
					phpClass.Methods = idx.extractMethodsFromClass(node, fileContent)

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
				
				// Create a new method and add it to the methods map
				methods[methodName] = PHPMethod{
					Name: methodName,
					Line: int(methodNameNode.Range().StartPoint.Row) + 1,
				}
			}
		}
	}

	return methods
}
