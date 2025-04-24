package treesitterhelper

import tree_sitter "github.com/tree-sitter/go-tree-sitter"

func IsPHPRenderStorefrontCall(node *tree_sitter.Node, content []byte) bool {
	if node.Kind() != "string_content" {
		return false
	}

	methodCall := node.Parent().Parent().Parent().Parent()

	if methodCall.Kind() != "member_call_expression" {
		return false
	}

	nameNode := GetFirstNodeOfKind(methodCall, "name")

	if nameNode == nil {
		return false
	}

	if string(nameNode.Utf8Text(content)) != "renderStorefront" {
		return false
	}

	return true
}

func IsPHPRenderStorefrontCallEdit(node *tree_sitter.Node, content []byte) bool {
	if node.Kind() != "string" {
		return false
	}

	methodCall := node.Parent().Parent().Parent()

	if methodCall.Kind() != "member_call_expression" {
		return false
	}

	nameNode := GetFirstNodeOfKind(methodCall, "name")

	if nameNode == nil {
		return false
	}

	if string(nameNode.Utf8Text(content)) != "renderStorefront" {
		return false
	}

	return true
}
