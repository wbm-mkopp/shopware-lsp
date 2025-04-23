package symfony

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
	"go.etcd.io/bbolt"
	"go.etcd.io/bbolt/errors"
)

// Bucket names for bbolt
var (
	servicesBucket   = []byte("services")
	aliasesBucket    = []byte("aliases")
	tagsBucket       = []byte("tags")
	parametersBucket = []byte("parameters")
	filesBucket      = []byte("files")       // Track indexed files
	fileHashBucket   = []byte("file_hashes") // Track file content hashes for change detection
)

// ServiceIndex maintains an index of all service IDs from XML files
type ServiceIndex struct {
	projectRoot      string
	db               *bbolt.DB
	mu               sync.RWMutex
	containerWatcher *ContainerWatcher
}

// NewServiceIndex creates a new service indexer for the given project root
func NewServiceIndex(projectRoot string, configDir string) (*ServiceIndex, error) {
	dbPath := filepath.Join(configDir, "symfony.db")

	// Open the database with different options for tests vs. production
	options := &bbolt.Options{
		Timeout:      1,
		NoSync:       true,
		FreelistType: bbolt.FreelistMapType,
	}

	db, err := bbolt.Open(dbPath, 0600, options)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists(servicesBucket); err != nil {
			return fmt.Errorf("failed to create services bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists(aliasesBucket); err != nil {
			return fmt.Errorf("failed to create aliases bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists(tagsBucket); err != nil {
			return fmt.Errorf("failed to create tags bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists(parametersBucket); err != nil {
			return fmt.Errorf("failed to create parameters bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists(filesBucket); err != nil {
			return fmt.Errorf("failed to create files bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists(fileHashBucket); err != nil {
			return fmt.Errorf("failed to create file hashes bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	idx := &ServiceIndex{
		projectRoot: projectRoot,
		db:          db,
	}

	// Initialize the container watcher after the index is created
	containerWatcher, err := NewContainerWatcher(projectRoot)
	if err != nil {
		log.Printf("Failed to initialize container watcher: %v", err)
		// Continue without the container watcher
	} else {
		idx.containerWatcher = containerWatcher
		log.Printf("Symfony container watcher initialized")
	}

	return idx, nil
}

func (idx *ServiceIndex) ID() string {
	return "symfony.service"
}

// Index scans the project for XML files and builds the service index
func (idx *ServiceIndex) Index(forceReindex bool) error {
	startTime := time.Now()

	// If forceReindex is true, delete the buckets and recreate them
	if forceReindex {
		log.Printf("Force reindexing requested, clearing existing service index")
		err := idx.db.Update(func(tx *bbolt.Tx) error {
			// Delete existing buckets
			buckets := [][]byte{servicesBucket, aliasesBucket, tagsBucket, parametersBucket, filesBucket, fileHashBucket}
			for _, bucket := range buckets {
				if err := tx.DeleteBucket(bucket); err != nil && err != errors.ErrBucketNotFound {
					return fmt.Errorf("failed to delete bucket: %w", err)
				}
			}
			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to clear index: %w", err)
		}
	}

	// Ensure all buckets exist (without holding the global lock yet)
	err := idx.db.Update(func(tx *bbolt.Tx) error {
		// Create buckets if they don't exist
		if _, err := tx.CreateBucketIfNotExists(servicesBucket); err != nil {
			return fmt.Errorf("failed to create services bucket: %w", err)
		}

		if _, err := tx.CreateBucketIfNotExists(aliasesBucket); err != nil {
			return fmt.Errorf("failed to create aliases bucket: %w", err)
		}

		if _, err := tx.CreateBucketIfNotExists(tagsBucket); err != nil {
			return fmt.Errorf("failed to create tags bucket: %w", err)
		}

		if _, err := tx.CreateBucketIfNotExists(parametersBucket); err != nil {
			return fmt.Errorf("failed to create parameters bucket: %w", err)
		}

		if _, err := tx.CreateBucketIfNotExists(filesBucket); err != nil {
			return fmt.Errorf("failed to create files bucket: %w", err)
		}

		if _, err := tx.CreateBucketIfNotExists(fileHashBucket); err != nil {
			return fmt.Errorf("failed to create file hash bucket: %w", err)
		}

		return nil
	})

	if err != nil {
		log.Printf("Error ensuring buckets exist: %v", err)
	}

	// Define directories to skip at project root level
	skipDirs := map[string]bool{
		"node_modules": true,
		"var":          true,
		"vendor-bin":   true,
		"bin":          true,
		"cache":        true,
		".git":         true,
		".github":      true,
		"tests":        true, // Skip test directories
	}

	// We can collect files without holding the lock
	var xmlFiles []string
	fileCollectionStart := time.Now()

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
					return filepath.SkipDir
				}
			}
			return nil
		}

		// Fast path: only process XML files with typical service file names to reduce overhead
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".xml" {
			return nil
		}

		// Quick pattern match for common Symfony service file names
		baseName := strings.ToLower(filepath.Base(path))
		if strings.Contains(baseName, "service") || strings.Contains(baseName, "container") ||
			strings.Contains(baseName, "config") || strings.Contains(baseName, "dependency") {
			xmlFiles = append(xmlFiles, path)
		} else {
			// Check if this looks like a Symfony service file by checking the first few bytes
			f, err := os.Open(path)
			if err != nil {
				return nil // Skip if we can't open
			}
			defer func() {
				_ = f.Close()
			}()

			// Read the first 1024 bytes to check for typical Symfony XML patterns
			buffer := make([]byte, 1024)
			n, err := f.Read(buffer)
			if err != nil || n == 0 {
				return nil
			}

			// Check for common patterns in Symfony XML files
			content := string(buffer[:n])
			if strings.Contains(content, "<container") ||
				strings.Contains(content, "<services") ||
				strings.Contains(content, "<service") {
				xmlFiles = append(xmlFiles, path)
			}
		}
		return nil
	})

	log.Printf("Found %d potential XML service files in %v", len(xmlFiles), time.Since(fileCollectionStart))

	if err != nil {
		return fmt.Errorf("failed to walk project directory: %w", err)
	}

	// Now we can safely lock for the processing phase
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Process files in parallel
	processingStart := time.Now()
	numWorkers := runtime.NumCPU()
	if numWorkers > 16 {
		numWorkers = 16 // Limit workers to prevent too much contention
	}

	var wg sync.WaitGroup
	filesChan := make(chan string, len(xmlFiles))

	// Start worker goroutines
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range filesChan {
				idx.processFile(path)
			}
		}()
	}

	// Send files to workers
	for _, path := range xmlFiles {
		filesChan <- path
	}
	close(filesChan)

	// Wait for all workers to finish
	wg.Wait()

	// After processing, clean up entries from files that no longer exist
	// Get the list of previously indexed files
	var existingFiles = make(map[string]bool)
	_ = idx.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(filesBucket)
		if b == nil {
			return nil // Bucket might not exist yet
		}
		return b.ForEach(func(k, v []byte) error {
			existingFiles[string(k)] = true
			return nil
		})
	})

	// Track which files we've processed in this run
	currentFiles := make(map[string]bool)
	for _, path := range xmlFiles {
		currentFiles[path] = true
	}

	// Remove entries from files that no longer exist
	for existingFile := range existingFiles {
		if !currentFiles[existingFile] {
			// This file no longer exists, remove all its entries
			idx.removeServicesFromFile(existingFile)

			// Remove from files tracking and hashes
			_ = idx.db.Update(func(tx *bbolt.Tx) error {
				// Remove from files bucket
				filesBucket := tx.Bucket(filesBucket)
				if filesBucket != nil {
					if err := filesBucket.Delete([]byte(existingFile)); err != nil {
						return err
					}
				}

				// Remove from file hash bucket
				hashBucket := tx.Bucket(fileHashBucket)
				if hashBucket != nil {
					if err := hashBucket.Delete([]byte(existingFile)); err != nil {
						return err
					}
				}

				return nil
			})
		}
	}

	// Count the number of services
	var serviceCount int
	_ = idx.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(servicesBucket)
		serviceCount = b.Stats().KeyN
		return nil
	})

	log.Printf("Processed %d XML files in %v", len(xmlFiles), time.Since(processingStart))
	log.Printf("Finished indexing %d services in %v", serviceCount, time.Since(startTime))

	return nil
}

// processFile parses an XML file and adds any service IDs to the index
func (idx *ServiceIndex) processFile(path string) {
	// Skip processing if the file is large (>1MB) as it's unlikely to be a service definition
	fileInfo, err := os.Stat(path)
	if err != nil {
		return
	}

	// Skip very large XML files (likely not service definitions)
	if fileInfo.Size() > 1024*1024 {
		return
	}

	// Check if file has changed by comparing hash
	content, err := os.ReadFile(path)
	if err != nil {
		return
	}

	// Calculate xxhash of file content
	hash := xxhash.Sum64(content)

	// Check if file has changed
	var fileChanged bool
	err = idx.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket(fileHashBucket)
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

	// If file hasn't changed, just make sure it's tracked but don't reprocess
	if !fileChanged {
		_ = idx.db.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket(filesBucket)
			if b != nil {
				return b.Put([]byte(path), []byte{1})
			}
			return nil
		})
		return
	}

	services, aliases, params, err := ParseXMLServices(content, path)
	if err != nil {
		// Don't log errors for non-service XML files to reduce noise
		return
	}

	// Skip if no services, aliases, or parameters were found
	if len(services) == 0 && len(aliases) == 0 && len(params) == 0 {
		return
	}

	// Pre-allocate and marshal all data outside the transaction for better performance
	type itemToStore struct {
		bucket []byte
		key    []byte
		data   []byte
	}

	var items []itemToStore
	// Prepare service entries
	for _, service := range services {
		data, err := json.Marshal(service)
		if err != nil {
			continue
		}
		items = append(items, itemToStore{
			bucket: servicesBucket,
			key:    []byte(service.ID),
			data:   data,
		})
	}

	// Prepare alias entries
	for _, alias := range aliases {
		data, err := json.Marshal(alias)
		if err != nil {
			continue
		}
		items = append(items, itemToStore{
			bucket: aliasesBucket,
			key:    []byte(alias.ID),
			data:   data,
		})
	}

	// Prepare parameter entries
	for _, param := range params {
		data, err := json.Marshal(param)
		if err != nil {
			continue
		}
		items = append(items, itemToStore{
			bucket: parametersBucket,
			key:    []byte(param.Name),
			data:   data,
		})
	}

	// Collect all tags that need updating
	tagMap := make(map[string][]string)
	for _, service := range services {
		for tagName := range service.Tags {
			tagMap[tagName] = append(tagMap[tagName], service.ID)
		}
	}

	// Execute the batch transaction with all prepared data
	err = idx.db.Batch(func(tx *bbolt.Tx) error {
		// Store all pre-marshaled items
		for _, item := range items {
			bucket := tx.Bucket(item.bucket)
			if err := bucket.Put(item.key, item.data); err != nil {
				return err
			}
		}

		// Process tags using a single transaction per tag
		if len(tagMap) > 0 {
			tagsBucket := tx.Bucket(tagsBucket)
			for tagName, serviceIDs := range tagMap {
				var existingIDs []string

				// Get existing tag data
				tagData := tagsBucket.Get([]byte(tagName))
				if tagData != nil {
					if err := json.Unmarshal(tagData, &existingIDs); err != nil {
						continue
					}
				}

				// Add new service IDs
				for _, id := range serviceIDs {
					found := false
					for _, existingID := range existingIDs {
						if id == existingID {
							found = true
							break
						}
					}
					if !found {
						existingIDs = append(existingIDs, id)
					}
				}

				// Store updated tag data
				updatedTagData, err := json.Marshal(existingIDs)
				if err != nil {
					continue
				}
				if err := tagsBucket.Put([]byte(tagName), updatedTagData); err != nil {
					return err
				}
			}
		}

		// Track this file in the files bucket and store its hash
		filesBucket := tx.Bucket(filesBucket)
		if filesBucket != nil {
			if err := filesBucket.Put([]byte(path), []byte{1}); err != nil {
				return err
			}
		}

		// Store the file hash for future change detection
		hashBucket := tx.Bucket(fileHashBucket)
		if hashBucket != nil {
			hashBytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(hashBytes, hash)
			if err := hashBucket.Put([]byte(path), hashBytes); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		// Only log errors for files that were expected to contain services
		log.Printf("Failed to process file %s: %v", path, err)
	}
}

// removeServicesFromFile removes all services, aliases, and parameters from a specific file
func (idx *ServiceIndex) removeServicesFromFile(path string) {
	var servicesToRemove []string
	var tagUpdates map[string][]string
	var aliasesToRemove []string
	var parametersToRemove []string

	// First find all items to remove
	err := idx.db.View(func(tx *bbolt.Tx) error {
		// Find services from this file
		servicesBucket := tx.Bucket(servicesBucket)
		_ = servicesBucket.ForEach(func(k, v []byte) error {
			var service Service
			if err := json.Unmarshal(v, &service); err != nil {
				return nil // Skip invalid entries
			}
			if service.Path == path {
				servicesToRemove = append(servicesToRemove, service.ID)

				// Track affected tags
				if tagUpdates == nil {
					tagUpdates = make(map[string][]string)
				}
				for tagName := range service.Tags {
					tagUpdates[tagName] = append(tagUpdates[tagName], service.ID)
				}
			}
			return nil
		})

		// Find aliases from this file
		aliasesBucket := tx.Bucket(aliasesBucket)
		_ = aliasesBucket.ForEach(func(k, v []byte) error {
			var alias ServiceAlias
			if err := json.Unmarshal(v, &alias); err != nil {
				return nil // Skip invalid entries
			}
			if alias.Path == path {
				aliasesToRemove = append(aliasesToRemove, alias.ID)
			}
			return nil
		})

		// Find parameters from this file
		parametersBucket := tx.Bucket(parametersBucket)
		return parametersBucket.ForEach(func(k, v []byte) error {
			var param Parameter
			if err := json.Unmarshal(v, &param); err != nil {
				return nil // Skip invalid entries
			}
			if param.Path == path {
				parametersToRemove = append(parametersToRemove, param.Name)
			}
			return nil
		})
	})

	if err != nil {
		log.Printf("Error finding items to remove from file %s: %v", path, err)
		return
	}

	// Then remove them
	err = idx.db.Batch(func(tx *bbolt.Tx) error {
		// Remove services
		servicesBucket := tx.Bucket(servicesBucket)
		for _, id := range servicesToRemove {
			if err := servicesBucket.Delete([]byte(id)); err != nil {
				return fmt.Errorf("failed to delete service %s: %w", id, err)
			}
		}

		// Update tags
		tagsBucket := tx.Bucket(tagsBucket)
		for tagName, serviceIDsToRemove := range tagUpdates {
			// Get current tag data
			tagData := tagsBucket.Get([]byte(tagName))
			if tagData == nil {
				continue
			}

			var serviceIDs []string
			if err := json.Unmarshal(tagData, &serviceIDs); err != nil {
				return fmt.Errorf("failed to unmarshal tag data: %w", err)
			}

			// Remove services for this file
			var filteredIDs []string
			for _, id := range serviceIDs {
				shouldRemove := false
				for _, idToRemove := range serviceIDsToRemove {
					if id == idToRemove {
						shouldRemove = true
						break
					}
				}
				if !shouldRemove {
					filteredIDs = append(filteredIDs, id)
				}
			}

			// If no services left with this tag, remove the tag
			if len(filteredIDs) == 0 {
				if err := tagsBucket.Delete([]byte(tagName)); err != nil {
					return fmt.Errorf("failed to delete tag %s: %w", tagName, err)
				}
			} else {
				// Update with remaining service IDs
				updatedData, err := json.Marshal(filteredIDs)
				if err != nil {
					return fmt.Errorf("failed to marshal updated tag data: %w", err)
				}
				if err := tagsBucket.Put([]byte(tagName), updatedData); err != nil {
					return fmt.Errorf("failed to update tag %s: %w", tagName, err)
				}
			}
		}

		// Remove aliases
		aliasesBucket := tx.Bucket(aliasesBucket)
		for _, id := range aliasesToRemove {
			if err := aliasesBucket.Delete([]byte(id)); err != nil {
				return fmt.Errorf("failed to delete alias %s: %w", id, err)
			}
		}

		// Remove parameters
		parametersBucket := tx.Bucket(parametersBucket)
		for _, name := range parametersToRemove {
			if err := parametersBucket.Delete([]byte(name)); err != nil {
				return fmt.Errorf("failed to delete parameter %s: %w", name, err)
			}
		}

		return nil
	})

	if err != nil {
		log.Printf("Error removing items from file %s: %v", path, err)
	}
}

// GetAllServices returns all indexed service IDs
func (idx *ServiceIndex) GetAllServices() []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Get services from database
	var dbServiceIDs []string

	_ = idx.db.View(func(tx *bbolt.Tx) error {
		// Get service IDs
		servicesBucket := tx.Bucket(servicesBucket)
		if servicesBucket != nil {
			_ = servicesBucket.ForEach(func(k, v []byte) error {
				dbServiceIDs = append(dbServiceIDs, string(k))
				return nil
			})
		}

		// Get alias IDs
		aliasesBucket := tx.Bucket(aliasesBucket)
		if aliasesBucket != nil {
			_ = aliasesBucket.ForEach(func(k, v []byte) error {
				dbServiceIDs = append(dbServiceIDs, string(k))
				return nil
			})
		}

		return nil
	})

	// If container watcher is available, add any services that aren't in the database
	if idx.containerWatcher != nil && idx.containerWatcher.ContainerExists() {
		cwServices := idx.containerWatcher.GetAllServices()

		// Create a map of existing database service IDs for quick lookup
		dbServiceMap := make(map[string]struct{}, len(dbServiceIDs))
		for _, id := range dbServiceIDs {
			dbServiceMap[id] = struct{}{}
		}

		// Add container watcher services that aren't in the database
		for _, id := range cwServices {
			if _, exists := dbServiceMap[id]; !exists {
				dbServiceIDs = append(dbServiceIDs, id)
			}
		}
	}

	return dbServiceIDs
}

// GetServiceByID returns a specific service by its ID
func (idx *ServiceIndex) GetServiceByID(id string) (Service, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// First check the database
	var service Service
	var found bool

	// Use a read-only transaction
	_ = idx.db.View(func(tx *bbolt.Tx) error {
		// Check for direct service
		servicesBucket := tx.Bucket(servicesBucket)
		if servicesBucket == nil {
			return errors.ErrBucketNotFound
		}

		serviceData := servicesBucket.Get([]byte(id))
		if serviceData != nil {
			if err := json.Unmarshal(serviceData, &service); err == nil {
				found = true
				return nil
			}
		}

		// Check if it's an alias and resolve it
		aliasesBucket := tx.Bucket(aliasesBucket)
		if aliasesBucket == nil {
			return nil
		}

		aliasData := aliasesBucket.Get([]byte(id))
		if aliasData != nil {
			var alias ServiceAlias
			if err := json.Unmarshal(aliasData, &alias); err == nil {
				// Try to find the target service
				targetData := servicesBucket.Get([]byte(alias.Target))
				if targetData != nil {
					if err := json.Unmarshal(targetData, &service); err == nil {
						found = true
						return nil
					}
				}
			}
		}

		return nil
	})

	// If not found in database, fallback to container watcher
	if !found && idx.containerWatcher != nil && idx.containerWatcher.ContainerExists() {
		service, found = idx.containerWatcher.GetServiceByID(id)
	}

	return service, found
}

// Close shuts down the database and cleans up temporary files
func (idx *ServiceIndex) Close() error {
	var err error

	// Close the container watcher if it exists
	if idx.containerWatcher != nil {
		if watcherErr := idx.containerWatcher.Close(); watcherErr != nil {
			log.Printf("Error closing container watcher: %v", watcherErr)
			err = watcherErr
		}
		idx.containerWatcher = nil
	}

	// Close the database
	if idx.db != nil {
		dbErr := idx.db.Close()
		if dbErr != nil && err == nil {
			err = dbErr
		}
		idx.db = nil
	}

	return err
}

func (idx *ServiceIndex) FileCreated(ctx context.Context, params *protocol.CreateFilesParams) error {
	for _, file := range params.Files {
		if !strings.HasSuffix(strings.ToLower(file.URI), ".xml") {
			continue
		}

		idx.removeServicesFromFile(strings.TrimPrefix(file.URI, "file://"))
		idx.processFile(strings.TrimPrefix(file.URI, "file://"))
	}

	return nil
}

func (idx *ServiceIndex) FileRenamed(ctx context.Context, params *protocol.RenameFilesParams) error {
	for _, file := range params.Files {
		if !strings.HasSuffix(strings.ToLower(file.NewURI), ".xml") {
			continue
		}

		// Remove the old file from the index
		idx.removeServicesFromFile(strings.TrimPrefix(file.OldURI, "file://"))

		// Process the new file
		idx.processFile(strings.TrimPrefix(file.NewURI, "file://"))
	}

	return nil
}

func (idx *ServiceIndex) FileDeleted(ctx context.Context, params *protocol.DeleteFilesParams) error {
	for _, file := range params.Files {
		if !strings.HasSuffix(strings.ToLower(file.URI), ".xml") {
			continue
		}

		// Remove the file from the index
		idx.removeServicesFromFile(strings.TrimPrefix(file.URI, "file://"))
	}

	return nil
}

// GetCounts returns the number of services and aliases in the index
func (idx *ServiceIndex) GetCounts() (int, int) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var serviceCount, aliasCount int

	_ = idx.db.View(func(tx *bbolt.Tx) error {
		serviceCount = tx.Bucket(servicesBucket).Stats().KeyN
		aliasCount = tx.Bucket(aliasesBucket).Stats().KeyN
		return nil
	})

	return serviceCount, aliasCount
}

// GetAllTags returns all tag names in the index
func (idx *ServiceIndex) GetAllTags() []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var tags []string

	_ = idx.db.View(func(tx *bbolt.Tx) error {
		tagsBucket := tx.Bucket(tagsBucket)
		return tagsBucket.ForEach(func(k, v []byte) error {
			tags = append(tags, string(k))
			return nil
		})
	})

	return tags
}

// GetServicesByTag returns all service IDs that have the specified tag
func (idx *ServiceIndex) GetServicesByTag(tagName string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var serviceIDs []string

	_ = idx.db.View(func(tx *bbolt.Tx) error {
		tagsBucket := tx.Bucket(tagsBucket)
		tagData := tagsBucket.Get([]byte(tagName))
		if tagData != nil {
			if err := json.Unmarshal(tagData, &serviceIDs); err != nil {
				return err
			}
		}
		return nil
	})

	return serviceIDs
}

// GetTagCount returns the number of unique tags in the index
func (idx *ServiceIndex) GetTagCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var tagCount int

	_ = idx.db.View(func(tx *bbolt.Tx) error {
		tagCount = tx.Bucket(tagsBucket).Stats().KeyN
		return nil
	})

	return tagCount
}

// GetAliasByID returns a specific alias by its ID
func (idx *ServiceIndex) GetAliasByID(id string) (ServiceAlias, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// First check the database
	var alias ServiceAlias
	var found bool

	_ = idx.db.View(func(tx *bbolt.Tx) error {
		aliasesBucket := tx.Bucket(aliasesBucket)
		if aliasesBucket == nil {
			return nil
		}
		aliasData := aliasesBucket.Get([]byte(id))
		if aliasData != nil {
			if err := json.Unmarshal(aliasData, &alias); err == nil {
				found = true
			}
		}
		return nil
	})

	// If not found in database, fallback to container watcher
	if !found && idx.containerWatcher != nil && idx.containerWatcher.ContainerExists() {
		alias, found = idx.containerWatcher.GetAliasByID(id)
	}

	return alias, found
}

// GetAllParameters returns all parameter names in the index
func (idx *ServiceIndex) GetAllParameters() []Parameter {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var parameters []Parameter

	_ = idx.db.View(func(tx *bbolt.Tx) error {
		parametersBucket := tx.Bucket(parametersBucket)
		return parametersBucket.ForEach(func(k, v []byte) error {
			var param Parameter
			if err := json.Unmarshal(v, &param); err == nil {
				parameters = append(parameters, param)
			}
			return nil
		})
	})

	return parameters
}

// GetParameterByName returns a specific parameter value by its name
func (idx *ServiceIndex) GetParameterByName(name string) (Parameter, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// First check the database
	var parameter Parameter
	var found bool

	_ = idx.db.View(func(tx *bbolt.Tx) error {
		parametersBucket := tx.Bucket(parametersBucket)
		if parametersBucket == nil {
			return nil
		}
		paramData := parametersBucket.Get([]byte(name))
		if paramData != nil {
			if err := json.Unmarshal(paramData, &parameter); err == nil {
				found = true
			}
		}
		return nil
	})

	// If not found in database, fallback to container watcher
	if !found && idx.containerWatcher != nil && idx.containerWatcher.ContainerExists() {
		parameter, found = idx.containerWatcher.GetParameterByName(name)
	}

	return parameter, found
}

// GetParameterCount returns the number of parameters in the index
func (idx *ServiceIndex) GetParameterCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var parameterCount int

	_ = idx.db.View(func(tx *bbolt.Tx) error {
		parameterCount = tx.Bucket(parametersBucket).Stats().KeyN
		return nil
	})

	return parameterCount
}

type Location struct {
	Path string
	Line int
}

func (idx *ServiceIndex) GetServicesUsageByClassName(className string) []Location {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	var locations []Location

	_ = idx.db.View(func(tx *bbolt.Tx) error {
		// Check services with matching class
		servicesBucket := tx.Bucket(servicesBucket)
		err := servicesBucket.ForEach(func(k, v []byte) error {
			var service Service
			if err := json.Unmarshal(v, &service); err == nil {
				if service.Class == className {
					locations = append(locations, Location{
						Path: service.Path,
						Line: service.Line,
					})
				}
			}
			return nil
		})
		if err != nil {
			return err
		}

		// Check if an alias exists with this name
		aliasesBucket := tx.Bucket(aliasesBucket)
		aliasData := aliasesBucket.Get([]byte(className))
		if aliasData != nil {
			var alias ServiceAlias
			if err := json.Unmarshal(aliasData, &alias); err == nil {
				locations = append(locations, Location{
					Path: alias.Path,
					Line: alias.Line,
				})
			}
		}

		return nil
	})

	return locations
}
