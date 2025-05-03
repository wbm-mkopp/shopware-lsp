package php

import (
	"fmt"
	"os"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

// DebugAST parses a PHP file and prints the AST structure
func DebugAST(filePath string) {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Printf("Error reading file: %v\n", err)
		return
	}

	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())); err != nil {
		fmt.Printf("Error setting language: %v\n", err)
		return
	}

	defer parser.Close()

	tree := parser.Parse(fileContent, nil)
	rootNode := tree.RootNode()

	printNodeStructure(rootNode, fileContent, 0)
}

// printNodeStructure recursively prints the node structure
func printNodeStructure(node *tree_sitter.Node, fileContent []byte, depth int) {
	if node == nil {
		return
	}

	indent := ""
	for i := 0; i < depth; i++ {
		indent += "  "
	}

	nodeText := ""
	if node.NamedChildCount() == 0 {
		nodeText = string(node.Utf8Text(fileContent))
	}

	fmt.Printf("%sNode: %s, Text: %s\n", indent, node.Kind(), nodeText)

	// Print property declarations with more detail
	if node.Kind() == "property_declaration" || node.Kind() == "property_element" {
		fmt.Printf("%s  PROPERTY DETAIL - ChildCount: %d\n", indent, node.NamedChildCount())
		for i := uint(0); i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child != nil {
				childText := string(child.Utf8Text(fileContent))
				fmt.Printf("%s    Child %d: Kind=%s, Text=%s\n", indent, i, child.Kind(), childText)
			}
		}
	}

	// Print property promotions with more detail
	if node.Kind() == "property_promotion_parameter" {
		fmt.Printf("%s  PROPERTY PROMOTION DETAIL - ChildCount: %d\n", indent, node.NamedChildCount())
		for i := uint(0); i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child != nil {
				childText := string(child.Utf8Text(fileContent))
				fmt.Printf("%s    Child %d: Kind=%s, Text=%s\n", indent, i, child.Kind(), childText)
			}
		}
	}

	// Recursively print child nodes
	for i := uint(0); i < node.NamedChildCount(); i++ {
		printNodeStructure(node.NamedChild(i), fileContent, depth+1)
	}
}
