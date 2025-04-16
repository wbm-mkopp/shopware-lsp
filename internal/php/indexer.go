package php

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

type PHPClass struct {
	Name string
	Path string
	Line int
}

type PHPIndex struct {
	projectRoot string
	phpClasses  map[string]PHPClass
	mu          sync.RWMutex
	parser      *tree_sitter.Parser
}

func NewPHPIndex(projectRoot string) (*PHPIndex, error) {

	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())); err != nil {
		return nil, fmt.Errorf("failed to set language: %w", err)
	}

	idx := &PHPIndex{
		projectRoot: projectRoot,
		parser:      parser,
	}

	return idx, nil
}

func (idx *PHPIndex) ID() string {
	return "php.index"
}

func (idx *PHPIndex) Name() string {
	return "PHP Indexer"
}

func (idx *PHPIndex) Index() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Clear existing index
	idx.phpClasses = make(map[string]PHPClass)

	// Walk the project directory
	err := filepath.Walk(idx.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if info.IsDir() {
			// Skip node_modules and var folders at project root level
			relPath, err := filepath.Rel(idx.projectRoot, path)
			if err == nil {
				pathParts := strings.Split(relPath, string(os.PathSeparator))
				if len(pathParts) == 1 && (pathParts[0] == "node_modules" || pathParts[0] == "var") {
					log.Printf("Skipping directory: %s", path)
					return filepath.SkipDir
				}
			}
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(path), ".php") {
			return nil
		}

		log.Printf("Processing file: %s", path)

		// Try to parse as a Symfony services file
		idx.processFile(path)

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk project directory: %w", err)
	}

	log.Printf("Finished indexing %d classes", len(idx.phpClasses))

	return nil
}

func (idx *PHPIndex) processFile(path string) {
	for className, phpClass := range idx.GetClassesOfFile(path) {
		idx.phpClasses[className] = phpClass
	}
}

func (idx *PHPIndex) GetClassesOfFile(path string) map[string]PHPClass {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Failed to read file %s: %v", path, err)
		return nil
	}

	classes := make(map[string]PHPClass)

	tree := idx.parser.Parse(content, nil)

	rootNode := tree.RootNode()
	cursor := rootNode.Walk()

	currentNamespace := ""

	defer cursor.Close()

	if cursor.GotoFirstChild() {
		for {
			node := cursor.Node()

			if node.Kind() == "namespace_definition" {
				nameNode := node.Child(1)

				if nameNode != nil {
					currentNamespace = string(nameNode.Utf8Text(content))
				}
			}

			if node.Kind() == "class_declaration" {
				classNameNode := treesitterhelper.GetFirstNodeOfKind(node, "name")

				if classNameNode != nil {
					className := string(classNameNode.Utf8Text(content))

					if currentNamespace != "" {
						className = currentNamespace + "\\" + className
					}

					classes[className] = PHPClass{
						Name: className,
						Path: path,
						Line: int(classNameNode.Range().StartPoint.Row) + 1,
					}
				}
			}

			if !cursor.GotoNextSibling() {
				break
			}
		}
	}

	return classes
}

func (idx *PHPIndex) removeFile(path string) {
	for id, phpClass := range idx.phpClasses {
		if phpClass.Path == path {
			delete(idx.phpClasses, id)
		}
	}
}

func (idx *PHPIndex) Close() error {
	idx.parser.Close()
	return nil
}

func (idx *PHPIndex) FileCreated(ctx context.Context, params *protocol.CreateFilesParams) error {
	for _, file := range params.Files {
		if !strings.HasSuffix(strings.ToLower(file.URI), ".php") {
			continue
		}

		idx.removeFile(strings.TrimPrefix(file.URI, "file://"))
		idx.processFile(strings.TrimPrefix(file.URI, "file://"))
	}

	return nil
}

func (idx *PHPIndex) FileRenamed(ctx context.Context, params *protocol.RenameFilesParams) error {
	for _, file := range params.Files {
		if !strings.HasSuffix(strings.ToLower(file.NewURI), ".php") {
			continue
		}

		// Remove the old file from the index
		idx.removeFile(strings.TrimPrefix(file.OldURI, "file://"))

		// Process the new file
		idx.processFile(file.NewURI)
	}

	return nil
}

func (idx *PHPIndex) FileDeleted(ctx context.Context, params *protocol.DeleteFilesParams) error {
	for _, file := range params.Files {
		if !strings.HasSuffix(strings.ToLower(file.URI), ".php") {
			continue
		}

		// Remove the file from the index
		idx.removeFile(strings.TrimPrefix(file.URI, "file://"))
	}

	return nil
}

func (idx *PHPIndex) GetClass(className string) *PHPClass {
	class, found := idx.phpClasses[className]
	if !found {
		return nil
	}
	return &class
}

func (idx *PHPIndex) GetClassNames() []string {
	classNames := make([]string, 0, len(idx.phpClasses))
	for className := range idx.phpClasses {
		classNames = append(classNames, className)
	}
	return classNames
}
