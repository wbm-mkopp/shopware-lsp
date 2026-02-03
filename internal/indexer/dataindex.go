package indexer

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/vmihailenco/msgpack/v5"
	_ "modernc.org/sqlite"
)

// DataIndexer is a generic indexer that can store any type of data in a SQLite database
type DataIndexer[T any] struct {
	db     *sql.DB
	mu     sync.RWMutex
	dbPath string
}

// NewDataIndexer creates a new generic data indexer
func NewDataIndexer[T any](dbPath string) (*DataIndexer[T], error) {
	// Ensure parent directory exists for the DB file
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("failed to create db directory: %w", err)
	}

	// Open the database with WAL mode for concurrent access
	// Using _txlock=immediate to acquire locks early and avoid SQLITE_BUSY
	db, err := sql.Open("sqlite", dbPath+"?_txlock=immediate")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Set connection pool settings
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Enable WAL mode and set pragmas for concurrent access and optimization
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA cache_size=10000",
		"PRAGMA foreign_keys=ON",
		"PRAGMA auto_vacuum=INCREMENTAL", // Automatically reclaim space incrementally
		"PRAGMA wal_autocheckpoint=1000", // Checkpoint every 1000 pages (~4MB)
	}
	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed to set pragma %s: %w", pragma, err)
		}
	}

	// Create the tables if they don't exist
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS data (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			key TEXT NOT NULL,
			value BLOB NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_data_key ON data(key);
		
		CREATE TABLE IF NOT EXISTS files (
			file_path TEXT NOT NULL,
			data_id INTEGER NOT NULL,
			PRIMARY KEY (file_path, data_id),
			FOREIGN KEY (data_id) REFERENCES data(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_files_path ON files(file_path);
		CREATE INDEX IF NOT EXISTS idx_files_data_id ON files(data_id);
	`)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return &DataIndexer[T]{
		db:     db,
		dbPath: dbPath,
	}, nil
}

// SaveItem saves an item to the database with the given key and associates it with a file path
func (idx *DataIndexer[T]) SaveItem(filePath, key string, item T) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Marshal the item
	data, err := msgpack.Marshal(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %w", err)
	}

	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Insert the data
	result, err := tx.Exec("INSERT INTO data (key, value) VALUES (?, ?)", key, data)
	if err != nil {
		return fmt.Errorf("failed to save item: %w", err)
	}

	dataID, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	// Associate with file path
	_, err = tx.Exec("INSERT INTO files (file_path, data_id) VALUES (?, ?)", filePath, dataID)
	if err != nil {
		return fmt.Errorf("failed to save file association: %w", err)
	}

	return tx.Commit()
}

// BatchSaveItems saves multiple items in a single transaction
func (idx *DataIndexer[T]) BatchSaveItems(items map[string]map[string]T) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	dataStmt, err := tx.Prepare("INSERT INTO data (key, value) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare data statement: %w", err)
	}
	defer func() { _ = dataStmt.Close() }()

	fileStmt, err := tx.Prepare("INSERT INTO files (file_path, data_id) VALUES (?, ?)")
	if err != nil {
		return fmt.Errorf("failed to prepare file statement: %w", err)
	}
	defer func() { _ = fileStmt.Close() }()

	for filePath, keyItems := range items {
		for key, item := range keyItems {
			// Marshal item
			data, err := msgpack.Marshal(item)
			if err != nil {
				return fmt.Errorf("failed to marshal item: %w", err)
			}

			// Insert data
			result, err := dataStmt.Exec(key, data)
			if err != nil {
				return fmt.Errorf("failed to save item: %w", err)
			}

			dataID, err := result.LastInsertId()
			if err != nil {
				return fmt.Errorf("failed to get last insert id: %w", err)
			}

			// Associate with file path
			_, err = fileStmt.Exec(filePath, dataID)
			if err != nil {
				return fmt.Errorf("failed to save file association: %w", err)
			}
		}
	}

	return tx.Commit()
}

// GetValues returns all items with the given key
func (idx *DataIndexer[T]) GetValues(key string) ([]T, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	rows, err := idx.db.Query("SELECT value FROM data WHERE key = ?", key)
	if err != nil {
		return nil, fmt.Errorf("failed to query data: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []T
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		var item T
		if err := msgpack.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("failed to unmarshal item: %w", err)
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

// GetAllValues returns all items stored in the data table
func (idx *DataIndexer[T]) GetAllValues() ([]T, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	rows, err := idx.db.Query("SELECT value FROM data")
	if err != nil {
		return nil, fmt.Errorf("failed to query data: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var items []T
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		var item T
		if len(data) == 0 {
			continue
		}
		if err := msgpack.Unmarshal(data, &item); err != nil {
			return nil, fmt.Errorf("failed to unmarshal item: %w", err)
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

// GetAllKeys returns all unique keys in the database
func (idx *DataIndexer[T]) GetAllKeys() ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	rows, err := idx.db.Query("SELECT DISTINCT key FROM data")
	if err != nil {
		return nil, fmt.Errorf("failed to query keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("failed to scan key: %w", err)
		}
		keys = append(keys, key)
	}

	return keys, rows.Err()
}

// DeleteByFilePath deletes all items associated with the given file path
func (idx *DataIndexer[T]) DeleteByFilePath(filePath string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Delete data entries associated with this file
	_, err = tx.Exec(`
		DELETE FROM data WHERE id IN (
			SELECT data_id FROM files WHERE file_path = ?
		)
	`, filePath)
	if err != nil {
		return fmt.Errorf("failed to delete data: %w", err)
	}

	// Delete file associations
	_, err = tx.Exec("DELETE FROM files WHERE file_path = ?", filePath)
	if err != nil {
		return fmt.Errorf("failed to delete file associations: %w", err)
	}

	return tx.Commit()
}

// GetAllKeysByPath returns all unique keys associated with a specific file path
func (idx *DataIndexer[T]) GetAllKeysByPath(filePath string) ([]string, error) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	rows, err := idx.db.Query(`
		SELECT DISTINCT d.key FROM data d
		INNER JOIN files f ON d.id = f.data_id
		WHERE f.file_path = ?
	`, filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to query keys: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("failed to scan key: %w", err)
		}
		keys = append(keys, key)
	}

	return keys, rows.Err()
}

// BatchDeleteByFilePaths deletes all items associated with the given file paths in a single transaction
func (idx *DataIndexer[T]) BatchDeleteByFilePaths(filePaths []string) error {
	if len(filePaths) == 0 {
		return nil
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	tx, err := idx.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, filePath := range filePaths {
		// Delete data entries associated with this file
		_, err = tx.Exec(`
			DELETE FROM data WHERE id IN (
				SELECT data_id FROM files WHERE file_path = ?
			)
		`, filePath)
		if err != nil {
			return fmt.Errorf("failed to delete data: %w", err)
		}

		// Delete file associations
		_, err = tx.Exec("DELETE FROM files WHERE file_path = ?", filePath)
		if err != nil {
			return fmt.Errorf("failed to delete file associations: %w", err)
		}
	}

	return tx.Commit()
}

func (idx *DataIndexer[T]) Clear() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	_, err := idx.db.Exec("DELETE FROM files; DELETE FROM data;")
	if err != nil {
		return err
	}

	// Reclaim space after clearing all data
	_, err = idx.db.Exec("PRAGMA incremental_vacuum")
	return err
}

// Close closes the database with optimization
func (idx *DataIndexer[T]) Close() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Optimize query planner statistics before closing
	_, _ = idx.db.Exec("PRAGMA optimize")

	// Reclaim any remaining unused space
	_, _ = idx.db.Exec("PRAGMA incremental_vacuum")

	// Checkpoint and truncate the WAL file to reduce disk usage
	_, _ = idx.db.Exec("PRAGMA wal_checkpoint(TRUNCATE)")

	return idx.db.Close()
}
