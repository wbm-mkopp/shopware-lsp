package symfony

import (
	"slices"
	"strings"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func isServiceIdContext(node *tree_sitter.Node, docText string) bool {
	if node.Kind() == "AttValue" && node.Parent() != nil && node.Parent().Kind() == "Attribute" {
		attrNode := node.Parent()

		nameNode := treesitterhelper.GetFirstNodeOfKind(attrNode, "Name")
		if nameNode == nil {
			return false
		}

		attrName := nameNode.Utf8Text([]byte(docText))

		if attrName != "id" && attrName != "class" {
			return false
		}

		tagName := treesitterhelper.GetFirstNodeOfKind(attrNode.Parent(), "Name")

		if tagName == nil {
			return false
		}

		tagNameText := tagName.Utf8Text([]byte(docText))
		return tagNameText == "service"
	}

	return false
}

func isArgumentServiceContext(node *tree_sitter.Node, docText string) bool {
	if node.Kind() == "AttValue" && node.Parent() != nil && node.Parent().Kind() == "Attribute" {
		attrNode := node.Parent()

		// Get the attribute name
		nameNode := treesitterhelper.GetFirstNodeOfKind(attrNode, "Name")
		if nameNode == nil {
			return false
		}

		attrName := nameNode.Utf8Text([]byte(docText))
		if attrName != "id" {
			return false
		}

		// Get the parent element
		parentElement := attrNode.Parent()
		if parentElement == nil {
			return false
		}

		// Check if the parent element has a type="service" attribute
		attrValues := treesitterhelper.GetXmlAttributeValues(parentElement, docText)
		if attrValues == nil || attrValues["type"] != "service" {
			return false
		}

		// Check if the parent element is an argument element
		elementNameNode := treesitterhelper.GetFirstNodeOfKind(parentElement, "Name")
		if elementNameNode == nil {
			return false
		}

		elementName := elementNameNode.Utf8Text([]byte(docText))
		return elementName == "argument"
	}

	return false
}

var possibleTaggedTypes = []string{"tagged_iterator", "tagged_locator", "tagged"}

func isArgumentTagContext(node *tree_sitter.Node, docText string) bool {
	if node.Kind() == "AttValue" && node.Parent() != nil && node.Parent().Kind() == "Attribute" {
		attrNode := node.Parent()

		// Get the attribute name
		nameNode := treesitterhelper.GetFirstNodeOfKind(attrNode, "Name")
		if nameNode == nil {
			return false
		}

		attrName := nameNode.Utf8Text([]byte(docText))
		if attrName != "tag" {
			return false
		}

		// Get the parent element
		parentElement := attrNode.Parent()
		if parentElement == nil {
			return false
		}

		// Check if the parent element has a type="tagged_iterator" attribute
		attrValues := treesitterhelper.GetXmlAttributeValues(parentElement, docText)
		if attrValues == nil || !slices.Contains(possibleTaggedTypes, attrValues["type"]) {
			return false
		}

		// Check if the parent element is an argument element
		elementNameNode := treesitterhelper.GetFirstNodeOfKind(parentElement, "Name")
		if elementNameNode == nil {
			return false
		}

		elementName := elementNameNode.Utf8Text([]byte(docText))
		return elementName == "argument"
	}

	return false
}

func getCurrentAttributeValue(node *tree_sitter.Node, docText string) string {
	return strings.Trim(node.Utf8Text([]byte(docText)), "\"")
}

func getParentServiceId(node *tree_sitter.Node, docText string) string {
	argumentNode := node.Parent().Parent()

	if argumentNode == nil {
		return ""
	}

	serviceNode := argumentNode.Parent().Parent().Parent()

	if serviceNode == nil {
		return ""
	}

	startTag := serviceNode.NamedChild(0)

	if startTag == nil {
		return ""
	}

	elementNameNode := treesitterhelper.GetFirstNodeOfKind(startTag, "Name")
	if elementNameNode == nil {
		return ""
	}

	if elementNameNode.Utf8Text([]byte(docText)) != "service" {
		return ""
	}

	attrValues := treesitterhelper.GetXmlAttributeValues(startTag, docText)
	if attrValues == nil || attrValues["id"] == "" {
		return ""
	}

	return attrValues["id"]
}
