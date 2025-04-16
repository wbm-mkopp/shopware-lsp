package php

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
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
	watcher     *fsnotify.Watcher
	phpClasses  map[string]PHPClass
	mu          sync.RWMutex
	parser      *tree_sitter.Parser
}

func NewPHPIndex(projectRoot string) (*PHPIndex, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Printf("Failed to create watcher: %v", err)

		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())); err != nil {
		return nil, fmt.Errorf("failed to set language: %w", err)
	}

	idx := &PHPIndex{
		projectRoot: projectRoot,
		watcher:     watcher,
		parser:      parser,
	}

	// Start the file watcher
	go idx.watchFiles()

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
	return filepath.Walk(idx.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(strings.ToLower(path), ".php") {
			return nil
		}

		log.Printf("Processing file: %s", path)

		// Try to parse as a Symfony services file
		idx.processFile(path)

		// Add to watcher
		return idx.watcher.Add(path)
	})
}

func (idx *PHPIndex) processFile(path string) {
	content, err := os.ReadFile(path)
	if err != nil {
		log.Printf("Failed to read file %s: %v", path, err)
		return
	}

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
				classNameNode := node.Child(1)

				if classNameNode != nil {
					className := string(classNameNode.Utf8Text(content))

					if currentNamespace != "" {
						className = currentNamespace + "\\" + className
					}

					idx.phpClasses[className] = PHPClass{
						Name: className,
						Path: path,
						Line: int(node.Range().StartPoint.Row) + 1,
					}
				}
			}

			if !cursor.GotoNextSibling() {
				break
			}
		}
	}
}

func (idx *PHPIndex) removeFile(path string) {
	for id, phpClass := range idx.phpClasses {
		if phpClass.Path == path {
			delete(idx.phpClasses, id)
		}
	}
}

func (idx *PHPIndex) watchFiles() {
	debounceMap := make(map[string]time.Time)
	debounceInterval := 500 * time.Millisecond

	for {
		select {
		case event, ok := <-idx.watcher.Events:
			if !ok {
				return
			}

			// Skip non-XML files
			if !strings.HasSuffix(strings.ToLower(event.Name), ".xml") {
				continue
			}

			// Debounce file events (editors often trigger multiple events)
			now := time.Now()
			lastEvent, exists := debounceMap[event.Name]
			if exists && now.Sub(lastEvent) < debounceInterval {
				debounceMap[event.Name] = now
				continue
			}
			debounceMap[event.Name] = now

			// Skip debug logging

			idx.mu.Lock()
			// Handle file events
			if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
				// File was modified or created
				// Remove any existing services from this file
				idx.removeFile(event.Name)
				// Process the file again
				idx.processFile(event.Name)
			} else if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				// File was removed or renamed, remove its services from the index
				idx.removeFile(event.Name)
			}
			idx.mu.Unlock()

		case err, ok := <-idx.watcher.Errors:
			if !ok {
				return
			}
			// Log only critical errors
			if err != nil {
				log.Printf("Critical watcher error: %v", err)
			}
		}
	}
}

func (idx *PHPIndex) Close() error {
	idx.watcher.Close()
	idx.parser.Close()
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
