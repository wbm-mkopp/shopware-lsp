package symfony

import (
	"fmt"
	"path/filepath"
)

// ServiceIndexer implements the IndexerProvider interface for Symfony services
type ServiceIndexer struct {
	serviceIndex *ServiceIndex
	projectRoot  string
}

// NewServiceIndexer creates a new Symfony service indexer
func NewServiceIndexer(projectRoot string) (*ServiceIndexer, error) {
	serviceIndex, err := NewServiceIndex(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to create service index: %w", err)
	}

	return &ServiceIndexer{
		serviceIndex: serviceIndex,
		projectRoot:  projectRoot,
	}, nil
}

// ID returns a unique identifier for this indexer
func (i *ServiceIndexer) ID() string {
	return "symfony.service"
}

// Name returns a human-readable name for this indexer
func (i *ServiceIndexer) Name() string {
	return "Symfony Service Indexer"
}

// Index builds or updates the index
func (i *ServiceIndexer) Index() error {
	return i.serviceIndex.BuildIndex()
}

// Close cleans up resources used by the indexer
func (i *ServiceIndexer) Close() error {
	return i.serviceIndex.Close()
}

// GetServiceIndex returns the underlying service index
func (i *ServiceIndexer) GetServiceIndex() *ServiceIndex {
	return i.serviceIndex
}

// GetServiceCount returns the number of services in the index
func (i *ServiceIndexer) GetServiceCount() int {
	serviceCount, _ := i.serviceIndex.GetCounts()
	return serviceCount
}

// GetAliasCount returns the number of aliases in the index
func (i *ServiceIndexer) GetAliasCount() int {
	_, aliasCount := i.serviceIndex.GetCounts()
	return aliasCount
}

// GetTagCount returns the number of tags in the index
func (i *ServiceIndexer) GetTagCount() int {
	return i.serviceIndex.GetTagCount()
}

// GetProjectRoot returns the project root path
func (i *ServiceIndexer) GetProjectRoot() string {
	return i.projectRoot
}

// GetConfigDir returns the Symfony config directory
func (i *ServiceIndexer) GetConfigDir() string {
	return filepath.Join(i.projectRoot, "config")
}
