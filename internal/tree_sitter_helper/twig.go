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
