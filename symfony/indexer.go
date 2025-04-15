package symfony

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ServiceIndex maintains an index of all service IDs from XML files
type ServiceIndex struct {
	services    map[string]Service       // map[serviceID]Service
	aliases     map[string]ServiceAlias  // map[aliasID]ServiceAlias
	tags        map[string][]string      // map[tagName][]serviceIDs
	projectRoot string
	watcher     *fsnotify.Watcher
	mu          sync.RWMutex
}

// NewServiceIndex creates a new service indexer for the given project root
func NewServiceIndex(projectRoot string) (*ServiceIndex, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	idx := &ServiceIndex{
		services:    make(map[string]Service),
		aliases:     make(map[string]ServiceAlias),
		tags:        make(map[string][]string),
		projectRoot: projectRoot,
		watcher:     watcher,
	}

	// Start the file watcher
	go idx.watchFiles()

	return idx, nil
}

// BuildIndex scans the project for XML files and builds the service index
func (idx *ServiceIndex) BuildIndex() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	// Clear existing index
	idx.services = make(map[string]Service)
	idx.aliases = make(map[string]ServiceAlias)
	idx.tags = make(map[string][]string)

	// Walk the project directory
	return filepath.Walk(idx.projectRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process XML files
		if !strings.HasSuffix(strings.ToLower(path), ".xml") {
			return nil
		}

		// Try to parse as a Symfony services file
		idx.processFile(path)

		// Add to watcher
		return idx.watcher.Add(path)
	})
}

// processFile parses an XML file and adds any service IDs to the index
func (idx *ServiceIndex) processFile(path string) {
	services, aliases, err := ParseXMLServices(path)
	if err != nil {
		return // Skip files that can't be parsed
	}

	// Add services to index
	if len(services) > 0 {
		for _, service := range services {
			idx.services[service.ID] = service
			
			// Index tags
			for tagName := range service.Tags {
				if _, exists := idx.tags[tagName]; !exists {
					idx.tags[tagName] = []string{}
				}
				idx.tags[tagName] = append(idx.tags[tagName], service.ID)
			}
		}
	}

	// Add aliases to index
	if len(aliases) > 0 {
		for _, alias := range aliases {
			idx.aliases[alias.ID] = alias
		}
	}

	// Skip debug logging
}

// watchFiles monitors file changes and updates the index accordingly
func (idx *ServiceIndex) watchFiles() {
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
				idx.removeServicesFromFile(event.Name)
				// Process the file again
				idx.processFile(event.Name)
			} else if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
				// File was removed or renamed, remove its services from the index
				idx.removeServicesFromFile(event.Name)
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

// removeServicesFromFile removes all services from a specific file
func (idx *ServiceIndex) removeServicesFromFile(path string) {
	// Note: This function should be called with the mutex already locked
	// by the caller to avoid deadlocks

	// Remove services from this file and update tag index
	for id, service := range idx.services {
		if service.Path == path {
			// Remove service from tag index
			for tagName := range service.Tags {
				if serviceIDs, exists := idx.tags[tagName]; exists {
					// Remove this service ID from the tag's service list
					for i, serviceID := range serviceIDs {
						if serviceID == id {
							// Remove by swapping with the last element and truncating
							serviceIDs[i] = serviceIDs[len(serviceIDs)-1]
							idx.tags[tagName] = serviceIDs[:len(serviceIDs)-1]
							break
						}
					}
					
					// If no services left with this tag, remove the tag
					if len(idx.tags[tagName]) == 0 {
						delete(idx.tags, tagName)
					}
				}
			}
			
			// Remove the service itself
			delete(idx.services, id)
		}
	}

	// Remove aliases from this file
	for id, alias := range idx.aliases {
		if alias.Path == path {
			delete(idx.aliases, id)
		}
	}
}

// GetAllServices returns all indexed service IDs
func (idx *ServiceIndex) GetAllServices() []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// Allocate slice with capacity for all services and aliases
	services := make([]string, 0, len(idx.services)+len(idx.aliases))
	
	// Add service IDs
	for serviceID := range idx.services {
		services = append(services, serviceID)
	}
	
	// Add alias IDs
	for aliasID := range idx.aliases {
		services = append(services, aliasID)
	}
	
	return services
}

// GetServiceByID returns a specific service by its ID
func (idx *ServiceIndex) GetServiceByID(id string) (Service, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	service, exists := idx.services[id]
	if exists {
		return service, true
	}
	
	// Check if it's an alias and resolve it
	alias, isAlias := idx.aliases[id]
	if isAlias {
		// Try to find the target service
		targetService, targetExists := idx.services[alias.Target]
		if targetExists {
			return targetService, true
		}
	}
	
	return Service{}, false
}

// Close shuts down the file watcher
func (idx *ServiceIndex) Close() error {
	return idx.watcher.Close()
}

// GetCounts returns the number of services and aliases in the index
func (idx *ServiceIndex) GetCounts() (int, int) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	
	return len(idx.services), len(idx.aliases)
}

// GetAllTags returns all tag names in the index
func (idx *ServiceIndex) GetAllTags() []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	tags := make([]string, 0, len(idx.tags))
	for tag := range idx.tags {
		tags = append(tags, tag)
	}
	return tags
}

// GetServicesByTag returns all service IDs that have the specified tag
func (idx *ServiceIndex) GetServicesByTag(tagName string) []string {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	if serviceIDs, exists := idx.tags[tagName]; exists {
		// Return a copy to avoid concurrent modification issues
		result := make([]string, len(serviceIDs))
		copy(result, serviceIDs)
		return result
	}
	return []string{}
}

// GetTagCount returns the number of unique tags in the index
func (idx *ServiceIndex) GetTagCount() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	return len(idx.tags)
}
