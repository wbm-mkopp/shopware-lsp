package indexer

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/vmihailenco/msgpack/v5"
	"go.etcd.io/bbolt"
)

// DataIndexer is a generic indexer that can store any type of data in a bbolt database
type DataIndexer[T any] struct {
	db          *bbolt.DB
	mu          sync.RWMutex
	dbPath      string
	dataBucket  []byte
	filesBucket []byte
}

// NewDataIndexer creates a new generic data indexer
func NewDataIndexer[T any](dbPath string) (*DataIndexer[T], error) {
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
		if _, err := tx.CreateBucketIfNotExists([]byte("data")); err != nil {
			return fmt.Errorf("failed to create data bucket: %w", err)
		}

		if _, err := tx.CreateBucketIfNotExists([]byte("files")); err != nil {
			return fmt.Errorf("failed to create files bucket: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	return &DataIndexer[T]{
		db:          db,
		dbPath:      dbPath,
		dataBucket:  []byte("data"),
		filesBucket: []byte("files"),
	}, nil
}

// SaveItem saves an item to the database with the given key and associates it with a file path
func (idx *DataIndexer[T]) SaveItem(filePath, key string, item T) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	return idx.db.Update(func(tx *bbolt.Tx) error {
		dataBucket := tx.Bucket(idx.dataBucket)
		if dataBucket == nil {
			return fmt.Errorf("data bucket not found")
		}
		filesBucket := tx.Bucket(idx.filesBucket)
		if filesBucket == nil {
			return fmt.Errorf("files bucket not found")
		}

		// Marshal the item
		data, err := msgpack.Marshal(item)
		if err != nil {
			return fmt.Errorf("failed to marshal item: %w", err)
		}

		// Generate a unique ID using a Bolt sequence
		seq, err := dataBucket.NextSequence()
		if err != nil {
			return fmt.Errorf("failed to generate sequence: %w", err)
		}
		id := fmt.Sprintf("%s:%d", key, seq)

		// Save the item in the data bucket
		if err := dataBucket.Put([]byte(id), data); err != nil {
			return fmt.Errorf("failed to save item: %w", err)
		}

		// Load existing file entries
		var entries []string
		if fileEntries := filesBucket.Get([]byte(filePath)); fileEntries != nil {
			if err := msgpack.Unmarshal(fileEntries, &entries); err != nil {
				return fmt.Errorf("failed to unmarshal file entries: %w", err)
			}
		}

		// Add the new ID to the file entries
		entries = append(entries, id)

		// Marshal and save the updated file entries
		out, err := msgpack.Marshal(entries)
		if err != nil {
			return fmt.Errorf("failed to marshal file entries: %w", err)
		}
		if err := filesBucket.Put([]byte(filePath), out); err != nil {
			return fmt.Errorf("failed to save file entries: %w", err)
		}

		return nil
	})
}

// BatchSaveItems saves multiple items in a single transaction
func (idx *DataIndexer[T]) BatchSaveItems(items map[string]map[string]T) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	return idx.db.Update(func(tx *bbolt.Tx) error {
		dataBucket := tx.Bucket(idx.dataBucket)
		if dataBucket == nil {
			return fmt.Errorf("data bucket not found")
		}
		filesBucket := tx.Bucket(idx.filesBucket)
		if filesBucket == nil {
			return fmt.Errorf("files bucket not found")
		}

		for filePath, keyItems := range items {
			// Load existing file entries
			var fileEntries []string
			if existing := filesBucket.Get([]byte(filePath)); existing != nil {
				if err := msgpack.Unmarshal(existing, &fileEntries); err != nil {
					return fmt.Errorf("failed to unmarshal file entries: %w", err)
				}
			}

			for key, item := range keyItems {
				// Marshal item and save with a unique Bolt sequence ID
				data, err := msgpack.Marshal(item)
				if err != nil {
					return fmt.Errorf("failed to marshal item: %w", err)
				}
				seq, err := dataBucket.NextSequence()
				if err != nil {
					return fmt.Errorf("failed to generate sequence: %w", err)
				}
				id := fmt.Sprintf("%s:%d", key, seq)
				if err := dataBucket.Put([]byte(id), data); err != nil {
					return fmt.Errorf("failed to save item: %w", err)
				}
				fileEntries = append(fileEntries, id)
			}

			// Marshal and persist file entries
			out, err := msgpack.Marshal(fileEntries)
			if err != nil {
				return fmt.Errorf("failed to marshal file entries: %w", err)
			}
			if err := filesBucket.Put([]byte(filePath), out); err != nil {
				return fmt.Errorf("failed to save file entries: %w", err)
			}
		}

		return nil
	})
}

// GetValues returns all items with the given key
func (idx *DataIndexer[T]) GetValues(key string) ([]T, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var items []T
	if err := idx.db.View(func(tx *bbolt.Tx) error {
		dataBucket := tx.Bucket(idx.dataBucket)
		if dataBucket == nil {
			return fmt.Errorf("data bucket not found")
		}

		// Use prefix scan to get all entries
		prefix := []byte(key + ":")
		c := dataBucket.Cursor()
		for k, v := c.Seek(prefix); k != nil && bytes.HasPrefix(k, prefix); k, v = c.Next() {
			var item T
			if err := msgpack.Unmarshal(v, &item); err != nil {
				return fmt.Errorf("failed to unmarshal item: %w", err)
			}
			items = append(items, item)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return items, nil
}

// GetAllValues returns all items stored in the data bucket
func (idx *DataIndexer[T]) GetAllValues() ([]T, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var items []T
	if err := idx.db.View(func(tx *bbolt.Tx) error {
		dataBucket := tx.Bucket(idx.dataBucket)
		if dataBucket == nil {
			// Bucket doesn't exist, return empty slice
			return nil
		}

		// Get bucket stats to pre-allocate slice capacity
		stats := dataBucket.Stats()
		items = make([]T, 0, stats.KeyN)

		// Iterate over all key-value pairs in the data bucket
		c := dataBucket.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var item T
			// Ensure v is not nil or empty before unmarshalling
			if len(v) == 0 {
				// Skip empty values or handle as needed
				continue
			}
			if err := msgpack.Unmarshal(v, &item); err != nil {
				// Potentially log the error or handle corrupted data
				// For now, returning the error stops the whole operation
				return fmt.Errorf("failed to unmarshal item with key %s: %w", string(k), err)
			}
			items = append(items, item)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	return items, nil
}

// GetAllKeys returns all unique keys in the database
func (idx *DataIndexer[T]) GetAllKeys() ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	keyMap := make(map[string]struct{})

	err := idx.db.View(func(tx *bbolt.Tx) error {
		dataBucket := tx.Bucket(idx.dataBucket)
		if dataBucket == nil {
			return fmt.Errorf("data bucket not found")
		}

		cursor := dataBucket.Cursor()
		for k, _ := cursor.First(); k != nil; k, _ = cursor.Next() {
			keyStr := string(k)
			sep := strings.IndexByte(keyStr, ':')
			if sep > 0 {
				keyMap[keyStr[:sep]] = struct{}{}
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert map to slice
	keys := make([]string, 0, len(keyMap))
	for key := range keyMap {
		keys = append(keys, key)
	}

	return keys, nil
}

// DeleteByFilePath deletes all items associated with the given file path
func (idx *DataIndexer[T]) DeleteByFilePath(filePath string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	return idx.db.Update(func(tx *bbolt.Tx) error {
		// Get the data bucket
		dataBucket := tx.Bucket(idx.dataBucket)
		if dataBucket == nil {
			return fmt.Errorf("data bucket not found")
		}

		// Get the files bucket
		filesBucket := tx.Bucket(idx.filesBucket)
		if filesBucket == nil {
			return fmt.Errorf("files bucket not found")
		}

		// Get the file entries
		fileEntries := filesBucket.Get([]byte(filePath))
		if fileEntries == nil {
			return nil
		}
		var entries []string
		if err := msgpack.Unmarshal(fileEntries, &entries); err != nil {
			return fmt.Errorf("failed to unmarshal file entries: %w", err)
		}

		// Delete each item
		for _, id := range entries {
			if err := dataBucket.Delete([]byte(id)); err != nil {
				return fmt.Errorf("failed to delete item: %w", err)
			}
		}

		// Delete the file entry
		if err := filesBucket.Delete([]byte(filePath)); err != nil {
			return fmt.Errorf("failed to delete file entry: %w", err)
		}

		return nil
	})
}

// BatchDeleteByFilePaths deletes all items associated with the given file paths in a single transaction
func (idx *DataIndexer[T]) BatchDeleteByFilePaths(filePaths []string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	return idx.db.Update(func(tx *bbolt.Tx) error {
		// Get the data bucket
		dataBucket := tx.Bucket(idx.dataBucket)
		if dataBucket == nil {
			return fmt.Errorf("data bucket not found")
		}

		// Get the files bucket
		filesBucket := tx.Bucket(idx.filesBucket)
		if filesBucket == nil {
			return fmt.Errorf("files bucket not found")
		}

		// Process each file path
		for _, filePath := range filePaths {
			// Get the file entries
			fileEntries := filesBucket.Get([]byte(filePath))
			if fileEntries == nil {
				continue
			}
			var entries []string
			if err := msgpack.Unmarshal(fileEntries, &entries); err != nil {
				return fmt.Errorf("failed to unmarshal file entries: %w", err)
			}

			// Delete each item
			for _, id := range entries {
				if err := dataBucket.Delete([]byte(id)); err != nil {
					return fmt.Errorf("failed to delete item: %w", err)
				}
			}

			// Delete the file entry
			if err := filesBucket.Delete([]byte(filePath)); err != nil {
				return fmt.Errorf("failed to delete file entry: %w", err)
			}
		}

		return nil
	})
}

// Close closes the database
func (idx *DataIndexer[T]) Close() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()
	return idx.db.Close()
}
