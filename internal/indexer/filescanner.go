package indexer

import (
	"context"
	"encoding/binary"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/cespare/xxhash/v2"
	"go.etcd.io/bbolt"
)

var defaultSkipDirs = map[string]bool{
	"node_modules": true,
	"var":          true,
	"vendor-bin":   true,
	"bin":          true,
	"cache":        true,
	".git":         true,
	".github":      true,
	"tests":        true,
	"public":       true,
}

// FileScanner scans the project for files and tracks changes
type FileScanner struct {
	projectRoot string
	db          *bbolt.DB
	indexer     []Indexer
}

// NewFileScanner creates a new file scanner
func NewFileScanner(projectRoot string, dbPath string) (*FileScanner, error) {
	// Ensure parent directory exists for the DB file
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	// Open the database
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout:         time.Second,
		NoSync:          true,
		FreelistType:    bbolt.FreelistMapType,
		InitialMmapSize: 1024 * 1024 * 10, // 10MB initial mmap size
		PageSize:        4096,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create the buckets if they don't exist
	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte("file_hashes")); err != nil {
			return fmt.Errorf("failed to create file hashes bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	return &FileScanner{
		projectRoot: projectRoot,
		db:          db,
		indexer:     []Indexer{},
	}, nil
}

func (fs *FileScanner) AddIndexer(indexer Indexer) {
	fs.indexer = append(fs.indexer, indexer)
}

// Close closes the database
func (fs *FileScanner) Close() error {
	if fs.db != nil {
		return fs.db.Close()
	}

	for _, indexer := range fs.indexer {
		if err := indexer.Close(); err != nil {
			return err
		}
	}

	return nil
}

func (fs *FileScanner) IndexAll(ctx context.Context) error {
	startTime := time.Now()
	var files []string

	err := filepath.Walk(fs.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if info.IsDir() {
			// Skip directories in the skipDirs list
			relPath, err := filepath.Rel(fs.projectRoot, path)
			if err == nil {
				pathParts := strings.Split(relPath, string(os.PathSeparator))
				if len(pathParts) == 1 && defaultSkipDirs[pathParts[0]] {
					return filepath.SkipDir
				}
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if slices.Contains(scannedFileTypes, ext) {
			files = append(files, path)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to walk project directory: %w", err)
	}

	if err := fs.IndexFiles(ctx, files); err != nil {
		return fmt.Errorf("failed to index files: %w", err)
	}

	log.Printf("Indexing took %s", time.Since(startTime))

	return nil
}

// fileNeedsIndexing checks if a file needs to be indexed
func (fs *FileScanner) fileNeedsIndexing(path string) (bool, []byte, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, nil, err
	}

	// Calculate xxhash of file content
	hash := xxhash.Sum64(content)

	// Check if file has changed
	var fileChanged bool
	err = fs.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("file_hashes"))
		if b == nil {
			fileChanged = true
			return nil
		}

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

	return fileChanged, content, nil
}

// RemoveFile removes a file from the index
func (fs *FileScanner) RemoveFiles(ctx context.Context, paths []string) error {
	for _, indexer := range fs.indexer {
		if err := indexer.RemovedFiles(paths); err != nil {
			return err
		}
	}

	return fs.db.Update(func(tx *bbolt.Tx) error {
		hashBucket := tx.Bucket([]byte("file_hashes"))
		for _, path := range paths {
			if err := hashBucket.Delete([]byte(path)); err != nil {
				return err
			}
		}
		return nil
	})
}

// updateFileHash updates the stored hash for a file
func (fs *FileScanner) updateFileHash(path string, content []byte) error {
	hash := xxhash.Sum64(content)

	return fs.db.Update(func(tx *bbolt.Tx) error {
		hashBucket := tx.Bucket([]byte("file_hashes"))
		hashBytes := make([]byte, 8)
		binary.LittleEndian.PutUint64(hashBytes, hash)
		return hashBucket.Put([]byte(path), hashBytes)
	})
}

// IndexFiles processes multiple files in parallel
func (fs *FileScanner) IndexFiles(ctx context.Context, files []string) error {
	if len(files) == 0 {
		return nil
	}

	// Remove current entries from the database
	if err := fs.RemoveFiles(ctx, files); err != nil {
		return err
	}

	// Determine the number of worker goroutines to use
	workerCount := runtime.NumCPU() + 2
	if workerCount > 16 {
		workerCount = 16
	}

	// Create a channel to distribute work
	fileChan := make(chan string, 100)

	// Create a channel for errors
	errChan := make(chan error, len(files))

	// Create a wait group to wait for all workers to finish
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			parsers := CreateTreesitterParsers()

			for path := range fileChan {
				// Check if file needs indexing
				needsIndexing, content, err := fs.fileNeedsIndexing(path)
				if err != nil {
					// We'll just skip file errors to reduce noise
					continue
				}

				// If file hasn't changed, skip it
				if !needsIndexing {
					continue
				}

				ext := strings.ToLower(filepath.Ext(path))

				parser := parsers[ext]
				if parser == nil {
					panic(fmt.Sprintf("no parser found for file type: %s", ext))
				}

				tree := parser.Parse(content, nil)

				for _, indexer := range fs.indexer {
					if err := indexer.Index(path, tree.RootNode(), content); err != nil {
						errChan <- err
					}
				}

				tree.Close()

				// Update the file hash
				if err := fs.updateFileHash(path, content); err != nil {
					errChan <- err
				}
			}

			CloseTreesitterParsers(parsers)
		}()
	}

	// Send files to workers
	for _, path := range files {
		fileChan <- path
	}
	close(fileChan)

	// Wait for all workers to finish
	wg.Wait()
	close(errChan)

	// Check if there were any errors
	for err := range errChan {
		log.Printf("Error processing file: %v", err)
	}

	return nil
}

// ClearHashes clears all file hashes, forcing reindexing
func (fs *FileScanner) ClearHashes() error {
	return fs.db.Update(func(tx *bbolt.Tx) error {
		// Delete and recreate bucket
		if err := tx.DeleteBucket([]byte("file_hashes")); err != nil {
			return fmt.Errorf("failed to delete file hashes bucket: %w", err)
		}
		if _, err := tx.CreateBucket([]byte("file_hashes")); err != nil {
			return fmt.Errorf("failed to create file hashes bucket: %w", err)
		}
		return nil
	})
}
