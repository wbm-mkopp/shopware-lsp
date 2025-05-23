package treesitterhelper

import (
	"log"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func PrintAllNodes(node *tree_sitter.Node, content []byte, indent string) {
	log.Printf("%s%s (%s)", indent, node.Kind(), node.Utf8Text(content))

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(uint(i))
		PrintAllNodes(child, content, indent+"  ")
	}
}
