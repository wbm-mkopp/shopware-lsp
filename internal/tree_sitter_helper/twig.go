package treesitterhelper

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
