package php

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
	"go.etcd.io/bbolt"
)

type PHPClass struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Line int    `json:"line"`
}

// FileHash stores the hash of a processed file
type FileHash struct {
	Path string `json:"path"`
	Hash uint64 `json:"hash"`
}

type PHPIndex struct {
	projectRoot string
	db          *bbolt.DB
	mu          sync.RWMutex
	parser      *tree_sitter.Parser
	dbPath      string
}

// Bucket names for bbolt
var (
	classesBucket  = []byte("classes")
	fileHashBucket = []byte("file_hashes")
)

func NewPHPIndex(projectRoot string, configDir string) (*PHPIndex, error) {
	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())); err != nil {
		return nil, fmt.Errorf("failed to set language: %w", err)
	}

	dbPath := filepath.Join(configDir, "php.db")

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout:      1,
		NoSync:       true,
		FreelistType: bbolt.FreelistMapType,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(classesBucket); err != nil {
			return fmt.Errorf("failed to create classes bucket: %w", err)
		}

		if _, err := tx.CreateBucketIfNotExists(fileHashBucket); err != nil {
			return fmt.Errorf("failed to create file hash bucket: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	idx := &PHPIndex{
		projectRoot: projectRoot,
		parser:      parser,
		db:          db,
		dbPath:      dbPath,
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
	// Start timing
	startTime := time.Now()
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Clear existing index
	err := idx.db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(classesBucket); err != nil {
			return fmt.Errorf("failed to create classes bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to clear index: %w", err)
	}

	var phpFiles []string

	skipDirs := map[string]bool{
		"node_modules": true,
		"var":          true,
		"vendor-bin":   true,
		"bin":          true,
		"cache":        true,
		".git":         true,
		".github":      true,
	}

	err = filepath.Walk(idx.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if info.IsDir() {
			// Skip common directories at project root level
			relPath, err := filepath.Rel(idx.projectRoot, path)
			if err == nil {
				pathParts := strings.Split(relPath, string(os.PathSeparator))
				if len(pathParts) == 1 && skipDirs[pathParts[0]] {
					// Skip without logging to reduce output noise
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Fast path for PHP files
		if filepath.Ext(path) != ".php" {
			return nil
		}

		phpFiles = append(phpFiles, path)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk project directory: %w", err)
	}

	var wg sync.WaitGroup
	fileChan := make(chan string, 1000)

	workerCount := runtime.NumCPU() + 2
	if workerCount > 16 {
		workerCount = 16
	}

	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			// Create a new parser for each goroutine to avoid concurrent access
			workerParser := tree_sitter.NewParser()
			if err := workerParser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())); err != nil {
				log.Printf("Failed to set language for worker parser: %v", err)
				return
			}
			defer workerParser.Close()

			defer wg.Done()
			for path := range fileChan {
				idx.processFileWithHash(path, workerParser)
			}
		}()
	}

	// Send files to workers
	for _, path := range phpFiles {
		fileChan <- path
	}
	close(fileChan)

	// Wait for all workers to finish
	wg.Wait()

	// Count indexed classes
	var classCount int
	_ = idx.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(classesBucket)
		classCount = b.Stats().KeyN
		return nil
	})

	log.Printf("Finished indexing %d classes in %v", classCount, time.Since(startTime))

	return nil
}

func (idx *PHPIndex) processFile(path string) {
	classes := idx.GetClassesOfFile(path)
	if len(classes) == 0 {
		return
	}

	err := idx.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(classesBucket)
		for className, phpClass := range classes {
			data, err := json.Marshal(phpClass)
			if err != nil {
				return fmt.Errorf("failed to marshal class data: %w", err)
			}
			if err := b.Put([]byte(className), data); err != nil {
				return fmt.Errorf("failed to store class data: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("Error storing classes for file %s: %v", path, err)
	}
}

func (idx *PHPIndex) processFileWithHash(path string, parser *tree_sitter.Parser) {
	// Check if file has changed by comparing hash
	content, err := os.ReadFile(path)
	if err != nil {
		// Don't log every file error to reduce noise
		return
	}

	// Calculate xxhash of file content
	hash := xxhash.Sum64(content)

	// Check if file has changed
	var fileChanged bool
	err = idx.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(fileHashBucket)
		hashBytes := b.Get([]byte(path))
		if hashBytes == nil {
			fileChanged = true
			return nil
		}

		storedHash := binary.LittleEndian.Uint64(hashBytes)
		fileChanged = storedHash != hash
		return nil
	})
	if err != nil {
		fileChanged = true
	}

	if !fileChanged {
		return
	}

	classes := idx.GetClassesOfFileWithParser(path, parser, content)
	if len(classes) > 0 {
		err := idx.db.Batch(func(tx *bbolt.Tx) error {
			b := tx.Bucket(classesBucket)
			for className, phpClass := range classes {
				data, err := json.Marshal(phpClass)
				if err != nil {
					return fmt.Errorf("failed to marshal class data: %w", err)
				}
				if err := b.Put([]byte(className), data); err != nil {
					return fmt.Errorf("failed to store class data: %w", err)
				}
			}

			// Update the file hash in the same transaction
			hashBucket := tx.Bucket(fileHashBucket)
			hashBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(hashBytes, hash)
			return hashBucket.Put([]byte(path), hashBytes)
		})
		if err != nil {
			log.Printf("Error storing data for file %s: %v", path, err)
		}
	} else {
		err = idx.db.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket(fileHashBucket)
			hashBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(hashBytes, hash)
			return b.Put([]byte(path), hashBytes)
		})
		if err != nil {
			log.Printf("Error updating file hash for %s: %v", path, err)
		}
	}
}

func (idx *PHPIndex) GetClassesOfFile(path string) map[string]PHPClass {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	return idx.GetClassesOfFileWithParser(path, idx.parser, content)
}

func (idx *PHPIndex) GetClassesOfFileWithParser(path string, parser *tree_sitter.Parser, content []byte) map[string]PHPClass {
	classes := make(map[string]PHPClass)

	tree := parser.Parse(content, nil)

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
	err := idx.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket(fileHashBucket)
		return b.Delete([]byte(path))
	})
	if err != nil {
		log.Printf("Error removing file hash for %s: %v", path, err)
	}

	// Remove classes associated with the file
	var classesToRemove []string
	err = idx.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(classesBucket)
		return b.ForEach(func(k, v []byte) error {
			var phpClass PHPClass
			if err := json.Unmarshal(v, &phpClass); err != nil {
				return nil // Skip invalid entries
			}
			if phpClass.Path == path {
				classesToRemove = append(classesToRemove, string(k))
			}
			return nil
		})
	})
	if err != nil {
		log.Printf("Error finding classes to remove for %s: %v", path, err)
		return
	}

	// Remove the classes
	if len(classesToRemove) > 0 {
		err = idx.db.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket(classesBucket)
			for _, className := range classesToRemove {
				if err := b.Delete([]byte(className)); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			log.Printf("Error removing classes for %s: %v", path, err)
		}
	}
}

func (idx *PHPIndex) Close() error {
	idx.parser.Close()
	if idx.db != nil {
		return idx.db.Close()
	}
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
	var phpClass *PHPClass

	err := idx.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(classesBucket)
		data := b.Get([]byte(className))
		if data == nil {
			return nil
		}

		var class PHPClass
		if err := json.Unmarshal(data, &class); err != nil {
			return err
		}
		phpClass = &class
		return nil
	})

	if err != nil {
		log.Printf("Error retrieving class %s: %v", className, err)
		return nil
	}

	return phpClass
}

func (idx *PHPIndex) GetClassNames() []string {
	var classNames []string

	err := idx.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(classesBucket)
		return b.ForEach(func(k, v []byte) error {
			classNames = append(classNames, string(k))
			return nil
		})
	})

	if err != nil {
		log.Printf("Error retrieving class names: %v", err)
	}

	return classNames
}
