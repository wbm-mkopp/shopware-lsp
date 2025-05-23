package feature

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	tree_sitter_yaml "github.com/tree-sitter-grammars/tree-sitter-yaml/bindings/go"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestParseFeatureFile(t *testing.T) {
	// Read the test file
	filePath := filepath.Join("testdata", "feature.yaml")
	content, err := os.ReadFile(filePath)
	require.NoError(t, err, "Reading test file should not fail")

	// Parse the YAML file with tree-sitter
	parser := sitter.NewParser()
	err = parser.SetLanguage(sitter.NewLanguage(tree_sitter_yaml.Language()))
	require.NoError(t, err, "Setting language should not fail")

	tree := parser.Parse(content, nil)
	require.NotNil(t, tree, "Parsing YAML should not fail")

	// Parse the features from the file
	features, err := ParseFeatureFile(tree.RootNode(), content, filePath)
	require.NoError(t, err, "Parsing feature file should not fail")
	require.Len(t, features, 8, "Should find 8 features in the test file")

	// Verify the expected features are present
	expectedFeatures := map[string]int{
		"v6.5.0.0":                              4,
		"v6.6.0.0":                              8,
		"v6.7.0.0":                              12,
		"v6.8.0.0":                              16,
		"DISABLE_VUE_COMPAT":                    20,
		"ACCESSIBILITY_TWEAKS":                  24,
		"TELEMETRY_METRICS":                     29,
		"FLOW_EXECUTION_AFTER_BUSINESS_PROCESS": 34,
	}

	for _, feature := range features {
		expectedLine, ok := expectedFeatures[feature.Name]
		assert.True(t, ok, "Feature %s should be in the expected list", feature.Name)
		assert.Equal(t, expectedLine, feature.Line, "Feature %s should be at line %d", feature.Name, expectedLine)
		assert.Equal(t, filePath, feature.File, "Feature %s should have the correct file path", feature.Name)
	}
}
