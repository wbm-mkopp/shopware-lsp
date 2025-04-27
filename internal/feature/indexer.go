package feature

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/indexer"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

type FeatureIndexer struct {
	featureIndex *indexer.DataIndexer[Feature]
}

func NewFeatureIndexer(configDir string) (*FeatureIndexer, error) {
	featureIndex, err := indexer.NewDataIndexer[Feature](filepath.Join(configDir, "feature_flags.db"))
	if err != nil {
		return nil, err
	}

	return &FeatureIndexer{
		featureIndex: featureIndex,
	}, nil
}

func (i *FeatureIndexer) ID() string {
	return "feature.indexer"
}

func (i *FeatureIndexer) Index(path string, node *sitter.Node, fileContent []byte) error {
	// Only index .yaml files that might contain feature flags
	if !strings.HasSuffix(path, ".yaml") && !strings.HasSuffix(path, ".yml") {
		return nil
	}

	// Check if the file contains "feature" in the path
	if !strings.Contains(strings.ToLower(path), "feature") {
		return nil
	}

	// Extract feature flags from the file
	features, err := ParseFeatureFile(node, fileContent, path)
	if err != nil {
		return fmt.Errorf("parsing feature file: %w", err)
	}

	// No features found, nothing to do
	if len(features) == 0 {
		return nil
	}

	// Store the features in the database
	batchSave := make(map[string]map[string]Feature)

	// Group features by file
	for _, feature := range features {
		if _, ok := batchSave[feature.File]; !ok {
			batchSave[feature.File] = make(map[string]Feature)
		}
		batchSave[feature.File][feature.Name] = feature
	}

	// Save to the database
	if err := i.featureIndex.BatchSaveItems(batchSave); err != nil {
		return fmt.Errorf("saving features: %w", err)
	}

	return nil
}

func (i *FeatureIndexer) RemovedFiles(paths []string) error {
	// Remove files from the database
	if err := i.featureIndex.BatchDeleteByFilePaths(paths); err != nil {
		return fmt.Errorf("removing features: %w", err)
	}

	return nil
}

func (i *FeatureIndexer) Close() error {
	return i.featureIndex.Close()
}

func (i *FeatureIndexer) Clear() error {
	return i.featureIndex.Clear()
}

func (i *FeatureIndexer) GetFeatureByName(name string) ([]Feature, error) {
	return i.featureIndex.GetValues(name)
}

func (i *FeatureIndexer) GetAllFeatures() ([]Feature, error) {
	return i.featureIndex.GetAllValues()
}
