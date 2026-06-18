package twig

import (
	"regexp"
	"strings"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

var twigBlockOpenPattern = regexp.MustCompile(`\{%-?\s*block\s+([a-zA-Z0-9_]+)`)

type blockLineRange struct {
	name      string
	startLine int
	endLine   int
}

// ResolveBlockNameForExtend returns the Twig block to extend at the cursor.
func ResolveBlockNameForExtend(node *tree_sitter.Node, content []byte, line int) (string, bool) {
	if name, ok := BlockNameAtNode(node, content); ok {
		return name, true
	}

	if len(content) > 0 && line >= 0 {
		return BlockNameAtCursor(content, line)
	}

	return "", false
}

// BlockNameAtNode returns the Twig block name at the cursor when extending blocks
// from vendor templates. Handles block tags parsed as ERROR nodes in partials.
func BlockNameAtNode(node *tree_sitter.Node, content []byte) (string, bool) {
	if node == nil {
		return "", false
	}

	if name, ok := blockNameFromIdentifier(node, content); ok {
		return name, true
	}

	for current := node.Parent(); current != nil; current = current.Parent() {
		if current.Kind() != "block" {
			continue
		}

		if name := blockNameFromBlockNode(current, content); name != "" {
			return name, true
		}
	}

	return "", false
}

// BlockNameAtCursor returns the innermost block name containing the given line.
func BlockNameAtCursor(content []byte, line int) (string, bool) {
	ranges := findBlockLineRanges(content)

	var best *blockLineRange
	bestSpan := int(^uint(0) >> 1)

	for _, blockRange := range ranges {
		if line < blockRange.startLine || line > blockRange.endLine {
			continue
		}

		span := blockRange.endLine - blockRange.startLine
		if span < bestSpan {
			bestSpan = span
			copyRange := blockRange
			best = &copyRange
		}
	}

	if best == nil {
		return "", false
	}

	return best.name, true
}

func findBlockLineRanges(content []byte) []blockLineRange {
	lines := strings.Split(string(content), "\n")
	type stackEntry struct {
		name      string
		startLine int
	}

	var stack []stackEntry
	var ranges []blockLineRange

	for lineIndex, line := range lines {
		if match := twigBlockOpenPattern.FindStringSubmatch(line); match != nil {
			stack = append(stack, stackEntry{name: match[1], startLine: lineIndex})
		}

		if strings.Contains(line, "endblock") && len(stack) > 0 {
			entry := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			ranges = append(ranges, blockLineRange{
				name:      entry.name,
				startLine: entry.startLine,
				endLine:   lineIndex,
			})
		}
	}

	return ranges
}

func blockNameFromIdentifier(node *tree_sitter.Node, content []byte) (string, bool) {
	if node.Kind() != "identifier" {
		return "", false
	}

	if isBlockNameIdentifier(node, content) {
		return string(node.Utf8Text(content)), true
	}

	return "", false
}

func isBlockNameIdentifier(node *tree_sitter.Node, content []byte) bool {
	if treesitterhelper.And(
		treesitterhelper.NodeKind("identifier"),
		treesitterhelper.Ancestor(treesitterhelper.NodeKind("block"), 1),
	).Matches(node, content) {
		return true
	}

	parent := node.Parent()
	if parent == nil || parent.Kind() != "ERROR" {
		return false
	}

	return isBlockTagLinePrefix(linePrefixBeforeNode(content, node))
}

func blockNameFromBlockNode(blockNode *tree_sitter.Node, content []byte) string {
	for i := uint(0); i < blockNode.NamedChildCount(); i++ {
		child := blockNode.NamedChild(i)
		if child.Kind() == "identifier" {
			return string(child.Utf8Text(content))
		}
	}

	return ""
}

func linePrefixBeforeNode(content []byte, node *tree_sitter.Node) string {
	start := int(node.StartByte())
	if start > len(content) {
		start = len(content)
	}

	lineStart := start
	for lineStart > 0 && content[lineStart-1] != '\n' {
		lineStart--
	}

	return string(content[lineStart:start])
}

func isBlockTagLinePrefix(prefix string) bool {
	idx := strings.Index(prefix, "{%")
	if idx == -1 {
		return false
	}

	tag := strings.TrimSpace(prefix[idx:])
	return strings.HasPrefix(tag, "{% block") || strings.HasPrefix(tag, "{%- block")
}

func blockNameDeclaredInContent(content []byte, blockName string) bool {
	pattern := regexp.MustCompile(`\{%-?\s*block\s+` + regexp.QuoteMeta(blockName) + `(?:\s|%|\})`)
	return pattern.Match(content)
}
