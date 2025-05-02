package treesitterhelper

import (
	"slices"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func IsSCSSFunctionPattern(funcNames ...string) Pattern {
	return And(
		NodeKind("string_value"),
		Ancestor(
			And(
				NodeKind("call_expression"),
				HasChild(And(
					NodeKind("function_name"),
					FuncPattern(func(node *tree_sitter.Node, content []byte) bool {
						funcName := string(node.Utf8Text(content))

						return slices.Contains(funcNames, funcName)
					}),
				)),
			),
			2,
		),
	)
}
