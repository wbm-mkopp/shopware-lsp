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
			4,
		),
	)
}
