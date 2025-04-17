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

// ContainerWatcher watches the Symfony container XML file and keeps services in memory
type ContainerWatcher struct {
	projectRoot     string
	containerPath   string
	watcher         *fsnotify.Watcher
	services        map[string]Service
	aliases         map[string]ServiceAlias
	parameters      map[string]Parameter
	mu              sync.RWMutex
	lastUpdated     time.Time
	containerExists bool
}

// NewContainerWatcher creates a new watcher for the Symfony container XML file
func NewContainerWatcher(projectRoot string) (*ContainerWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	cw := &ContainerWatcher{
		projectRoot:  projectRoot,
		watcher:      watcher,
		services:     make(map[string]Service),
		aliases:      make(map[string]ServiceAlias),
		parameters:   make(map[string]Parameter),
	}

	// Find and load the container file initially
	if err := cw.findAndLoadContainer(); err != nil {
		log.Printf("Initial container load failed: %v", err)
	}

	// Start watching for changes
	go cw.watchChanges()

	return cw, nil
}

// findAndLoadContainer locates and loads the Symfony container XML file
func (cw *ContainerWatcher) findAndLoadContainer() error {
	// Look for the container file in the var/cache directory
	containerPath, err := cw.findContainerFile()
	if err != nil {
		cw.containerExists = false
		
		// Even if we can't find the container file, watch the var/cache directory
		// for when it might be created later
		cacheDir := filepath.Join(cw.projectRoot, "var", "cache")
		
		// Check if the cache directory exists
		if _, err := os.Stat(cacheDir); err == nil {
			// Watch the cache directory
			if err := cw.watcher.Add(cacheDir); err != nil {
				log.Printf("Failed to watch cache directory: %v", err)
			} else {
				log.Printf("Watching cache directory for container file creation")
			}
			
			// Also try to watch dev subdirectories if they exist
			entries, err := os.ReadDir(cacheDir)
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() && strings.HasPrefix(entry.Name(), "dev") {
						devDir := filepath.Join(cacheDir, entry.Name())
						if err := cw.watcher.Add(devDir); err != nil {
							log.Printf("Failed to watch dev directory %s: %v", devDir, err)
						} else {
							log.Printf("Watching dev directory %s for container file creation", devDir)
						}
					}
				}
			}
		}
		
		return err
	}

	cw.containerPath = containerPath
	cw.containerExists = true

	// Add the directory to the watcher
	containerDir := filepath.Dir(containerPath)
	if err := cw.watcher.Add(containerDir); err != nil {
		return err
	}

	// Load the container file
	return cw.loadContainer()
}

// findContainerFile searches for the Symfony container XML file
func (cw *ContainerWatcher) findContainerFile() (string, error) {
	cacheDir := filepath.Join(cw.projectRoot, "var", "cache")
	
	// Check if the cache directory exists
	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		return "", err
	}

	// Pattern to match Shopware_Core_KernelDevDebugContainer.xml
	pattern := filepath.Join(cacheDir, "dev*", "Shopware_Core_KernelDevDebugContainer.xml")
	
	// Find matching files
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", err
	}

	// Use the first match if any
	if len(matches) > 0 {
		return matches[0], nil
	}

	return "", os.ErrNotExist
}

// loadContainer loads the container XML file into memory
func (cw *ContainerWatcher) loadContainer() error {
	// Read the file
	content, err := os.ReadFile(cw.containerPath)
	if err != nil {
		return err
	}

	// Parse the XML
	services, aliases, params, err := ParseXMLServices(content, cw.containerPath)
	if err != nil {
		return err
	}

	// Update the in-memory cache
	cw.mu.Lock()
	defer cw.mu.Unlock()

	// Clear existing data
	cw.services = make(map[string]Service, len(services))
	cw.aliases = make(map[string]ServiceAlias, len(aliases))
	cw.parameters = make(map[string]Parameter, len(params))

	// Store the new data
	for _, service := range services {
		cw.services[service.ID] = service
	}

	for _, alias := range aliases {
		cw.aliases[alias.ID] = alias
	}

	for _, param := range params {
		cw.parameters[param.Name] = param
	}

	cw.lastUpdated = time.Now()
	log.Printf("Loaded %d services, %d aliases, and %d parameters from container XML", 
		len(services), len(aliases), len(params))

	return nil
}

// watchChanges monitors the container file for changes
func (cw *ContainerWatcher) watchChanges() {
	for {
		select {
		case event, ok := <-cw.watcher.Events:
			if !ok {
				return
			}

			// Check if the event is for our container file
			if cw.containerExists && event.Name == cw.containerPath && (event.Op&(fsnotify.Write|fsnotify.Create) != 0) {
				log.Printf("Container file changed, reloading")
				if err := cw.loadContainer(); err != nil {
					log.Printf("Failed to reload container: %v", err)
				}
			} else if !cw.containerExists && strings.HasSuffix(event.Name, "Shopware_Core_KernelDevDebugContainer.xml") && (event.Op&fsnotify.Create != 0) {
				// Container file was created
				log.Printf("Container file created: %s", event.Name)
				cw.containerPath = event.Name
				cw.containerExists = true
				if err := cw.loadContainer(); err != nil {
					log.Printf("Failed to load new container: %v", err)
				}
			}

		case err, ok := <-cw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)
		}
	}
}

// GetServiceByID returns a service by ID from memory
func (cw *ContainerWatcher) GetServiceByID(id string) (Service, bool) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	service, found := cw.services[id]
	return service, found
}

// GetAliasByID returns an alias by ID from memory
func (cw *ContainerWatcher) GetAliasByID(id string) (ServiceAlias, bool) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	alias, found := cw.aliases[id]
	return alias, found
}

// GetParameterByName returns a parameter by name from memory
func (cw *ContainerWatcher) GetParameterByName(name string) (Parameter, bool) {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	param, found := cw.parameters[name]
	return param, found
}

// GetAllServices returns all services from memory
func (cw *ContainerWatcher) GetAllServices() []string {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	
	result := make([]string, 0, len(cw.services))
	for id := range cw.services {
		result = append(result, id)
	}

	return result
}

// Close stops the watcher and cleans up resources
func (cw *ContainerWatcher) Close() error {
	return cw.watcher.Close()
}

// ContainerExists returns true if the container file exists
func (cw *ContainerWatcher) ContainerExists() bool {
	return cw.containerExists
}

// LastUpdated returns the time when the container was last updated
func (cw *ContainerWatcher) LastUpdated() time.Time {
	cw.mu.RLock()
	defer cw.mu.RUnlock()
	return cw.lastUpdated
}
