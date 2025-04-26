package snippet

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Snippet struct {
	Text string
	File string
	Line int
}

func parseSnippetFile(root *tree_sitter.Node, document []byte, filePath string) (map[string]Snippet, error) {
	// Find the object node which is the first child of the document node
	if root.Kind() == "document" && root.NamedChildCount() > 0 {
		root = root.NamedChild(0) // Get the object node
	}

	result := make(map[string]Snippet)
	extractValues("", root, document, result, filePath)

	return result, nil
}

func extractValues(prefix string, node *tree_sitter.Node, content []byte, result map[string]Snippet, filePath string) {
	// Check if this is an object with key-value pairs
	if node.Kind() == "object" {
		// Iterate through child nodes
		for i := 0; i < int(node.NamedChildCount()); i++ {
			pair := node.NamedChild(uint(i))

			if pair.Kind() == "pair" {
				// Get key and value
				key := pair.NamedChild(0)
				value := pair.NamedChild(1)

				if key != nil && key.Kind() == "string" {
					// Find the string_content node inside the string node
					var keyText string
					if key.NamedChildCount() > 0 && key.NamedChild(0).Kind() == "string_content" {
						keyContent := key.NamedChild(0)
						keyText = string(keyContent.Utf8Text(content))
					} else {
						// Fallback
						keyText = string(key.Utf8Text(content))
						keyText = strings.Trim(keyText, "\"")
					}

					// Build the new prefix
					newPrefix := keyText
					if prefix != "" {
						newPrefix = prefix + "." + keyText
					}

					if value.Kind() == "object" {
						// If value is an object, recursively extract its values
						extractValues(newPrefix, value, content, result, filePath)
					} else if value.Kind() == "string" {
						// Find the string_content node inside the string node
						var valueText string
						if value.NamedChildCount() > 0 && value.NamedChild(0).Kind() == "string_content" {
							valueContent := value.NamedChild(0)
							valueText = string(valueContent.Utf8Text(content))
						} else {
							// Fallback
							valueText = string(value.Utf8Text(content))
							valueText = strings.Trim(valueText, "\"")
						}
						result[newPrefix] = Snippet{
							Text: valueText,
							File: filePath,
							Line: int(value.Range().StartPoint.Row) + 1,
						}
					} else if value.Kind() == "number" || value.Kind() == "true" || value.Kind() == "false" || value.Kind() == "null" {
						// For non-string primitive values, convert to string
						valueText := string(value.Utf8Text(content))
						result[newPrefix] = Snippet{
							Text: valueText,
							File: filePath,
							Line: int(value.Range().StartPoint.Row) + 1,
						}
					}
				}
			}
		}
	}
}
