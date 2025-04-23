package symfony

import (
	"fmt"
	"log"
	"path/filepath"

	"github.com/shopware/shopware-lsp/internal/indexer"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ServiceIndex maintains an index of all service IDs from XML files
type ServiceIndex struct {
	projectRoot      string
	serviceIndex     *indexer.DataIndexer[Service]
	parameterIndex   *indexer.DataIndexer[Parameter]
	containerWatcher *ContainerWatcher
}

// NewServiceIndex creates a new service indexer for the given project root
func NewServiceIndex(projectRoot string, configDir string) (*ServiceIndex, error) {
	serviceIndex, err := indexer.NewDataIndexer[Service](filepath.Join(configDir, "symfony.service"))
	if err != nil {
		return nil, fmt.Errorf("failed to create service index: %w", err)
	}

	parameterIndex, err := indexer.NewDataIndexer[Parameter](filepath.Join(configDir, "symfony.parameter"))
	if err != nil {
		return nil, fmt.Errorf("failed to create parameter index: %w", err)
	}

	idx := &ServiceIndex{
		projectRoot:    projectRoot,
		serviceIndex:   serviceIndex,
		parameterIndex: parameterIndex,
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
func (idx *ServiceIndex) Index(path string, node *tree_sitter.Node, fileContent []byte) error {
	services, params, err := ParseXMLServices(path, node, fileContent)
	if err != nil {
		return err
	}

	serviceWrite := make(map[string]map[string]Service)
	parameterWrite := make(map[string]map[string]Parameter)

	for _, service := range services {
		if _, ok := serviceWrite[service.ID]; !ok {
			serviceWrite[service.ID] = make(map[string]Service)
		}
		serviceWrite[service.Path][service.ID] = service
	}

	for _, param := range params {
		if _, ok := parameterWrite[param.Name]; !ok {
			parameterWrite[param.Name] = make(map[string]Parameter)
		}
		parameterWrite[param.Path][param.Name] = param
	}

	if err := idx.parameterIndex.BatchSaveItems(parameterWrite); err != nil {
		return err
	}

	if err := idx.serviceIndex.BatchSaveItems(serviceWrite); err != nil {
		return err
	}

	return nil
}

func (idx *ServiceIndex) RemovedFiles(paths []string) error {
	if err := idx.serviceIndex.BatchDeleteByFilePaths(paths); err != nil {
		return err
	}

	if err := idx.parameterIndex.BatchDeleteByFilePaths(paths); err != nil {
		return err
	}

	return nil
}

// GetAllServices returns all indexed service IDs
func (idx *ServiceIndex) GetAllServices() []string {
	dbServiceIDs, err := idx.serviceIndex.GetAllKeys()
	if err != nil {
		panic(err)
	}

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
	services, err := idx.serviceIndex.GetValues(id)
	if err != nil {
		return Service{}, false
	}

	if len(services) > 0 {
		return services[0], true
	}

	// If not found in database, fallback to container watcher
	if idx.containerWatcher != nil && idx.containerWatcher.ContainerExists() {
		return idx.containerWatcher.GetServiceByID(id)
	}

	return Service{}, false
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

	if err := idx.serviceIndex.Close(); err != nil {
		return err
	}

	if err := idx.parameterIndex.Close(); err != nil {
		return err
	}

	return err
}

// GetAllTags returns all tag names in the index
func (idx *ServiceIndex) GetAllTags() []string {
	panic("not implemented")
}

// GetServicesByTag returns all service IDs that have the specified tag
func (idx *ServiceIndex) GetServicesByTag(tagName string) []string {
	panic("not implemented")
}

// GetAllParameters returns all parameter names in the index
func (idx *ServiceIndex) GetAllParameters() []Parameter {
	panic("not implemented")
}

// GetParameterByName returns a specific parameter value by its name
func (idx *ServiceIndex) GetParameterByName(name string) (Parameter, bool) {
	panic("not implemented")
}

type Location struct {
	Path string
	Line int
}

func (idx *ServiceIndex) GetServicesUsageByClassName(className string) []Location {
	panic("not implemented")
}
