package twig

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type TwigFile struct {
	// Name of the bundle
	BundleName string
	Path       string
	// Relative Path, used inside of Twig
	RelPath        string
	Blocks         map[string]TwigBlock
	ExtendsFile    string
	ExtendsTagLine int
}

type TwigBlock struct {
	Name string
	Line int
}

func ParseTwig(filePath string, node *tree_sitter.Node, content []byte) (*TwigFile, error) {
	file := &TwigFile{
		Path:       filePath,
		BundleName: getBundleNameByPath(filePath),
		RelPath:    convertToRelativePath(filePath),
		Blocks:     make(map[string]TwigBlock),
	}

	var cursor = node.Walk()
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

			if node.Kind() == "tag" {
				// Check if this is an extends tag by examining the tag text
				tagText := string(node.Utf8Text(content))
				isExtendsTag := false

				// Check if the tag contains "extends" or "sw_extends"
				if strings.Contains(tagText, "extends") || strings.Contains(tagText, "sw_extends") {
					isExtendsTag = true
				}

				// If it's an extends tag, look for the string parameter
				if isExtendsTag {
					for i := 0; i < int(node.NamedChildCount()); i++ {
						child := node.NamedChild(uint(i))

						if child.Kind() == "string" {
							file.ExtendsFile = strings.Trim(string(child.Utf8Text(content)), "\"")
							file.ExtendsTagLine = int(node.Range().StartPoint.Row) + 1
							break
						}
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
