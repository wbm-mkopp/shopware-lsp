package theme

import (
	"fmt"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ParseThemeConfig parses the config section from a theme.json file and returns a slice of ThemeConfigField
func ParseThemeConfig(root *tree_sitter.Node, document []byte, filePath string) ([]ThemeConfigField, error) {
	// Find the object node which is the first child of the document node
	if root.Kind() == "document" && root.NamedChildCount() > 0 {
		root = root.NamedChild(0) // Get the object node
	}

	if root.Kind() != "object" {
		return nil, fmt.Errorf("root node is not an object: %s", root.Kind())
	}

	// Final result with all fields
	var fields []ThemeConfigField

	// Find the config section in the theme.json
	configNode := findConfigNode(root, document)
	if configNode == nil {
		return fields, nil // Return empty slice if no config section found
	}

	// Parse blocks and fields
	for i := 0; i < int(configNode.NamedChildCount()); i++ {
		pair := configNode.NamedChild(uint(i))

		if pair.Kind() == "pair" {
			key := pair.NamedChild(0)
			value := pair.NamedChild(1)

			if key != nil && key.Kind() == "string" {
				keyText := extractStringContent(key, document)

				if keyText == "fields" && value.Kind() == "object" {
					// Parse fields and append to result
					fields = parseFields(value, document, filePath)
				}
			}
		}
	}

	return fields, nil
}

// findConfigNode finds the config section in the theme.json
func findConfigNode(root *tree_sitter.Node, document []byte) *tree_sitter.Node {
	for i := 0; i < int(root.NamedChildCount()); i++ {
		pair := root.NamedChild(uint(i))

		if pair.Kind() == "pair" {
			key := pair.NamedChild(0)
			value := pair.NamedChild(1)

			if key != nil && key.Kind() == "string" {
				keyText := extractStringContent(key, document)

				if keyText == "config" && value.Kind() == "object" {
					return value
				}
			}
		}
	}
	return nil
}

// extractStringContent extracts a string content from a string node
func extractStringContent(node *tree_sitter.Node, content []byte) string {
	if node.NamedChildCount() > 0 && node.NamedChild(0).Kind() == "string_content" {
		stringContent := node.NamedChild(0)
		return string(stringContent.Utf8Text(content))
	}

	// Fallback
	str := string(node.Utf8Text(content))
	return strings.Trim(str, "\"")
}

// parseFields parses the fields in the config section and returns a slice of ThemeConfigField
func parseFields(node *tree_sitter.Node, content []byte, filePath string) []ThemeConfigField {
	var fields []ThemeConfigField

	for i := 0; i < int(node.NamedChildCount()); i++ {
		pair := node.NamedChild(uint(i))

		if pair.Kind() == "pair" {
			key := pair.NamedChild(0)
			value := pair.NamedChild(1)

			if key != nil && key.Kind() == "string" && value.Kind() == "object" {
				fieldKey := extractStringContent(key, content)
				field := ThemeConfigField{
					Key:   fieldKey,
					Label: make(map[string]string),
					Scss:  true,
					Path:  filePath,
					Line:  int(pair.Range().StartPoint.Row) + 1, // Convert to 1-based line number
				}

				// Parse the field properties
				for j := 0; j < int(value.NamedChildCount()); j++ {
					fieldPair := value.NamedChild(uint(j))

					if fieldPair.Kind() == "pair" {
						fieldPropKey := fieldPair.NamedChild(0)
						fieldPropValue := fieldPair.NamedChild(1)

						if fieldPropKey != nil && fieldPropKey.Kind() == "string" {
							fieldPropKeyText := extractStringContent(fieldPropKey, content)

							switch fieldPropKeyText {
							case "label":
								if fieldPropValue.Kind() == "object" {
									extractLabels(fieldPropValue, content, field.Label)
								}
							case "type":
								if fieldPropValue.Kind() == "string" {
									field.Type = extractStringContent(fieldPropValue, content)
								}
							case "value":
								if fieldPropValue.Kind() == "string" {
									field.Value = extractStringContent(fieldPropValue, content)
								}
							case "editable":
								if fieldPropValue.Kind() == "true" {
									field.Editable = true
								} else if fieldPropValue.Kind() == "false" {
									field.Editable = false
								}
							case "scss":
								if fieldPropValue.Kind() == "true" {
									field.Scss = true
								} else if fieldPropValue.Kind() == "false" {
									field.Scss = false
								}
							case "order":
								if fieldPropValue.Kind() == "number" {
									// For simplicity, we assume its an integer
									orderStr := string(fieldPropValue.Utf8Text(content))
									var order int
									if _, err := fmt.Sscanf(orderStr, "%d", &order); err == nil {
										field.Order = order
									}
								}
							case "block":
								if fieldPropValue.Kind() == "string" {
									field.Block = extractStringContent(fieldPropValue, content)
								}
							}
						}
					}
				}

				fields = append(fields, field)
			}
		}
	}

	return fields
}

// extractLabels extracts localized labels from an object
func extractLabels(node *tree_sitter.Node, content []byte, result map[string]string) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		pair := node.NamedChild(uint(i))

		if pair.Kind() == "pair" {
			key := pair.NamedChild(0)
			value := pair.NamedChild(1)

			if key != nil && key.Kind() == "string" && value.Kind() == "string" {
				locale := extractStringContent(key, content)
				label := extractStringContent(value, content)
				result[locale] = label
			}
		}
	}
}
