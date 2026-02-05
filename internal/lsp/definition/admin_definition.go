package definition

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/admin"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// AdminDefinitionProvider provides go-to-definition for Shopware Admin Vue components
type AdminDefinitionProvider struct {
	adminIndexer *admin.AdminComponentIndexer
}

// NewAdminDefinitionProvider creates a new admin definition provider
func NewAdminDefinitionProvider(lspServer *lsp.Server) *AdminDefinitionProvider {
	adminIndexer, _ := lspServer.GetIndexer("admin.component.indexer")

	return &AdminDefinitionProvider{
		adminIndexer: adminIndexer.(*admin.AdminComponentIndexer),
	}
}

// GetDefinition returns the definition location for Vue components
func (p *AdminDefinitionProvider) GetDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if params.Node == nil {
		return []protocol.Location{}
	}

	ext := strings.ToLower(filepath.Ext(params.TextDocument.URI))

	// Handle JS/TS files
	if ext == ".js" || ext == ".ts" {
		return p.jsDefinition(ctx, params)
	}

	// Handle Twig files (admin templates)
	if ext == ".twig" {
		// Only process Twig files in administration directory
		if strings.Contains(params.TextDocument.URI, "Resources/app/administration") {
			return p.twigDefinition(ctx, params)
		}
	}

	return []protocol.Location{}
}

// twigDefinition handles go-to-definition for Vue components in Twig templates
func (p *AdminDefinitionProvider) twigDefinition(_ context.Context, params *protocol.DefinitionParams) []protocol.Location {
	node := params.Node
	content := params.DocumentContent

	// <sw-button<caret>> - cursor on component tag name
	if admin.TwigHTMLTagNamePattern.Matches(node, content) {
		return p.componentDefinition(node, content)
	}

	// <sw-button label<caret>="x"> or <sw-button :disabled<caret>="y">
	// cursor on prop attribute name or Vue directive
	if admin.TwigPropAttributePattern.Matches(node, content) {
		return p.propDefinition(node, content)
	}

	// <template #default<caret>> or <template #actions<caret>>
	// cursor on slot name (# shorthand parsed as inline_comment or vue_directive)
	if p.isSlotReference(node, content) {
		return p.slotDefinition(node, content)
	}

	return []protocol.Location{}
}

// componentDefinition returns the definition location for a component tag name
func (p *AdminDefinitionProvider) componentDefinition(node *tree_sitter.Node, content []byte) []protocol.Location {
	componentName := string(node.Utf8Text(content))
	if componentName == "" {
		return []protocol.Location{}
	}

	// Look up the component in the index
	components, err := p.adminIndexer.GetComponent(componentName)
	if err != nil || len(components) == 0 {
		return []protocol.Location{}
	}

	// Build location results
	var locations []protocol.Location
	for _, comp := range components {
		// Prefer definition path if available, otherwise use registration file
		targetPath := comp.DefinitionPath
		targetLine := 1 // Default to start of file for definition files

		if targetPath == "" || !fileExists(targetPath) {
			// Fallback to registration file
			targetPath = comp.FilePath
			targetLine = comp.Line
		}

		if targetPath == "" {
			continue
		}

		locations = append(locations, protocol.Location{
			URI: fmt.Sprintf("file://%s", targetPath),
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      targetLine - 1, // Convert to 0-based
					Character: 0,
				},
				End: protocol.Position{
					Line:      targetLine - 1,
					Character: 0,
				},
			},
		})
	}

	return locations
}

// propDefinition returns the definition location for a prop attribute
// <sw-button label<caret>="x"> - jump to prop definition in component's JS file
func (p *AdminDefinitionProvider) propDefinition(node *tree_sitter.Node, content []byte) []protocol.Location {
	// Get the attribute name
	attrName := string(node.Utf8Text(content))
	if attrName == "" {
		return []protocol.Location{}
	}

	// Normalize attribute name: remove Vue binding prefixes and convert to camelCase
	// e.g., ":position-identifier" -> "positionIdentifier"
	propName := admin.NormalizePropName(attrName)
	if propName == "" {
		return []protocol.Location{}
	}

	// Find the parent component tag name
	// e.g., for <sw-button label="x">, returns "sw-button"
	componentName := admin.GetComponentNameFromAttribute(node, content)
	if componentName == "" {
		return []protocol.Location{}
	}

	// Look up the component with its definition
	components, err := p.adminIndexer.GetComponentWithDefinition(componentName)
	if err != nil || len(components) == 0 {
		return []protocol.Location{}
	}

	// Find the prop in the component definition
	comp := components[0]
	for _, prop := range comp.Props {
		if prop.Name == propName {
			// Get the definition file path
			targetPath := comp.DefinitionPath
			if targetPath == "" {
				targetPath = comp.FilePath
			}

			if targetPath == "" || prop.Line == 0 {
				continue
			}

			return []protocol.Location{
				{
					URI: fmt.Sprintf("file://%s", targetPath),
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      prop.Line - 1, // Convert to 0-based
							Character: 0,
						},
						End: protocol.Position{
							Line:      prop.Line - 1,
							Character: 0,
						},
					},
				},
			}
		}
	}

	return []protocol.Location{}
}

// isSlotReference checks if the node is a slot reference in a template tag
// <template #slotName> or <template v-slot:slotName>
func (p *AdminDefinitionProvider) isSlotReference(node *tree_sitter.Node, content []byte) bool {
	if node == nil {
		return false
	}

	nodeText := string(node.Utf8Text(content))
	nodeKind := node.Kind()

	// Check for # shorthand (parsed as inline_comment in Twig)
	// e.g., <template #default>
	if nodeKind == "inline_comment" && strings.HasPrefix(nodeText, "#") {
		// Verify it's inside a <template> tag
		startTag := admin.FindParentStartTag(node)
		if startTag != nil {
			tagName := admin.GetTagNameFromStartTag(startTag, content)
			return tagName == "template"
		}
	}

	// Check for vue_directive with v-slot or # prefix
	// e.g., <template v-slot:default> or <template #default>
	if nodeKind == "vue_directive" {
		if strings.HasPrefix(nodeText, "#") || strings.HasPrefix(nodeText, "v-slot:") {
			startTag := admin.FindParentStartTag(node)
			if startTag != nil {
				tagName := admin.GetTagNameFromStartTag(startTag, content)
				return tagName == "template"
			}
		}
	}

	return false
}

// slotDefinition returns the definition location for a slot reference
// <template #default<caret>> - jump to <slot name="default"> in component's template
func (p *AdminDefinitionProvider) slotDefinition(node *tree_sitter.Node, content []byte) []protocol.Location {
	// Extract slot name from the node text
	slotName := p.extractSlotName(node, content)
	if slotName == "" {
		return []protocol.Location{}
	}

	// Find the parent component that contains this template
	componentName := p.findParentComponentForSlot(node, content)
	if componentName == "" {
		return []protocol.Location{}
	}

	// Look up the component with its definition
	components, err := p.adminIndexer.GetComponentWithDefinition(componentName)
	if err != nil || len(components) == 0 {
		return []protocol.Location{}
	}

	// Find the slot in the component's slots
	comp := components[0]
	for _, slot := range comp.Slots {
		if slot.Name == slotName {
			// Get the template path
			templatePath := comp.TemplatePath
			if templatePath == "" {
				continue
			}

			return []protocol.Location{
				{
					URI: fmt.Sprintf("file://%s", templatePath),
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      slot.Line - 1, // Convert to 0-based
							Character: 0,
						},
						End: protocol.Position{
							Line:      slot.Line - 1,
							Character: 0,
						},
					},
				},
			}
		}
	}

	return []protocol.Location{}
}

// extractSlotName extracts the slot name from a slot reference node
// "#default" -> "default", "v-slot:actions" -> "actions"
func (p *AdminDefinitionProvider) extractSlotName(node *tree_sitter.Node, content []byte) string {
	nodeText := string(node.Utf8Text(content))

	var slotName string

	// Remove # prefix
	if strings.HasPrefix(nodeText, "#") {
		slotName = strings.TrimPrefix(nodeText, "#")
	} else if strings.HasPrefix(nodeText, "v-slot:") {
		// Remove v-slot: prefix
		slotName = strings.TrimPrefix(nodeText, "v-slot:")
	}

	// Clean up: the inline_comment may include trailing ">", newlines, etc.
	// e.g., "#content>\n" -> "content"
	if idx := strings.IndexAny(slotName, ">\n\r\t ="); idx != -1 {
		slotName = slotName[:idx]
	}
	slotName = strings.TrimSpace(slotName)

	return slotName
}

// findParentComponentForSlot finds the parent component tag that contains this slot template
// Structure: <component-name> ... <template #slot-name> ... </template> ... </component-name>
func (p *AdminDefinitionProvider) findParentComponentForSlot(node *tree_sitter.Node, content []byte) string {
	// First, find the <template> tag we're in
	templateStartTag := admin.FindParentStartTag(node)
	if templateStartTag == nil {
		return ""
	}

	// Find the html_tag that contains this template
	templateHtmlTag := admin.FindAncestorOfKind(templateStartTag, "html_tag")
	if templateHtmlTag == nil {
		// Try finding via sibling traversal
		return p.findParentComponentFromSiblings(templateStartTag, content)
	}

	// Find the parent html_tag (the component)
	parent := templateHtmlTag.Parent()
	if parent == nil {
		return p.findParentComponentFromSiblings(templateStartTag, content)
	}

	parentHtmlTag := admin.FindAncestorOfKind(parent, "html_tag")
	if parentHtmlTag == nil {
		return p.findParentComponentFromSiblings(templateStartTag, content)
	}

	// Get the tag name from the parent
	for i := uint(0); i < parentHtmlTag.ChildCount(); i++ {
		child := parentHtmlTag.Child(i)
		if child.Kind() == "html_start_tag" {
			tagName := admin.GetTagNameFromStartTag(child, content)
			if admin.IsComponentTag(tagName) {
				// Verify component exists
				components, err := p.adminIndexer.GetComponent(tagName)
				if err == nil && len(components) > 0 {
					return tagName
				}
			}
		}
	}

	return p.findParentComponentFromSiblings(templateStartTag, content)
}

// findParentComponentFromSiblings finds the parent component by walking through siblings
// This is needed because in Twig AST, html_tag nodes are often siblings rather than nested
func (p *AdminDefinitionProvider) findParentComponentFromSiblings(node *tree_sitter.Node, content []byte) string {
	// Find the html_tag that contains our template
	templateHtmlTag := admin.FindAncestorOfKind(node, "html_tag")
	if templateHtmlTag == nil {
		return ""
	}

	// Track tag stack to find unclosed parent
	tagStack := []string{}

	// Walk backwards through siblings
	current := templateHtmlTag.PrevSibling()
	for current != nil {
		if current.Kind() == "html_tag" {
			// Check for start tag or end tag
			var startTag, endTag *tree_sitter.Node
			for i := uint(0); i < current.ChildCount(); i++ {
				child := current.Child(i)
				switch child.Kind() {
				case "html_start_tag":
					startTag = child
				case "html_end_tag":
					endTag = child
				}
			}

			if endTag != nil {
				// Push to stack (going backwards)
				tagName := admin.GetTagNameFromEndTag(endTag, content)
				tagStack = append(tagStack, tagName)
			} else if startTag != nil {
				tagName := admin.GetTagNameFromStartTag(startTag, content)

				// Check if matches pending end tag
				if len(tagStack) > 0 && tagStack[len(tagStack)-1] == tagName {
					tagStack = tagStack[:len(tagStack)-1]
				} else {
					// Unclosed start tag - potential parent
					if admin.IsComponentTag(tagName) {
						components, err := p.adminIndexer.GetComponent(tagName)
						if err == nil && len(components) > 0 {
							return tagName
						}
					}
				}
			}
		}
		current = current.PrevSibling()
	}

	return ""
}

// kebabToCamel converts kebab-case to camelCase (used by tests)
func kebabToCamel(s string) string {
	return admin.KebabToCamel(s)
}

func (p *AdminDefinitionProvider) jsDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	node := params.Node
	content := params.DocumentContent

	// Check if we're on a string that could be a component name
	if node.Kind() != "string" && node.Kind() != "string_fragment" {
		return []protocol.Location{}
	}

	// Check if this string is in a Component.extend or Component.register call
	if !p.isInComponentCall(node, content) {
		return []protocol.Location{}
	}

	// Extract the component name
	componentName := p.extractComponentName(node, content)
	if componentName == "" {
		return []protocol.Location{}
	}

	// Look up the component in the index
	components, err := p.adminIndexer.GetComponent(componentName)
	if err != nil || len(components) == 0 {
		return []protocol.Location{}
	}

	// Build location results
	var locations []protocol.Location
	for _, comp := range components {
		// Prefer definition path if available, otherwise use registration file
		targetPath := comp.DefinitionPath
		targetLine := 1 // Default to start of file for definition files

		if targetPath == "" || !fileExists(targetPath) {
			// Fallback to registration file
			targetPath = comp.FilePath
			targetLine = comp.Line
		}

		if targetPath == "" {
			continue
		}

		locations = append(locations, protocol.Location{
			URI: fmt.Sprintf("file://%s", targetPath),
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      targetLine - 1, // Convert to 0-based
					Character: 0,
				},
				End: protocol.Position{
					Line:      targetLine - 1,
					Character: 0,
				},
			},
		})
	}

	return locations
}

// isInComponentCall checks if the node is within a Component.register/extend call
// Component.extend('<caret>', 'parent', ...) or Component.register('<caret>', ...)
func (p *AdminDefinitionProvider) isInComponentCall(node *tree_sitter.Node, content []byte) bool {
	// Use the shared pattern: string inside Component.register/extend call
	pattern := treesitterhelper.Ancestor(admin.JSComponentCallPattern, 5)
	return pattern.Matches(node, content)
}

// extractComponentName extracts the string content from the node
func (p *AdminDefinitionProvider) extractComponentName(node *tree_sitter.Node, content []byte) string {
	if node.Kind() == "string_fragment" {
		return string(node.Utf8Text(content))
	}

	if node.Kind() == "string" {
		// Find the string_fragment child
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child.Kind() == "string_fragment" {
				return string(child.Utf8Text(content))
			}
		}
		// Fallback: trim quotes
		text := string(node.Utf8Text(content))
		return strings.Trim(text, "\"'")
	}

	return ""
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	// Simple check - we could use os.Stat but for LSP purposes
	// we'll just return true and let the editor handle missing files
	return path != ""
}
