package treesitterhelper

import (
	"log"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func PrintAllNodes(node *tree_sitter.Node, indent string) {
	log.Printf("%s%s", indent, node.Kind())

	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(uint(i))
		PrintAllNodes(child, indent+"  ")
	}
}
