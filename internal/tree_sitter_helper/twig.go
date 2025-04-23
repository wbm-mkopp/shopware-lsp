package treesitterhelper

import (
	"slices"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func IsTwigTag(node *tree_sitter.Node, docText []byte, tagNames ...string) bool {
	if node.Kind() != "string" {
		return false
	}

	parent := node.Parent()

	if parent.Kind() != "tag" {
		return false
	}

	nameNode := GetFirstNodeOfKind(parent, "keyword")

	if nameNode == nil {
		return false
	}

	if !slices.Contains(tagNames, string(nameNode.Utf8Text(docText))) {
		return false
	}

	return true
}
