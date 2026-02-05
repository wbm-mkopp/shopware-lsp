package treesitterhelper

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

// JSThisMethodCallPattern matches this.$tc('key') or this.$t('key') patterns
// Used for admin snippet translations in JavaScript files
func JSThisMethodCallPattern(methodNames ...string) Pattern {
	if len(methodNames) == 0 {
		// Return a pattern that never matches
		return FuncPattern(func(node *tree_sitter.Node, content []byte) bool {
			return false
		})
	}

	// Build OR pattern for method names
	var methodPatterns []Pattern
	for _, name := range methodNames {
		methodPatterns = append(methodPatterns, NodeText(name))
	}

	var methodMatcher Pattern
	if len(methodPatterns) == 1 {
		methodMatcher = methodPatterns[0]
	} else {
		methodMatcher = Or(methodPatterns...)
	}

	// Pattern to check if we're in the right call_expression context
	callExpressionPattern := And(
		NodeKind("call_expression"),
		HasChild(
			And(
				NodeKind("member_expression"),
				HasChild(
					NodeKind("this"),
				),
				HasChild(
					And(
						NodeKind("property_identifier"),
						methodMatcher,
					),
				),
			),
		),
	)

	return FuncPattern(func(node *tree_sitter.Node, content []byte) bool {
		// Check the node itself
		nodeKind := node.Kind()

		// If we're on a string or string_fragment, check ancestors
		if nodeKind == "string" || nodeKind == "string_fragment" {
			return Ancestor(callExpressionPattern, 3).Matches(node, content)
		}

		// If we're on an unnamed node (like quote character), check if parent is string
		// and that string is in the right context
		parent := node.Parent()
		if parent != nil && parent.Kind() == "string" {
			return Ancestor(callExpressionPattern, 3).Matches(parent, content)
		}

		return false
	})
}

// JSAdminSnippetPattern matches this.$tc('key') or this.$t('key') patterns
func JSAdminSnippetPattern() Pattern {
	return JSThisMethodCallPattern("$tc", "$t")
}
