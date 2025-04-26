package feature

import (
	"fmt"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type Feature struct {
	Name     string
	File     string
	Line     int
}

func ParseFeatureFile(root *tree_sitter.Node, document []byte, filePath string) ([]Feature, error) {
	features := []Feature{}

	// Directly traverse the tree to find block_mapping_pair with key "name"
	traverseForFeatures(root, document, filePath, &features)

	if len(features) == 0 {
		return features, fmt.Errorf("could not find flags node in file: %s", filePath)
	}

	return features, nil
}

func traverseForFeatures(node *tree_sitter.Node, document []byte, filePath string, features *[]Feature) {
	// For debugging
	if node.Kind() == "block_mapping_pair" {
		keyNode := node.ChildByFieldName("key")
		if keyNode != nil {
			for i := uint(0); i < keyNode.NamedChildCount(); i++ {
				child := keyNode.NamedChild(i)
				if child.Kind() == "plain_scalar" {
					for j := uint(0); j < child.NamedChildCount(); j++ {
						textNode := child.NamedChild(j)
						if textNode.Kind() == "string_scalar" {
							keyText := string(textNode.Utf8Text(document))
							
							// If this is a "name" key
							if keyText == "name" {
								// Look for the value
								valueNode := node.ChildByFieldName("value")
								if valueNode != nil {
									for k := uint(0); k < valueNode.NamedChildCount(); k++ {
										valChild := valueNode.NamedChild(k)
										if valChild.Kind() == "plain_scalar" {
											for l := uint(0); l < valChild.NamedChildCount(); l++ {
												nameNode := valChild.NamedChild(l)
												if nameNode.Kind() == "string_scalar" {
													nameText := string(nameNode.Utf8Text(document))
													*features = append(*features, Feature{
														Name: nameText,
														File: filePath,
														Line: int(nameNode.Range().StartPoint.Row) + 1,
													})
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}

	// Recursively search all children
	for i := uint(0); i < node.NamedChildCount(); i++ {
		traverseForFeatures(node.NamedChild(i), document, filePath, features)
	}
}

