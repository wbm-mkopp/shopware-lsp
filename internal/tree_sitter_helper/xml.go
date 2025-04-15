package treesitterhelper

import (
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func GetFirstNodeOfKind(node *tree_sitter.Node, kind string) *tree_sitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(uint(i))
		if child.Kind() == kind {
			return child
		}
	}
	return nil
}

func GetXmlAttributeValues(node *tree_sitter.Node, documentText string) map[string]string {
	result := make(map[string]string)

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(uint(i))
		if child.Kind() == "Attribute" {
			nameNode := GetFirstNodeOfKind(child, "Name")
			valueNode := GetFirstNodeOfKind(child, "AttValue")

			if nameNode != nil && valueNode != nil {
				name := nameNode.Utf8Text([]byte(documentText))
				value := valueNode.Utf8Text([]byte(documentText))
				result[name] = value
			}
		}
	}

	return result
}
