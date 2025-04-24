package treesitterhelper

import (
	"strings"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Common patterns library that can be reused
var (
	// PHP patterns
	PHPMethodCallPattern = func(methodName string) Pattern {
		return And(
			NodeKind("member_call_expression"),
			HasChild(And(
				NodeKind("name"),
				NodeText(methodName),
			)),
		)
	}

	PHPStringLiteralPattern = AnyNodeKind("string", "string_content")

	PHPRenderStorefrontCallPattern = And(
		PHPStringLiteralPattern,
		Ancestor(PHPMethodCallPattern("renderStorefront"), 4),
	)

	// XML patterns
	XMLServicePattern = And(
		NodeKind("element"),
		HasChild(And(
			NodeKind("tag_name"),
			NodeText("service"),
		)),
	)

	XMLServiceWithIdPattern = func(serviceId string) Pattern {
		return And(
			XMLServicePattern,
			HasChild(And(
				NodeKind("attribute"),
				HasChild(And(
					NodeKind("attribute_name"),
					NodeText("id"),
				)),
				HasChild(And(
					NodeKind("attribute_value"),
					NodeText(serviceId),
				)),
			)),
		)
	}

	// Twig patterns
	TwigBlockPattern = And(
		NodeKind("tag"),
		HasChild(And(
			NodeKind("tag_name"),
			NodeText("block"),
		)),
	)

	TwigExtendsPattern = And(
		NodeKind("tag"),
		HasChild(And(
			NodeKind("tag_name"),
			NodeText("extends"),
		)),
	)
	
	// Generic Twig tag pattern builder
	TwigTagPattern = func(tagNames ...string) Pattern {
		// Based on the tree structure, we need to check for nodes with specific kinds (extends, include, tag)
		// and then look for a keyword child with specific text
		tagKindPatterns := []Pattern{}
		
		// Add patterns for direct tag kinds (extends, include)
		for _, tagName := range tagNames {
			// Some tags are represented by their own node kind (extends, include)
			// Others are generic "tag" nodes with a keyword child
			if tagName == "extends" || tagName == "include" {
				tagKindPatterns = append(tagKindPatterns, NodeKind(tagName))
			} else {
				// For other tags like sw_extends, sw_include, block, etc.
				tagKindPatterns = append(tagKindPatterns,
					And(
						NodeKind("tag"),
						HasChild(And(
							NodeKind("keyword"),
							NodeText(tagName),
						)),
					),
				)
			}
		}
		
		return Or(tagKindPatterns...)
	}
	
	// Pattern for a string inside a specific Twig tag
	TwigStringInTagPattern = func(tagNames ...string) Pattern {
		return And(
			NodeKind("string"),
			FuncPattern(func(node *tree_sitter.Node, content []byte) bool {
				parent := node.Parent()
				if parent == nil {
					return false
				}
				
				// Check if parent is one of our tag kinds directly
				parentKind := parent.Kind()
				for _, tagName := range tagNames {
					if parentKind == tagName { // For direct kinds like "extends" or "include"
						return true
					}
				}
				
				// Check for generic "tag" with keyword
				if parentKind == "tag" {
					// Find the keyword child and check its text
					for i := 0; i < int(parent.ChildCount()); i++ {
						child := parent.Child(uint(i))
						if child != nil && child.Kind() == "keyword" {
							keywordText := string(child.Utf8Text(content))
							for _, tagName := range tagNames {
								if keywordText == tagName {
									return true
								}
							}
						}
					}
				}
				
				return false
			}),
		)
	}
)

// Pattern defines a pattern that can be matched against a tree-sitter node
type Pattern interface {
	Matches(node *tree_sitter.Node, content []byte) bool
}

// Create a pattern from a function
func FuncPattern(matchFunc func(node *tree_sitter.Node, content []byte) bool) Pattern {
	return &funcPattern{matchFunc: matchFunc}
}

type funcPattern struct {
	matchFunc func(node *tree_sitter.Node, content []byte) bool
}

func (p *funcPattern) Matches(node *tree_sitter.Node, content []byte) bool {
	return p.matchFunc(node, content)
}

// Chain multiple patterns using AND logic
func And(patterns ...Pattern) Pattern {
	return &andPattern{patterns: patterns}
}

type andPattern struct {
	patterns []Pattern
}

func (p *andPattern) Matches(node *tree_sitter.Node, content []byte) bool {
	for _, pattern := range p.patterns {
		if !pattern.Matches(node, content) {
			return false
		}
	}
	return true
}

// Chain multiple patterns using OR logic
func Or(patterns ...Pattern) Pattern {
	return &orPattern{patterns: patterns}
}

type orPattern struct {
	patterns []Pattern
}

func (p *orPattern) Matches(node *tree_sitter.Node, content []byte) bool {
	for _, pattern := range p.patterns {
		if pattern.Matches(node, content) {
			return true
		}
	}
	return false
}

// Negate a pattern
func Not(pattern Pattern) Pattern {
	return &notPattern{pattern: pattern}
}

type notPattern struct {
	pattern Pattern
}

func (p *notPattern) Matches(node *tree_sitter.Node, content []byte) bool {
	return !p.pattern.Matches(node, content)
}

// Match a node's kind
func NodeKind(kind string) Pattern {
	return &nodeKindPattern{kind: kind}
}

type nodeKindPattern struct {
	kind string
}

func (p *nodeKindPattern) Matches(node *tree_sitter.Node, content []byte) bool {
	return node.Kind() == p.kind
}

// Match any of the node kinds
func AnyNodeKind(kinds ...string) Pattern {
	return &anyNodeKindPattern{kinds: kinds}
}

type anyNodeKindPattern struct {
	kinds []string
}

func (p *anyNodeKindPattern) Matches(node *tree_sitter.Node, content []byte) bool {
	kind := node.Kind()
	for _, k := range p.kinds {
		if kind == k {
			return true
		}
	}
	return false
}

// Match a node's text content
func NodeText(text string) Pattern {
	return &nodeTextPattern{text: text}
}

type nodeTextPattern struct {
	text string
}

func (p *nodeTextPattern) Matches(node *tree_sitter.Node, content []byte) bool {
	return string(node.Utf8Text(content)) == p.text
}

// Match a node's text content with contains
func NodeTextContains(substring string) Pattern {
	return &nodeTextContainsPattern{substring: substring}
}

type nodeTextContainsPattern struct {
	substring string
}

func (p *nodeTextContainsPattern) Matches(node *tree_sitter.Node, content []byte) bool {
	return strings.Contains(string(node.Utf8Text(content)), p.substring)
}

// Match a parent node at a specific level
func ParentOfKind(kind string, level int) Pattern {
	return &parentOfKindPattern{kind: kind, level: level}
}

type parentOfKindPattern struct {
	kind  string
	level int
}

func (p *parentOfKindPattern) Matches(node *tree_sitter.Node, content []byte) bool {
	parent := node
	for i := 0; i < p.level && parent != nil; i++ {
		parent = parent.Parent()
	}
	return parent != nil && parent.Kind() == p.kind
}

// Match a node that has a child of a specific kind
func HasChildOfKind(kind string) Pattern {
	return &hasChildOfKindPattern{kind: kind}
}

type hasChildOfKindPattern struct {
	kind string
}

func (p *hasChildOfKindPattern) Matches(node *tree_sitter.Node, content []byte) bool {
	return GetFirstNodeOfKind(node, p.kind) != nil
}

// Match a child node with a specific pattern
func HasChild(pattern Pattern) Pattern {
	return &hasChildPattern{pattern: pattern}
}

type hasChildPattern struct {
	pattern Pattern
}

func (p *hasChildPattern) Matches(node *tree_sitter.Node, content []byte) bool {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(uint(i))
		if p.pattern.Matches(child, content) {
			return true
		}
	}
	return false
}

// Match a specific named child with a pattern
func NamedChild(index uint, pattern Pattern) Pattern {
	return &namedChildPattern{index: index, pattern: pattern}
}

type namedChildPattern struct {
	index   uint
	pattern Pattern
}

func (p *namedChildPattern) Matches(node *tree_sitter.Node, content []byte) bool {
	if int(p.index) >= int(node.NamedChildCount()) {
		return false
	}
	child := node.NamedChild(p.index)
	return p.pattern.Matches(child, content)
}

// Match a sequence of patterns against named children
func ChildSequence(patterns ...Pattern) Pattern {
	return &childSequencePattern{patterns: patterns}
}

type childSequencePattern struct {
	patterns []Pattern
}

func (p *childSequencePattern) Matches(node *tree_sitter.Node, content []byte) bool {
	if int(node.NamedChildCount()) < len(p.patterns) {
		return false
	}

	for i, pattern := range p.patterns {
		child := node.NamedChild(uint(i))
		if !pattern.Matches(child, content) {
			return false
		}
	}
	return true
}

// Match an ancestor node that matches the pattern
func Ancestor(pattern Pattern, maxDepth int) Pattern {
	return &ancestorPattern{pattern: pattern, maxDepth: maxDepth}
}

type ancestorPattern struct {
	pattern  Pattern
	maxDepth int
}

func (p *ancestorPattern) Matches(node *tree_sitter.Node, content []byte) bool {
	current := node.Parent()
	depth := 0

	for current != nil && depth < p.maxDepth {
		if p.pattern.Matches(current, content) {
			return true
		}
		current = current.Parent()
		depth++
	}
	return false
}

// Position based patterns
func FirstChild(pattern Pattern) Pattern {
	return NamedChild(0, pattern)
}

// Capture patterns allow retrieving nodes that matched
type CapturePattern interface {
	Pattern
	GetCapturedNode() *tree_sitter.Node
}

// Create a capture that can be reused in patterns
func Capture(name string, pattern Pattern) CapturePattern {
	return &capturePattern{name: name, pattern: pattern}
}

type capturePattern struct {
	name    string
	pattern Pattern
	result  *tree_sitter.Node
}

func (p *capturePattern) Matches(node *tree_sitter.Node, content []byte) bool {
	if p.pattern.Matches(node, content) {
		p.result = node
		return true
	}
	return false
}

func (p *capturePattern) GetCapturedNode() *tree_sitter.Node {
	return p.result
}

// Utility function to match a pattern and return the first matching node
func FindFirst(root *tree_sitter.Node, pattern Pattern, content []byte) *tree_sitter.Node {
	// Simple traversal implementation 
	if pattern.Matches(root, content) {
		return root
	}

	for i := 0; i < int(root.NamedChildCount()); i++ {
		child := root.NamedChild(uint(i))
		if result := FindFirst(child, pattern, content); result != nil {
			return result
		}
	}
	
	return nil
}

// Utility function to find all nodes matching a pattern
func FindAll(root *tree_sitter.Node, pattern Pattern, content []byte) []*tree_sitter.Node {
	var results []*tree_sitter.Node
	
	var visit func(node *tree_sitter.Node)
	visit = func(node *tree_sitter.Node) {
		if pattern.Matches(node, content) {
			results = append(results, node)
		}
		
		for i := 0; i < int(node.NamedChildCount()); i++ {
			visit(node.NamedChild(uint(i)))
		}
	}
	
	visit(root)
	return results
}

// Helper functions for common pattern use cases

// Check if a node is an XML service with a specific ID
func IsXMLServiceWithID(node *tree_sitter.Node, content []byte, serviceId string) bool {
	return XMLServiceWithIdPattern(serviceId).Matches(node, content)
}

// Find all block nodes in a Twig document
func FindAllTwigBlocks(root *tree_sitter.Node, content []byte) []*tree_sitter.Node {
	return FindAll(root, TwigBlockPattern, content)
}

// Get the name of a Twig block
func GetTwigBlockName(blockNode *tree_sitter.Node, content []byte) string {
	nameCapture := Capture("name", NodeKind("string"))
	
	blockWithNamePattern := And(
		TwigBlockPattern,
		HasChild(nameCapture),
	)
	
	if blockWithNamePattern.Matches(blockNode, content) {
		nameNode := nameCapture.GetCapturedNode()
		if nameNode != nil {
			return string(nameNode.Utf8Text(content))
		}
	}
	
	return ""
}