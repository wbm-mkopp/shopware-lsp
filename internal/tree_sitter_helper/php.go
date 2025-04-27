package treesitterhelper

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

func IsPHPThisMethodCall(node *tree_sitter.Node, content []byte, methodName string) Pattern {
	return And(
		Or(
			NodeKind("string_content"),
			NodeKind("encapsed_string"),
			NodeKind("string"),
		),
		Ancestor(
			And(
				NodeKind("member_call_expression"),
				HasChild(And(
					NodeKind("name"),
					NodeText(methodName),
				)),
			),
			4, // Maximum depth to search up for ancestor
		),
	)
}

func IsPHPRenderStorefrontCall(node *tree_sitter.Node, content []byte) bool {
	pattern := And(
		NodeKind("string_content"),
		Ancestor(
			And(
				NodeKind("member_call_expression"),
				HasChild(And(
					NodeKind("name"),
					NodeText("renderStorefront"),
				)),
			),
			4, // Maximum depth to search up for ancestor
		),
	)

	return pattern.Matches(node, content)
}

// Pattern-based implementation for editor completion
func IsPHPRenderStorefrontCallEdit(node *tree_sitter.Node, content []byte) bool {
	// For PHP editor, handle both raw string and string_content nodes
	pattern := And(
		Or(
			NodeKind("string_content"),
			NodeKind("encapsed_string"),
			NodeKind("string"),
		),
		Ancestor(
			And(
				NodeKind("member_call_expression"),
				HasChild(And(
					NodeKind("name"),
					NodeText("renderStorefront"),
				)),
			),
			4, // Maximum depth to search up for ancestor
		),
	)

	return pattern.Matches(node, content)
}

func IsPHPRedirectToRoute(node *tree_sitter.Node, content []byte) bool {
	// For PHP editor, handle both raw string and string_content nodes
	pattern := And(
		Or(
			NodeKind("string_content"),
			NodeKind("encapsed_string"),
			NodeKind("string"),
		),
		Ancestor(
			And(
				NodeKind("member_call_expression"),
				HasChild(And(
					NodeKind("name"),
					NodeText("redirectToRoute"),
				)),
			),
			4, // Maximum depth to search up for ancestor
		),
	)

	return pattern.Matches(node, content)
}
