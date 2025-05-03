package systemconfig

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/indexer"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// SystemConfigIndexer is responsible for indexing system config XML files
type SystemConfigIndexer struct {
	configIndex *indexer.DataIndexer[SystemConfigEntry]
}

// NewSystemConfigIndexer creates a new system config indexer
func NewSystemConfigIndexer(configDir string) (*SystemConfigIndexer, error) {
	configIndexer, err := indexer.NewDataIndexer[SystemConfigEntry](filepath.Join(configDir, "system_config.db"))
	if err != nil {
		return nil, err
	}

	return &SystemConfigIndexer{
		configIndex: configIndexer,
	}, nil
}

// ID returns the unique identifier for this indexer
func (s *SystemConfigIndexer) ID() string {
	return "systemconfig.indexer"
}

// Index processes a file and indexes any system config entries found
func (s *SystemConfigIndexer) Index(path string, node *tree_sitter.Node, fileContent []byte) error {
	// Skip non-system config files
	if !strings.HasSuffix(path, ".xml") || strings.Contains(path, "/_fixtures/") || strings.Contains(path, "/_fixture/") {
		return nil
	}

	// Check if it's a system config XML file
	if !IsSystemConfigXML(fileContent) {
		return nil
	}

	// We already have the file content, so we can pass it directly
	entries, err := IndexSystemConfigFile(fileContent, path)
	if err != nil {
		return err
	}

	log.Printf("Indexed %d system config entries from %s", len(entries), path)

	for _, entry := range entries {
		if entry.Namespace != "" {
			log.Printf("Entry: %s", fmt.Sprintf("%s.%s", entry.Namespace, entry.Name))
		} else {
			log.Printf("Warning: Empty namespace for entry %s in file %s", entry.Name, path)
		}
	}

	// Prepare batch save
	batchSave := make(map[string]map[string]SystemConfigEntry)

	for _, entry := range entries {
		// Use the fully qualified name (namespace + name) as the key
		entryKey := entry.Namespace + "." + entry.Name

		if _, ok := batchSave[entry.FilePath]; !ok {
			batchSave[entry.FilePath] = make(map[string]SystemConfigEntry)
		}
		batchSave[entry.FilePath][entryKey] = entry
	}

	return s.configIndex.BatchSaveItems(batchSave)
}

// RemovedFiles handles cleanup when files are removed
func (s *SystemConfigIndexer) RemovedFiles(paths []string) error {
	return s.configIndex.BatchDeleteByFilePaths(paths)
}

// Close closes the indexer
func (s *SystemConfigIndexer) Close() error {
	return s.configIndex.Close()
}

// Clear clears all indexed data
func (s *SystemConfigIndexer) Clear() error {
	return s.configIndex.Clear()
}

// GetSystemConfigEntries returns all system config entry keys
func (s *SystemConfigIndexer) GetSystemConfigEntries() ([]string, error) {
	return s.configIndex.GetAllKeys()
}

// GetSystemConfigEntry returns all entries for a specific key
func (s *SystemConfigIndexer) GetSystemConfigEntry(key string) ([]SystemConfigEntry, error) {
	return s.configIndex.GetValues(key)
}

// GetAllSystemConfigEntries returns all system config entries
func (s *SystemConfigIndexer) GetAllSystemConfigEntries() ([]SystemConfigEntry, error) {
	return s.configIndex.GetAllValues()
}
