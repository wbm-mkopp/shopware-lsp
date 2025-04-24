package treesitterhelper

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// GetYAMLValue extracts the scalar value from a YAML node
func GetYAMLValue(node *tree_sitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	// Handle different node types
	if node.Kind() == "flow_node" {
		// Extract value and remove quotes if present
		value := string(node.Utf8Text(source))
		return strings.Trim(value, "\"'")
	} else if node.Kind() == "block_scalar" {
		// For multiline values
		return string(node.Utf8Text(source))
	}

	return ""
}
