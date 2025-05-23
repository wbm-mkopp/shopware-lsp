package treesitterhelper

import (
	"testing"

	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	"github.com/stretchr/testify/assert"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Helper function to debug tree nodes - commented out to avoid test output
/*
func debugNode(node *tree_sitter.Node, content []byte, depth int) {
	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}

	fmt.Printf("%sKind: %s", indent, node.Kind())

	if node.NamedChildCount() == 0 {
		fmt.Printf(" Text: %q", string(node.Utf8Text(content)))
	}
	fmt.Println()

	// Print all children
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(uint(i))
		if child != nil {
			debugNode(child, content, depth+1)
		}
	}
}
*/

func TestTwigTagStructure(t *testing.T) {
	parser := tree_sitter.NewParser()

	// Skip the test if we don't have the Twig language available
	err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language()))
	if err != nil {
		t.Skip("Skipping test that requires tree-sitter-twig")
		return
	}

	twigCode := []byte(`{% extends "base.html.twig" %}
{% include "components/header.twig" %}
{% sw_extends "storefront/base.html.twig" %}`)

	tree := parser.Parse(twigCode, nil)
	defer tree.Close()

	// Find string nodes in different tag types (extends, include, tag/sw_extends)
	var extendsString, includeString, swExtendsString *tree_sitter.Node

	// Manually find the nodes we want to test
	for i := 0; i < int(tree.RootNode().NamedChildCount()); i++ {
		node := tree.RootNode().NamedChild(uint(i))

		switch node.Kind() {
		case "extends":
			// Find string child in extends
			for j := 0; j < int(node.ChildCount()); j++ {
				child := node.Child(uint(j))
				if child.Kind() == "string" {
					extendsString = child
					break
				}
			}
		case "include":
			// Find string child in include
			for j := 0; j < int(node.ChildCount()); j++ {
				child := node.Child(uint(j))
				if child.Kind() == "string" {
					includeString = child
					break
				}
			}
		case "tag":
			// Check if this is the sw_extends tag
			for j := 0; j < int(node.ChildCount()); j++ {
				child := node.Child(uint(j))
				if child.Kind() == "keyword" && string(child.Utf8Text(twigCode)) == "sw_extends" {
					// Found the sw_extends tag, now find its string
					for k := 0; k < int(node.ChildCount()); k++ {
						stringChild := node.Child(uint(k))
						if stringChild.Kind() == "string" {
							swExtendsString = stringChild
							break
						}
					}
					break
				}
			}
		}
	}

	// Test our string nodes
	assert.NotNil(t, extendsString, "Should find string node in extends tag")
	assert.NotNil(t, includeString, "Should find string node in include tag")
	assert.NotNil(t, swExtendsString, "Should find string node in sw_extends tag")

	// Test our patterns
	// For extends tag
	if extendsString != nil {
		assert.True(t, TwigStringInTagPattern("extends").Matches(extendsString, twigCode),
			"String in extends tag should match extends")
		assert.False(t, TwigStringInTagPattern("include").Matches(extendsString, twigCode),
			"String in extends tag should not match include")
		assert.False(t, TwigStringInTagPattern("sw_extends").Matches(extendsString, twigCode),
			"String in extends tag should not match sw_extends")
	}

	// For include tag
	if includeString != nil {
		assert.True(t, TwigStringInTagPattern("include").Matches(includeString, twigCode),
			"String in include tag should match include")
		assert.False(t, TwigStringInTagPattern("extends").Matches(includeString, twigCode),
			"String in include tag should not match extends")
		assert.False(t, TwigStringInTagPattern("sw_extends").Matches(includeString, twigCode),
			"String in include tag should not match sw_extends")
	}

	// For sw_extends tag
	if swExtendsString != nil {
		assert.True(t, TwigStringInTagPattern("sw_extends").Matches(swExtendsString, twigCode),
			"String in sw_extends tag should match sw_extends")
		assert.False(t, TwigStringInTagPattern("extends").Matches(swExtendsString, twigCode),
			"String in sw_extends tag should not match extends")
		assert.False(t, TwigStringInTagPattern("include").Matches(swExtendsString, twigCode),
			"String in sw_extends tag should not match include")
	}

	// Test with multiple tag names
	if extendsString != nil && includeString != nil {
		assert.True(t, TwigStringInTagPattern("extends", "include").Matches(extendsString, twigCode),
			"String in extends tag should match [extends, include]")
		assert.True(t, TwigStringInTagPattern("extends", "include").Matches(includeString, twigCode),
			"String in include tag should match [extends, include]")
	}

	// Test complex combination
	if swExtendsString != nil {
		assert.True(t, TwigStringInTagPattern("extends", "sw_extends").Matches(swExtendsString, twigCode),
			"String in sw_extends tag should match [extends, sw_extends]")
	}
}

func TestExtractSwIconObjectToMap(t *testing.T) {
	parser := tree_sitter.NewParser()

	// Skip the test if we don't have the Twig language available
	err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language()))
	if err != nil {
		t.Skip("Skipping test that requires tree-sitter-twig")
		return
	}

	twigCode := []byte(`{% sw_icon 'file' style {'size': 'xs', 'color': 'red'} %}`)

	tree := parser.Parse(twigCode, nil)
	defer tree.Close()

	// Find the tag node
	var tagNode *tree_sitter.Node
	for i := 0; i < int(tree.RootNode().ChildCount()); i++ {
		node := tree.RootNode().Child(uint(i))
		if node != nil && node.Kind() == "tag" {
			tagNode = node
			break
		}
	}

	assert.NotNil(t, tagNode, "Should find tag node")

	if tagNode != nil {
		// Extract the object to map
		result := ExtractSwIconObjectToMap(tagNode, twigCode)

		// Verify the extracted map
		assert.Equal(t, "xs", result["size"], "Should extract size value")
		assert.Equal(t, "red", result["color"], "Should extract color value")
		assert.Len(t, result, 2, "Should have exactly 2 pairs")
	}
}
