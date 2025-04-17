package twig

import (
	"errors"

	tree_sitter_twig "github.com/kaermorchen/tree-sitter-twig/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type TwigFile struct {
	Path string
	// Relative Path, used inside of Twig
	RelPath string
	Blocks  map[string]TwigBlock
}

type TwigBlock struct {
	Name string
	Line int
}

func ParseTwig(content []byte, filePath string) (*TwigFile, error) {
	parser := tree_sitter.NewParser()

	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())); err != nil {
		return nil, err
	}

	tree := parser.Parse(content, nil)
	if tree == nil {
		return nil, errors.New("failed to parse Twig")
	}
	defer tree.Close()

	rootNode := tree.RootNode()
	if rootNode == nil {
		return nil, errors.New("failed to get root node")
	}

	file := &TwigFile{
		Path:   filePath,
		Blocks: make(map[string]TwigBlock),
	}

	var cursor = rootNode.Walk()
	defer cursor.Close()

	if cursor.GotoFirstChild() {
		for {
			node := cursor.Node()

			if node.Kind() == "block" {
				for i := 0; i < int(node.NamedChildCount()); i++ {
					child := node.NamedChild(uint(i))
					if child.Kind() == "identifier" {
						blockName := string(child.Utf8Text(content))
						file.Blocks[blockName] = TwigBlock{
							Name: blockName,
							Line: int(child.Range().StartPoint.Row) + 1,
						}
						break
					}
				}
			}

			if !cursor.GotoNextSibling() {
				break
			}
		}
	}

	return file, nil
}
