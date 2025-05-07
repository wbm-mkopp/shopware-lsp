package theme

import (
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/indexer"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_json "github.com/tree-sitter/tree-sitter-json/bindings/go"
)

// ThemeConfigIndexer is responsible for indexing theme.json files
type ThemeConfigIndexer struct {
	configIndex *indexer.DataIndexer[ThemeConfigField]
}

// NewThemeConfigIndexer creates a new theme config indexer
func NewThemeConfigIndexer(configDir string) (*ThemeConfigIndexer, error) {
	configIndexer, err := indexer.NewDataIndexer[ThemeConfigField](filepath.Join(configDir, "theme_config.db"))
	if err != nil {
		return nil, err
	}

	return &ThemeConfigIndexer{
		configIndex: configIndexer,
	}, nil
}

// ID returns the unique identifier for this indexer
func (t *ThemeConfigIndexer) ID() string {
	return "theme.indexer"
}

// Index processes a file and indexes any theme config fields found
func (t *ThemeConfigIndexer) Index(path string, node *tree_sitter.Node, fileContent []byte) error {
	// Skip non-theme.json files
	if !strings.HasSuffix(path, "theme.json") {
		return nil
	}

	// Parse the theme.json file
	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_json.Language())); err != nil {
		return err
	}

	tree := parser.Parse(fileContent, nil)
	if tree == nil {
		return nil
	}
	defer tree.Close()

	// Extract theme config fields
	fields, err := ParseThemeConfig(tree.RootNode(), fileContent, path)
	if err != nil {
		return err
	}

	// Prepare batch save
	batchSave := make(map[string]map[string]ThemeConfigField)

	for _, field := range fields {
		if _, ok := batchSave[field.Path]; !ok {
			batchSave[field.Path] = make(map[string]ThemeConfigField)
		}
		batchSave[field.Path][field.Key] = field
	}

	return t.configIndex.BatchSaveItems(batchSave)
}

// RemovedFiles handles cleanup when files are removed
func (t *ThemeConfigIndexer) RemovedFiles(paths []string) error {
	return t.configIndex.BatchDeleteByFilePaths(paths)
}

// Close closes the indexer
func (t *ThemeConfigIndexer) Close() error {
	return t.configIndex.Close()
}

// Clear clears all indexed data
func (t *ThemeConfigIndexer) Clear() error {
	return t.configIndex.Clear()
}

// GetThemeConfigFields returns all theme config field keys
func (t *ThemeConfigIndexer) GetThemeConfigFields() ([]string, error) {
	return t.configIndex.GetAllKeys()
}

// GetThemeConfigField returns all fields for a specific key
func (t *ThemeConfigIndexer) GetThemeConfigField(key string) ([]ThemeConfigField, error) {
	return t.configIndex.GetValues(key)
}

// GetAllThemeConfigFields returns all theme config fields
func (t *ThemeConfigIndexer) GetAllThemeConfigFields() ([]ThemeConfigField, error) {
	return t.configIndex.GetAllValues()
}

// IsThemeFile checks if a file is a theme.json file
func IsThemeFile(path string) bool {
	return strings.HasSuffix(path, "theme.json")
}
