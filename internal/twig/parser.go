package twig

import (
	"crypto/sha512"
	"fmt"
	"regexp"
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Regular expression to match Shopware block version comments: {# shopware-block: hash@version #}
var shopwareBlockCommentRegex = regexp.MustCompile(`\{#\s*shopware-block:\s*([a-f0-9]+)@([\w\.\-]+)\s*#\}`)

// calculateBlockHash generates a SHA-512 hash for the given block content
func calculateBlockHash(content string) string {
	hash := sha512.New()
	hash.Write([]byte(content))
	return fmt.Sprintf("%x", hash.Sum(nil))
}

// ParseVersionComment extracts version information from a Shopware block comment
func ParseVersionComment(comment string, line int) *TwigVersionComment {
	matches := shopwareBlockCommentRegex.FindStringSubmatch(comment)
	if len(matches) == 3 {
		return &TwigVersionComment{
			Hash:    matches[1],
			Version: matches[2],
			Line:    line,
		}
	}
	return nil
}

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

type TwigVersionComment struct {
	Hash    string
	Version string
	Line    int
}

type TwigBlockHash struct {
	Name         string
	RelativePath string
	AbsolutePath string
	Hash         string
	Text         string
}

type TwigBlock struct {
	Name string
	Line int
	Hash string
	Text string
	VersionComment *TwigVersionComment
}

// findBlocks recursively traverses the tree to find all blocks
func findBlocks(node *tree_sitter.Node, content []byte, file *TwigFile) {
	if node.Kind() == "block" {
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(uint(i))
			if child.Kind() == "identifier" {
				blockName := string(child.Utf8Text(content))
				
				// Extract the entire block content including the tag
				blockText := string(node.Utf8Text(content))
				blockHash := calculateBlockHash(blockText)
				
				// Look for a version comment before this block
				var versionComment *TwigVersionComment
				if prevSibling := findPreviousComment(node, content); prevSibling != nil {
					commentText := string(prevSibling.Utf8Text(content))
					versionComment = ParseVersionComment(commentText, int(prevSibling.Range().StartPoint.Row)+1)
				}
				
				file.Blocks[blockName] = TwigBlock{
					Name:           blockName,
					Line:           int(child.Range().StartPoint.Row) + 1,
					Hash:           blockHash,
					Text:           blockText,
					VersionComment: versionComment,
				}
				break
			}
		}
	}

	// Recursively process all named children
	for i := 0; i < int(node.NamedChildCount()); i++ {
		findBlocks(node.NamedChild(uint(i)), content, file)
	}
}

// findPreviousComment looks for a comment node before the given block node
func findPreviousComment(blockNode *tree_sitter.Node, content []byte) *tree_sitter.Node {
	parent := blockNode.Parent()
	if parent == nil {
		return nil
	}

	// Look for a comment in the previous siblings
	for i := 0; i < int(parent.NamedChildCount()); i++ {
		child := parent.NamedChild(uint(i))
		
		// Compare by position to identify the block node
		if child.Range().StartPoint.Row == blockNode.Range().StartPoint.Row &&
		   child.Range().StartPoint.Column == blockNode.Range().StartPoint.Column {
			// Found our block, now look at previous siblings
			for j := i - 1; j >= 0; j-- {
				prevSibling := parent.NamedChild(uint(j))
				if prevSibling.Kind() == "comment" {
					commentText := string(prevSibling.Utf8Text(content))
					if strings.Contains(commentText, "shopware-block:") {
						return prevSibling
					}
				}
			}
			break
		}
	}
	return nil
}

func ParseTwig(filePath string, node *tree_sitter.Node, content []byte) (*TwigFile, error) {
	file := &TwigFile{
		Path:       filePath,
		BundleName: getBundleNameByPath(filePath),
		RelPath:    ConvertToRelativePath(filePath),
		Blocks:     make(map[string]TwigBlock),
	}

	// Find all blocks recursively
	findBlocks(node, content, file)

	// Find extends tag
	var cursor = node.Walk()
	defer cursor.Close()

	if cursor.GotoFirstChild() {
		for {
			node := cursor.Node()

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
							file.ExtendsFile = strings.Trim(strings.Trim(string(child.Utf8Text(content)), "\""), "'")
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
