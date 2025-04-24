package treesitterhelper

import (
	"testing"

	"github.com/stretchr/testify/assert"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

func TestPHPPatterns(t *testing.T) {
	parser := tree_sitter.NewParser()
	assert.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())))

	phpCode := []byte(`<?php
	// PHP renderStorefront example
	$controller->renderStorefront("some/template.html.twig", ["foo" => "bar"]);
	
	// Other PHP code
	$controller->render("other/template.html.twig");
	`)

	tree := parser.Parse(phpCode, nil)
	defer tree.Close()

	// Define our pattern for renderStorefront
	renderStorefrontPattern := And(
		NodeKind("string_content"),
		FuncPattern(func(node *tree_sitter.Node, content []byte) bool {
			// Check if the text contains the template path
			text := string(node.Utf8Text(content))
			return text == "some/template.html.twig"
		}),
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

	// Find all matches in the parsed tree
	matches := FindAll(tree.RootNode(), renderStorefrontPattern, phpCode)

	// Verify we found just the renderStorefront string node
	assert.Equal(t, 1, len(matches), "Should find exactly one renderStorefront call")
	assert.Contains(t, string(matches[0].Utf8Text(phpCode)), "some/template.html.twig",
		"Found string node should contain template path")

	// Test logical operator patterns
	nonRenderStorefrontPattern := And(
		NodeKind("string_content"),
		FuncPattern(func(node *tree_sitter.Node, content []byte) bool {
			// Check if the text contains the non-renderStorefront template path
			text := string(node.Utf8Text(content))
			return text == "other/template.html.twig"
		}),
		Not(
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
		),
	)

	nonRenderMatches := FindAll(tree.RootNode(), nonRenderStorefrontPattern, phpCode)
	assert.Equal(t, 1, len(nonRenderMatches), "Should find non-renderStorefront string node")
	assert.Contains(t, string(nonRenderMatches[0].Utf8Text(phpCode)), "other/template.html.twig",
		"Found string node should contain the non-renderStorefront template path")
}

func TestTwigPatterns(t *testing.T) {
	parser := tree_sitter.NewParser()
	// Skip test if Twig language not available
	t.Skip("Skipping test that requires tree-sitter-twig")

	twigCode := []byte(`{% block content %}
	<div>
		Some content
		{% block nested %}
			Nested content
		{% endblock %}
	</div>
{% endblock %}

{% block footer %}
	Footer content
{% endblock %}`)

	tree := parser.Parse(twigCode, nil)
	defer tree.Close()

	// Define pattern for Twig blocks
	blockPattern := And(
		NodeKind("tag"),
		HasChild(And(
			NodeKind("tag_name"),
			NodeText("block"),
		)),
	)

	// Find all blocks
	blocks := FindAll(tree.RootNode(), blockPattern, twigCode)
	assert.Equal(t, 3, len(blocks), "Should find three block tags")

	// Test capture pattern to extract block names
	blockNames := []string{}
	for _, blockNode := range blocks {
		nameCapture := Capture("blockName", NodeKind("string"))

		blockWithNamePattern := And(
			blockPattern,
			HasChild(nameCapture),
		)

		if blockWithNamePattern.Matches(blockNode, twigCode) {
			nameNode := nameCapture.GetCapturedNode()
			if nameNode != nil {
				blockNames = append(blockNames, string(nameNode.Utf8Text(twigCode)))
			}
		}
	}

	assert.ElementsMatch(t, []string{"content", "nested", "footer"}, blockNames,
		"Should extract all block names correctly")
}

func TestPatternComposition(t *testing.T) {
	// Create example patterns to test composition
	pattern1 := NodeKind("tag")
	pattern2 := NodeText("test")
	pattern3 := HasChild(NodeKind("string"))

	// Test AND composition
	andPattern := And(pattern1, pattern2, pattern3)

	assert.NotNil(t, andPattern, "AND pattern should be properly constructed")

	// Test OR composition
	orPattern := Or(pattern1, pattern2, pattern3)

	assert.NotNil(t, orPattern, "OR pattern should be properly constructed")

	// Test NOT composition
	notPattern := Not(pattern1)

	assert.NotNil(t, notPattern, "NOT pattern should be properly constructed")

	// Test complex composition
	complexPattern := And(
		NodeKind("tag"),
		Or(
			HasChild(NodeText("if")),
			HasChild(NodeText("for")),
		),
		Not(HasChild(NodeText("raw"))),
	)

	assert.NotNil(t, complexPattern, "Complex pattern should be properly constructed")
}

// Common pattern library test
func TestPatternsLibrary(t *testing.T) {
	// PHP patterns
	phpMethodCallPattern := PHPMethodCallPattern("renderStorefront")
	assert.NotNil(t, phpMethodCallPattern, "PHP method call pattern should be constructed")

	// XML patterns
	xmlServicePattern := XMLServicePattern
	assert.NotNil(t, xmlServicePattern, "XML service pattern should be constructed")

	xmlServiceWithIdPattern := XMLServiceWithIdPattern("myService")
	assert.NotNil(t, xmlServiceWithIdPattern, "XML service with ID pattern should be constructed")

	// Twig patterns
	twigBlockPattern := TwigBlockPattern
	assert.NotNil(t, twigBlockPattern, "Twig block pattern should be constructed")

	twigExtendsPattern := TwigExtendsPattern
	assert.NotNil(t, twigExtendsPattern, "Twig extends pattern should be constructed")
}
