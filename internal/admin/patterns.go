package admin

import (
	"strings"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// =============================================================================
// Pattern Matchers (defined first as they're used by other patterns)
// =============================================================================

// nodeTextPrefixPattern matches nodes whose text starts with a given prefix
type nodeTextPrefixPattern struct {
	prefix string
}

func (p nodeTextPrefixPattern) Matches(node *tree_sitter.Node, content []byte) bool {
	if node == nil {
		return false
	}
	text := string(node.Utf8Text(content))
	return strings.HasPrefix(text, p.prefix)
}

// NodeTextPrefix creates a pattern that matches nodes whose text starts with prefix
func NodeTextPrefix(prefix string) treesitterhelper.Pattern {
	return nodeTextPrefixPattern{prefix: prefix}
}

// =============================================================================
// JavaScript/TypeScript Patterns for Admin Component Registration
// =============================================================================

// NOTE: JSComponentCallPattern is defined in indexer.go as it's used during indexing
// It matches: Component.register('name', ...) | Component.extend('name', 'parent', ...) | Shopware.Component.*

// JSComponentExtendCallPattern matches only Component.extend() calls (not register)
// Used for diagnostics to check if parent component exists
//
// Example: Component.extend('my-component', 'sw-parent', ...)
var JSComponentExtendCallPattern = treesitterhelper.And(
	treesitterhelper.NodeKind("call_expression"),
	treesitterhelper.HasChild(
		treesitterhelper.And(
			treesitterhelper.NodeKind("member_expression"),
			treesitterhelper.Or(
				treesitterhelper.NodeText("Shopware.Component.extend"),
				treesitterhelper.NodeText("Component.extend"),
			),
		),
	),
)

// JSStringInComponentExtendPattern matches a string node inside Component.extend() arguments
// Used for completion and go-to-definition when cursor is on component name string
//
// Example: Component.extend('my-component', '<caret>', ...)
//
//	^^^^^^^^^ matches this string
var JSStringInComponentExtendPattern = treesitterhelper.And(
	treesitterhelper.AnyNodeKind("string", "string_fragment"),
	treesitterhelper.Ancestor(JSComponentExtendCallPattern, 5),
)

// =============================================================================
// Twig/HTML Patterns for Admin Templates
// =============================================================================

// TwigHTMLStartTagPattern matches an html_start_tag node
// Used as base pattern for component tag detection
//
// Example: <sw-button label="Click">
//
//	^^^^^^^^^^^^^^^^^^^^^^^ matches entire start tag
var TwigHTMLStartTagPattern = treesitterhelper.NodeKind("html_start_tag")

// TwigHTMLTagNamePattern matches an html_tag_name node
// Used for go-to-definition when clicking on component tag name
//
// Example: <sw-button label="Click">
//
//	^^^^^^^^^ matches "sw-button"
var TwigHTMLTagNamePattern = treesitterhelper.NodeKind("html_tag_name")

// TwigHTMLAttributeNamePattern matches an html_attribute_name node
// Used for completion and go-to-definition on prop attributes
//
// Example: <sw-button label="Click">
//
//	^^^^^ matches "label"
var TwigHTMLAttributeNamePattern = treesitterhelper.NodeKind("html_attribute_name")

// TwigVueDirectivePattern matches a vue_directive node
// Used for completion and go-to-definition on Vue bindings (:prop, v-bind:prop)
//
// Example: <sw-button :disabled="isDisabled">
//
//	^^^^^^^^^ matches ":disabled"
var TwigVueDirectivePattern = treesitterhelper.NodeKind("vue_directive")

// TwigPropAttributePattern matches either html_attribute_name or vue_directive
// Used for prop-related features (completion, go-to-definition, hover)
//
// Example: <sw-button label="x" :disabled="y">
//
//	^^^^^            ^^^^^^^^^ both match
var TwigPropAttributePattern = treesitterhelper.AnyNodeKind("html_attribute_name", "vue_directive")

// TwigSlotShorthandPattern matches the # character used for Vue slot shorthand
// In Twig parser, # inside HTML is parsed as inline_comment
//
// Example: <template #default>
//
//	^^^^^^^^ matches "#default" (parsed as inline_comment)
var TwigSlotShorthandPattern = treesitterhelper.And(
	treesitterhelper.NodeKind("inline_comment"),
	NodeTextPrefix("#"),
)

// =============================================================================
// Helper Functions
// =============================================================================

// IsComponentTag checks if a tag name represents a Vue component (contains hyphen)
// Standard HTML elements don't contain hyphens, Vue components do
func IsComponentTag(tagName string) bool {
	return strings.Contains(tagName, "-") && tagName != "template"
}

// GetTagNameFromStartTag extracts the tag name from an html_start_tag node
func GetTagNameFromStartTag(startTag *tree_sitter.Node, content []byte) string {
	if startTag == nil || startTag.Kind() != "html_start_tag" {
		return ""
	}

	for i := uint(0); i < startTag.ChildCount(); i++ {
		child := startTag.Child(i)
		if child.Kind() == "html_tag_name" {
			return string(child.Utf8Text(content))
		}
	}
	return ""
}

// GetTagNameFromEndTag extracts the tag name from an html_end_tag node
func GetTagNameFromEndTag(endTag *tree_sitter.Node, content []byte) string {
	if endTag == nil || endTag.Kind() != "html_end_tag" {
		return ""
	}

	for i := uint(0); i < endTag.ChildCount(); i++ {
		child := endTag.Child(i)
		if child.Kind() == "html_tag_name" {
			return string(child.Utf8Text(content))
		}
	}
	return ""
}

// FindAncestorOfKind walks up the tree to find an ancestor of the given kind
func FindAncestorOfKind(node *tree_sitter.Node, kind string) *tree_sitter.Node {
	if node == nil {
		return nil
	}
	current := node.Parent()
	for current != nil {
		if current.Kind() == kind {
			return current
		}
		current = current.Parent()
	}
	return nil
}

// FindParentStartTag finds the html_start_tag that contains this node
func FindParentStartTag(node *tree_sitter.Node) *tree_sitter.Node {
	return FindAncestorOfKind(node, "html_start_tag")
}

// GetComponentNameFromAttribute finds the component name for an attribute node
// Walks up to find the html_start_tag, then extracts the tag name
//
// Example: <sw-button label="x">
//
//	^^^^^ given this node, returns "sw-button"
func GetComponentNameFromAttribute(node *tree_sitter.Node, content []byte) string {
	startTag := FindParentStartTag(node)
	if startTag == nil {
		return ""
	}

	tagName := GetTagNameFromStartTag(startTag, content)
	if !IsComponentTag(tagName) {
		return ""
	}

	return tagName
}

// NormalizePropName normalizes an attribute name to a prop name
// Removes Vue binding prefixes (:, v-bind:) and converts kebab-case to camelCase
// Returns empty string for event handlers (@, v-on:) and directives (v-if, v-for, etc.)
//
// Examples:
//
//	"label"                -> "label"
//	":disabled"            -> "disabled"
//	"v-bind:position-id"   -> "positionId"
//	"position-identifier"  -> "positionIdentifier"
//	"@click"               -> "" (event handler)
//	"v-if"                 -> "" (directive)
func NormalizePropName(attrName string) string {
	name := attrName

	// Remove Vue binding prefixes
	if strings.HasPrefix(name, "v-bind:") {
		name = strings.TrimPrefix(name, "v-bind:")
	} else if strings.HasPrefix(name, "v-on:") {
		return "" // Event handler
	} else if strings.HasPrefix(name, ":") {
		name = strings.TrimPrefix(name, ":")
	} else if strings.HasPrefix(name, "@") {
		return "" // Event handler shorthand
	} else if strings.HasPrefix(name, "v-") {
		return "" // Vue directive
	}

	// Convert kebab-case to camelCase
	return KebabToCamel(name)
}

// KebabToCamel converts kebab-case to camelCase
//
// Examples:
//
//	"position-identifier" -> "positionIdentifier"
//	"my-prop-name"        -> "myPropName"
//	"label"               -> "label"
func KebabToCamel(s string) string {
	parts := strings.Split(s, "-")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// CamelToKebab converts camelCase to kebab-case
//
// Examples:
//
//	"positionIdentifier" -> "position-identifier"
//	"myPropName"         -> "my-prop-name"
//	"label"              -> "label"
func CamelToKebab(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '-')
			}
			result = append(result, byte(c+'a'-'A'))
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}
