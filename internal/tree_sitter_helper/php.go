package treesitterhelper

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

// Pattern-based implementation
func IsPHPRenderStorefrontCall(node *tree_sitter.Node, content []byte) bool {
	pattern := And(
		NodeKind("string_content"),
		ParentOfKind("member_call_expression", 4),
		Ancestor(
			And(
				NodeKind("member_call_expression"),
				HasChild(And(
					NodeKind("name"),
					NodeText("renderStorefront"),
				)),
			),
			4,
		),
	)
	
	return pattern.Matches(node, content)
}

// Pattern-based implementation
func IsPHPRenderStorefrontCallEdit(node *tree_sitter.Node, content []byte) bool {
	pattern := And(
		NodeKind("string"),
		ParentOfKind("member_call_expression", 3),
		Ancestor(
			And(
				NodeKind("member_call_expression"),
				HasChild(And(
					NodeKind("name"),
					NodeText("renderStorefront"),
				)),
			),
			3,
		),
	)
	
	return pattern.Matches(node, content)
}
