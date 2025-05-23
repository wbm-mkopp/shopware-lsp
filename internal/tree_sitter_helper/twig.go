package treesitterhelper

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TwigTransPattern() Pattern {
	return And(
		NodeKind("string"),
		Ancestor(
			And(
				NodeKind("filter_expression"),
				HasChild(
					And(
						NodeKind("function"),
						NodeText("trans"),
					),
				),
			),
			1,
		),
	)
}

func TwigBlockWithNamePattern(blockName string) Pattern {
	return And(
		NodeKind("block"),
		HasChild(
			And(
				NodeKind("identifier"),
				NodeText(blockName),
			),
		),
	)
}

func TwigAutocompleteFilterPattern() Pattern {
	return Or(
		// {{ foo|<caret> }}
		And(
			NodeKind("operator"),
			Ancestor(NodeKind("filter_expression"), 1),
		),

		// {{ foo|test<caret> }}
		And(
			NodeKind("function"),
			Ancestor(NodeKind("filter_expression"), 1),
		),
	)
}

func TwigSwIconInPackPattern() Pattern {
	return And(
		NodeKind("string"),
		Ancestor(
			And(
				NodeKind("pair"),
				HasChild(
					And(
						NodeKind("string"),
						NodeText("'pack'"),
					),
				),
				Ancestor(
					And(
						NodeKind("tag"),
						HasChild(
							And(
								NodeKind("keyword"),
								NodeText("sw_icon"),
							),
						),
					),
					2,
				),
			),
			1,
		),
	)
}

// ExtractSwIconObjectToMap extracts the object from a sw_icon tag and converts it to a Go map
func ExtractSwIconObjectToMap(tagNode *tree_sitter.Node, content []byte) map[string]string {
	result := make(map[string]string)

	// Find the object node within the tag
	var objectNode *tree_sitter.Node
	for i := 0; i < int(tagNode.ChildCount()); i++ {
		child := tagNode.Child(uint(i))
		if child != nil && child.Kind() == "object" {
			objectNode = child
			break
		}
	}

	if objectNode == nil {
		return result
	}

	// Extract pairs from the object
	for i := 0; i < int(objectNode.ChildCount()); i++ {
		child := objectNode.Child(uint(i))
		if child != nil && child.Kind() == "pair" {
			key, value := extractPairKeyValue(child, content)
			if key != "" && value != "" {
				result[key] = value
			}
		}
	}

	return result
}

// extractPairKeyValue extracts key and value from a pair node
func extractPairKeyValue(pairNode *tree_sitter.Node, content []byte) (string, string) {
	var key, value string

	for i := 0; i < int(pairNode.ChildCount()); i++ {
		child := pairNode.Child(uint(i))
		if child == nil {
			continue
		}

		switch child.Kind() {
		case "string":
			text := string(child.Utf8Text(content))
			// Remove quotes from string
			text = strings.Trim(text, "'\"")

			if key == "" {
				key = text
			} else if value == "" {
				value = text
			}
		}
	}

	return key, value
}
