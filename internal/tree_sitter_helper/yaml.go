package treesitterhelper

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

var isServicesNode = And(
	NodeKind("block_mapping_pair"),
	HasChild(
		And(
			NodeKind("flow_node"),
			HasChild(
				And(
					NodeKind("plain_scalar"),
					HasChild(
						And(
							NodeKind("string_scalar"),
							NodeText("services"),
						),
					),
				),
			),
		),
	),
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
	} else if node.Kind() == "string_scalar" {
		return string(node.Utf8Text(source))
	} else if node.Kind() == "single_quote_scalar" {
		return strings.Trim(node.Utf8Text(source), "\"'")
	}

	return ""
}

func IsYamlServiceId(node *tree_sitter.Node, source []byte) bool {
	return Or(
		And(
			NodeKind("block_mapping_pair"),
			Ancestor(
				isServicesNode,
				3,
			),
		),
		And(
			NodeKind("string_scalar"),
			Ancestor(
				And(
					NodeKind("flow_node"),
					NodeName("key"),
					Ancestor(
						isServicesNode,
						7,
					),
				),
				2,
			),
		),
	).Matches(node, source)
}

func IsYamlArgumentServiceId(node *tree_sitter.Node, source []byte) bool {
	return And(
		NodeKind("single_quote_scalar"),
		Ancestor(
			And(
				NodeKind("block_node"),
				Ancestor(
					And(
						NodeKind("block_mapping_pair"),
						HasChild(
							And(
								NodeKind("flow_node"),
								HasChild(
									And(
										NodeKind("plain_scalar"),
										HasChild(
											And(
												NodeKind("string_scalar"),
												NodeText("arguments"),
											),
										),
									),
								),
							),
						),
						Ancestor(
							And(
								NodeKind("block_mapping_pair"),
								Ancestor(
									isServicesNode,
									3,
								),
							),
							3,
						),
					),
					1,
				),
			),
			4,
		),
	).Matches(node, source)
}

func IsYamlClassPropertyInService() Pattern {
	return And(
		NodeKind("string_scalar"),
		Ancestor(
			IsYamlClassPropertyInServiceToType(),
			4,
		),
	)
}

func IsYamlClassPropertyInServiceToType() Pattern {
	return And(
		NodeKind("block_mapping"),
		HasChild(
			And(
				NodeKind("block_mapping_pair"),
				HasChild(
					And(
						NodeKind("flow_node"),
						NodeName("key"),
						HasChild(
							And(
								NodeKind("plain_scalar"),
								NodeText("class"),
							),
						),
					),
				),
				Ancestor(
					And(
						NodeKind("block_mapping_pair"),
						Ancestor(
							isServicesNode,
							3,
						),
					),
					3,
				),
			),
		),
	)
}
