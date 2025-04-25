package treesitterhelper

import (
	"slices"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func SymfonyServiceIsServiceId(node *tree_sitter.Node, docText []byte) bool {
	if node.Kind() == "AttValue" && node.Parent() != nil && node.Parent().Kind() == "Attribute" {
		attrNode := node.Parent()

		nameNode := GetFirstNodeOfKind(attrNode, "Name")
		if nameNode == nil {
			return false
		}

		attrName := nameNode.Utf8Text([]byte(docText))

		if attrName != "id" && attrName != "class" {
			return false
		}

		tagName := GetFirstNodeOfKind(attrNode.Parent(), "Name")

		if tagName == nil {
			return false
		}

		tagNameText := tagName.Utf8Text(docText)
		return tagNameText == "service"
	}

	return false
}

func SymfonyServiceIsServiceTag(node *tree_sitter.Node, docText []byte) bool {
	if node.Kind() == "AttValue" && node.Parent() != nil && node.Parent().Kind() == "Attribute" {
		attrNode := node.Parent()

		// Get the attribute name
		nameNode := GetFirstNodeOfKind(attrNode, "Name")
		if nameNode == nil {
			return false
		}

		attrName := nameNode.Utf8Text(docText)
		if attrName != "id" {
			return false
		}

		// Get the parent element
		parentElement := attrNode.Parent()
		if parentElement == nil {
			return false
		}

		// Check if the parent element has a type="service" attribute
		attrValues := GetXmlAttributeValues(parentElement, docText)
		if attrValues == nil || attrValues["type"] != "service" {
			return false
		}

		// Check if the parent element is an argument element
		elementNameNode := GetFirstNodeOfKind(parentElement, "Name")
		if elementNameNode == nil {
			return false
		}

		elementName := elementNameNode.Utf8Text(docText)
		return elementName == "argument"
	}

	return false
}

var possibleTaggedTypes = []string{"tagged_iterator", "tagged_locator", "tagged"}

func SymfonyServiceIsArgumentTag(node *tree_sitter.Node, docText []byte) bool {
	if node.Kind() == "AttValue" && node.Parent() != nil && node.Parent().Kind() == "Attribute" {
		attrNode := node.Parent()

		// Get the attribute name
		nameNode := GetFirstNodeOfKind(attrNode, "Name")
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
		attrValues := GetXmlAttributeValues(parentElement, docText)
		if attrValues == nil || !slices.Contains(possibleTaggedTypes, attrValues["type"]) {
			return false
		}

		// Check if the parent element is an argument element
		elementNameNode := GetFirstNodeOfKind(parentElement, "Name")
		if elementNameNode == nil {
			return false
		}

		elementName := elementNameNode.Utf8Text([]byte(docText))
		return elementName == "argument"
	}

	return false
}

func GetNodeText(node *tree_sitter.Node, docText []byte) string {
	return strings.Trim(strings.Trim(node.Utf8Text(docText), "\""), "'")
}

// SymfonyServiceIsParameterReference checks if the node is inside a parameter reference (like %parameter.name%)
func SymfonyServiceIsParameterReference(node *tree_sitter.Node, docText []byte) bool {
	if node.Kind() == "ETag" {
		parentNode := node.Parent()

		startTag := GetFirstNodeOfKind(parentNode, "STag")
		if startTag == nil {
			return false
		}

		attributeName := GetFirstNodeOfKind(startTag, "Name")

		if attributeName == nil {
			return false
		}

		if attributeName.Utf8Text([]byte(docText)) != "argument" {
			return false
		}

		contentTag := GetFirstNodeOfKind(parentNode, "content")

		if contentTag == nil {
			return false
		}

		charTag := GetFirstNodeOfKind(contentTag, "CharData")

		if charTag == nil {
			return false
		}

		nodeText := charTag.Utf8Text(docText)

		return strings.Contains(nodeText, "%") && (!strings.HasPrefix(nodeText, "%") || len(nodeText) == 1)
	}

	if node.Kind() == "CharData" {
		parentNode := node.Parent().Parent()

		startTag := GetFirstNodeOfKind(parentNode, "STag")
		if startTag == nil {
			return false
		}

		attributeName := GetFirstNodeOfKind(startTag, "Name")

		if attributeName == nil {
			return false
		}

		if attributeName.Utf8Text([]byte(docText)) != "argument" {
			return false
		}

		nodeText := node.Utf8Text([]byte(docText))

		return strings.Contains(nodeText, "%")
	}

	return false
}

// SymfonyGetCurrentServiceIdFromArgument returns the current service id from an argument node (service or tag)
// <service id="<get-this>">
// <argument id="<caret>">
func SymfonyGetCurrentServiceIdFromArgument(node *tree_sitter.Node, docText []byte) string {
	elementIdCapture := Capture("elementId", NodeKind("AttValue"))

	if !Ancestor(
		And(
			NodeKind("element"),
			HasChild(And(
				NodeKind("STag"),
				HasChild(And(
					NodeKind("Name"),
					NodeText("service"),
				)),
				HasChild(And(
					NodeKind("Attribute"),
					HasChild(And(
						NodeKind("Name"),
						NodeTextUnescaped("id"),
					)),
					HasChild(And(
						elementIdCapture,
					)),
				)),
			)),
		),
		5,
	).Matches(node, docText) {
		return ""
	}

	capturedNode := elementIdCapture.GetCapturedNode()
	if capturedNode == nil {
		return ""
	}

	return strings.Trim(string(capturedNode.Utf8Text(docText)), "\"")
}

// SymfonyServiceIsTagElement returns true if the node is a tag element
// <tag name="<caret>"/>
func SymfonyServiceIsTagElement(node *tree_sitter.Node, docText []byte) bool {
	if node.Kind() != "AttValue" {
		return false
	}

	nameNode := GetFirstNodeOfKind(node.Parent(), "Name")

	if nameNode == nil {
		return false
	}

	if nameNode.Utf8Text([]byte(docText)) != "name" {
		return false
	}

	elementNameNode := GetFirstNodeOfKind(node.Parent().Parent(), "Name")

	if elementNameNode == nil {
		return false
	}

	if elementNameNode.Utf8Text(docText) != "tag" {
		return false
	}

	return true
}
