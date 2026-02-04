package completion

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/admin"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// AdminCompletionProvider provides completions for Shopware Admin Vue components
type AdminCompletionProvider struct {
	adminIndexer *admin.AdminComponentIndexer
}

// NewAdminCompletionProvider creates a new admin completion provider
func NewAdminCompletionProvider(server *lsp.Server) *AdminCompletionProvider {
	adminIndexer, _ := server.GetIndexer("admin.component.indexer")

	return &AdminCompletionProvider{
		adminIndexer: adminIndexer.(*admin.AdminComponentIndexer),
	}
}

// GetCompletions returns completion items for admin components
func (p *AdminCompletionProvider) GetCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if params.Node == nil {
		return []protocol.CompletionItem{}
	}

	ext := strings.ToLower(filepath.Ext(params.TextDocument.URI))

	// Handle JS/TS files
	if ext == ".js" || ext == ".ts" {
		return p.jsCompletions(ctx, params)
	}

	// Handle Twig files (admin templates)
	if ext == ".twig" {
		// Only process Twig files in administration directory
		if strings.Contains(params.TextDocument.URI, "Resources/app/administration") {
			return p.twigCompletions(ctx, params)
		}
	}

	return []protocol.CompletionItem{}
}

// jsCompletions handles completions in JS/TS files
func (p *AdminCompletionProvider) jsCompletions(_ context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	var items []protocol.CompletionItem

	// Check if we're in the second argument of Component.extend (parent component name)
	if p.isInExtendParentArgument(params.Node, params.DocumentContent) {
		items = append(items, p.getComponentCompletions()...)
	}

	return items
}

// twigCompletions handles completions in Twig admin templates
func (p *AdminCompletionProvider) twigCompletions(_ context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	node := params.Node
	content := params.DocumentContent

	// Check if we're in an HTML tag name position
	if p.isInHTMLTagName(node, content) {
		return p.getComponentTagCompletions()
	}

	// Check if we're in a slot name position (# or v-slot:)
	if componentName := p.getComponentNameForSlotCompletion(node, content); componentName != "" {
		return p.getSlotCompletions(componentName)
	}

	// Check if we're in an HTML attribute position
	if componentName := p.getComponentNameForAttributeCompletion(node, content); componentName != "" {
		return p.getComponentPropCompletions(componentName)
	}

	return []protocol.CompletionItem{}
}

// isInHTMLTagName checks if the cursor is in an HTML tag name position
func (p *AdminCompletionProvider) isInHTMLTagName(node *tree_sitter.Node, content []byte) bool {
	if node == nil {
		return false
	}

	// Direct match on html_tag_name
	if node.Kind() == "html_tag_name" {
		return true
	}

	// Check if we're inside an html_start_tag or html_end_tag
	// and the cursor is at the tag name position
	if node.Kind() == "<" || node.Kind() == "</" {
		// Check if parent is html_start_tag or html_end_tag
		parent := node.Parent()
		if parent != nil && (parent.Kind() == "html_start_tag" || parent.Kind() == "html_end_tag") {
			return true
		}
		// Also trigger when parent is ERROR (incomplete tag being typed)
		if parent != nil && parent.Kind() == "ERROR" {
			return true
		}
	}

	// Handle case where we just typed '<' and it's still parsed as content
	// Check if the content ends with '<' or '<' followed by partial tag name
	if node.Kind() == "content" {
		nodeText := string(node.Utf8Text(content))
		trimmed := strings.TrimRight(nodeText, " \t\n\r")
		// Check if content ends with '<' or '<' followed by letters (partial tag name)
		if strings.HasSuffix(trimmed, "<") {
			return true
		}
		// Check for partial tag name like "<sw-" or "<sw-but"
		lastLT := strings.LastIndex(trimmed, "<")
		if lastLT != -1 {
			afterLT := trimmed[lastLT+1:]
			// If there's text after '<' and no '>' or space, we're typing a tag name
			if len(afterLT) > 0 && !strings.ContainsAny(afterLT, "> \t\n\r") {
				return true
			}
		}
	}

	return false
}

// getComponentTagCompletions returns completion items for component tags in Twig
func (p *AdminCompletionProvider) getComponentTagCompletions() []protocol.CompletionItem {
	componentNames, err := p.adminIndexer.GetAllComponentNames()
	if err != nil {
		return []protocol.CompletionItem{}
	}

	items := make([]protocol.CompletionItem, 0, len(componentNames))
	for _, name := range componentNames {
		// Create snippet: <component-name>$0</component-name>
		// $0 is the cursor position after insertion
		snippet := name + ">$0</" + name + ">"

		item := protocol.CompletionItem{
			Label:            name,
			Kind:             int(protocol.ClassCompletion),
			InsertText:       snippet,
			InsertTextFormat: int(protocol.SnippetTextFormat),
		}

		// Try to get component details for documentation
		components, err := p.adminIndexer.GetComponentWithDefinition(name)
		if err == nil && len(components) > 0 {
			comp := components[0]
			doc := "**Shopware Admin Component**\n\n"

			if comp.ExtendsComponent != "" {
				doc += "**Extends:** `" + comp.ExtendsComponent + "`\n\n"
			}

			if len(comp.Props) > 0 {
				doc += "**Props:** "
				propNames := make([]string, 0, len(comp.Props))
				for _, prop := range comp.Props {
					propNames = append(propNames, prop.Name)
				}
				doc += strings.Join(propNames, ", ") + "\n"
			}

			item.Documentation.Kind = "markdown"
			item.Documentation.Value = doc
		}

		items = append(items, item)
	}

	// Add template tag with slot shorthand
	// Don't close the template yet - the slot completion will close it
	templateItem := protocol.CompletionItem{
		Label:            "template",
		Kind:             int(protocol.ClassCompletion),
		Detail:           "slot template",
		InsertText:       "template #",
		InsertTextFormat: int(protocol.SnippetTextFormat),
	}
	templateItem.Documentation.Kind = "markdown"
	templateItem.Documentation.Value = "**Vue Slot Template**\n\nUsed to fill named slots in parent components.\n\nExample: `<template #default>...</template>`"
	items = append(items, templateItem)

	return items
}

// isInExtendParentArgument checks if cursor is in the parent component argument of Component.extend
// Pattern: Component.extend('name', '<caret>', ...)
func (p *AdminCompletionProvider) isInExtendParentArgument(node *tree_sitter.Node, content []byte) bool {
	// We need to check:
	// 1. We're in a string node (or quote character inside a string)
	// 2. The string is the second argument in a Component.extend call

	if node == nil {
		return false
	}

	// If we're on a quote character (', "), check parent node
	checkNode := node
	nodeText := string(node.Utf8Text(content))
	if nodeText == "'" || nodeText == "\"" {
		if parent := node.Parent(); parent != nil && parent.Kind() == "string" {
			checkNode = parent
		}
	}

	// Pattern: string inside arguments of a Component.extend call
	pattern := treesitterhelper.And(
		treesitterhelper.AnyNodeKind("string", "string_fragment"),
		treesitterhelper.Ancestor(
			treesitterhelper.And(
				treesitterhelper.NodeKind("call_expression"),
				treesitterhelper.HasChild(
					treesitterhelper.And(
						treesitterhelper.NodeKind("member_expression"),
						treesitterhelper.Or(
							treesitterhelper.NodeText("Component.extend"),
							treesitterhelper.NodeText("Shopware.Component.extend"),
						),
					),
				),
			),
			5, // Allow a few levels up to reach the call_expression
		),
	)

	if !pattern.Matches(checkNode, content) {
		return false
	}

	// Now verify this is the second string argument (parent name), not the first (component name)
	// We need to find the arguments node and check position
	return p.isSecondStringArgument(checkNode, content)
}

// isSecondStringArgument checks if the current node is the second string argument in a call
func (p *AdminCompletionProvider) isSecondStringArgument(node *tree_sitter.Node, content []byte) bool {
	// Walk up to find the arguments node
	current := node
	for current != nil {
		if current.Kind() == "arguments" {
			break
		}
		current = current.Parent()
	}

	if current == nil {
		return false
	}

	// Count string arguments before our node
	stringCount := 0
	targetFound := false

	for i := uint(0); i < current.ChildCount(); i++ {
		child := current.Child(i)
		if child.Kind() == "string" {
			stringCount++
			// Check if this string contains our node
			if containsNode(child, node) {
				targetFound = true
				break
			}
		}
	}

	// We want to be in the second string argument
	return targetFound && stringCount == 2
}

// containsNode checks if parent contains child (by position)
func containsNode(parent, child *tree_sitter.Node) bool {
	if parent == nil || child == nil {
		return false
	}
	parentRange := parent.Range()
	childRange := child.Range()

	return childRange.StartByte >= parentRange.StartByte && childRange.EndByte <= parentRange.EndByte
}

// getComponentCompletions returns completion items for all registered components
func (p *AdminCompletionProvider) getComponentCompletions() []protocol.CompletionItem {
	componentNames, err := p.adminIndexer.GetAllComponentNames()
	if err != nil {
		return []protocol.CompletionItem{}
	}

	items := make([]protocol.CompletionItem, 0, len(componentNames))
	for _, name := range componentNames {
		item := protocol.CompletionItem{
			Label: name,
			Kind:  int(protocol.ClassCompletion),
		}

		// Try to get component details for documentation
		components, err := p.adminIndexer.GetComponent(name)
		if err == nil && len(components) > 0 {
			comp := components[0]
			doc := "**Shopware Admin Component**\n\n"

			if comp.ExtendsComponent != "" {
				doc += "**Extends:** `" + comp.ExtendsComponent + "`\n\n"
			}

			if comp.FilePath != "" {
				doc += "**Registered in:** `" + filepath.Base(comp.FilePath) + "`\n"
			}

			item.Documentation.Kind = "markdown"
			item.Documentation.Value = doc
		}

		items = append(items, item)
	}

	return items
}

// getComponentNameForAttributeCompletion checks if we're in a position to complete attributes
// and returns the component name if so, empty string otherwise
func (p *AdminCompletionProvider) getComponentNameForAttributeCompletion(node *tree_sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	// Check if we're directly on html_attribute_name or vue_directive
	if node.Kind() == "html_attribute_name" || node.Kind() == "vue_directive" {
		return p.findComponentNameFromNode(node)
	}

	// Check if we're inside an html_start_tag (after tag name, in attribute area)
	// This handles the case when cursor is on whitespace or just typed a space
	startTag := p.findAncestorOfKind(node, "html_start_tag")
	if startTag != nil {
		// Make sure we're after the tag name (not on < or the tag name itself)
		if node.Kind() != "<" && node.Kind() != "html_tag_name" {
			return p.getTagNameFromStartTag(startTag, content)
		}
	}

	// Handle ERROR node case (incomplete attribute being typed)
	if node.Kind() == "ERROR" || (node.Parent() != nil && node.Parent().Kind() == "ERROR") {
		// Try to find the html_start_tag ancestor
		startTag := p.findAncestorOfKind(node, "html_start_tag")
		if startTag != nil {
			return p.getTagNameFromStartTag(startTag, content)
		}
	}

	return ""
}

// findAncestorOfKind walks up the tree to find an ancestor of the given kind
func (p *AdminCompletionProvider) findAncestorOfKind(node *tree_sitter.Node, kind string) *tree_sitter.Node {
	current := node
	for current != nil {
		if current.Kind() == kind {
			return current
		}
		current = current.Parent()
	}
	return nil
}

// findComponentNameFromNode walks up from an attribute node to find the component name
func (p *AdminCompletionProvider) findComponentNameFromNode(node *tree_sitter.Node) string {
	// Walk up to find html_start_tag
	startTag := p.findAncestorOfKind(node, "html_start_tag")
	if startTag == nil {
		return ""
	}

	// Find the html_tag_name child
	for i := uint(0); i < startTag.ChildCount(); i++ {
		child := startTag.Child(i)
		if child.Kind() == "html_tag_name" {
			return string(child.Utf8Text(nil)) // We need content here
		}
	}
	return ""
}

// getTagNameFromStartTag extracts the tag name from an html_start_tag node
func (p *AdminCompletionProvider) getTagNameFromStartTag(startTag *tree_sitter.Node, content []byte) string {
	for i := uint(0); i < startTag.ChildCount(); i++ {
		child := startTag.Child(i)
		if child.Kind() == "html_tag_name" {
			return string(child.Utf8Text(content))
		}
	}
	return ""
}

// getComponentPropCompletions returns completion items for component props
func (p *AdminCompletionProvider) getComponentPropCompletions(componentName string) []protocol.CompletionItem {
	components, err := p.adminIndexer.GetComponentWithDefinition(componentName)
	if err != nil || len(components) == 0 {
		return []protocol.CompletionItem{}
	}

	var items []protocol.CompletionItem

	for _, comp := range components {
		// Add props
		for _, prop := range comp.Props {
			// Regular prop
			item := protocol.CompletionItem{
				Label:  prop.Name,
				Kind:   int(protocol.PropertyCompletion),
				Detail: prop.Type,
			}

			// Build documentation
			doc := ""
			if prop.Type != "" {
				doc += "**Type:** `" + prop.Type + "`\n\n"
			}
			if prop.Required {
				doc += "**Required**\n\n"
			}
			if prop.Default != "" {
				doc += "**Default:** `" + prop.Default + "`\n"
			}

			if doc != "" {
				item.Documentation.Kind = "markdown"
				item.Documentation.Value = doc
			}

			items = append(items, item)

			// Also add Vue binding shorthand (:prop)
			bindingItem := protocol.CompletionItem{
				Label:            ":" + prop.Name,
				Kind:             int(protocol.PropertyCompletion),
				Detail:           prop.Type + " (v-bind)",
				InsertText:       ":" + prop.Name + "=\"$0\"",
				InsertTextFormat: int(protocol.SnippetTextFormat),
			}
			if doc != "" {
				bindingItem.Documentation.Kind = "markdown"
				bindingItem.Documentation.Value = doc
			}
			items = append(items, bindingItem)
		}

		// Add events (emits)
		for _, emit := range comp.Emits {
			item := protocol.CompletionItem{
				Label:            "@" + emit,
				Kind:             int(protocol.EventCompletion),
				Detail:           "event",
				InsertText:       "@" + emit + "=\"$0\"",
				InsertTextFormat: int(protocol.SnippetTextFormat),
			}
			items = append(items, item)
		}
	}

	return items
}

// GetTriggerCharacters returns the characters that trigger this completion provider
func (p *AdminCompletionProvider) GetTriggerCharacters() []string {
	return []string{"'", "\"", "<", " ", "#"}
}

// getComponentNameForSlotCompletion checks if we're in a position to complete slot names
// and returns the parent component name if so
// Slot completion is triggered inside a <template #...> or <template v-slot:...> tag
// that is a direct child of a component tag
func (p *AdminCompletionProvider) getComponentNameForSlotCompletion(node *tree_sitter.Node, content []byte) string {
	if node == nil {
		return ""
	}

	nodeText := string(node.Utf8Text(content))
	nodeKind := node.Kind()

	// Handle case: # is parsed as inline_comment in Twig, but inside html_start_tag it's a slot shorthand
	// Check if we're in an html_start_tag with a <template> tag
	if nodeKind == "inline_comment" && strings.HasPrefix(nodeText, "#") {
		parent := node.Parent()
		if parent != nil && parent.Kind() == "html_start_tag" {
			tagName := p.getTagNameFromStartTag(parent, content)
			if tagName == "template" {
				return p.findParentComponentForSlot(node, content)
			}
			// If we're directly in a component tag (not template), still try to find slots
			// This handles cases like <sw-card #default> directly
			if tagName != "" {
				if p.adminIndexer != nil {
					components, err := p.adminIndexer.GetComponent(tagName)
					if err == nil && len(components) > 0 {
						return tagName
					}
				}
			}
		}
	}

	// Handle case: typing # inside <template #>
	if nodeText == "#" || strings.HasPrefix(nodeText, "#") {
		return p.findParentComponentForSlot(node, content)
	}

	// Handle case: typing v-slot: inside <template v-slot:>
	if nodeKind == "vue_directive" || strings.HasPrefix(nodeText, "v-slot") {
		return p.findParentComponentForSlot(node, content)
	}

	// Handle case: inside html_attribute_name that is a slot directive
	if nodeKind == "html_attribute_name" {
		if strings.HasPrefix(nodeText, "#") || strings.HasPrefix(nodeText, "v-slot") {
			return p.findParentComponentForSlot(node, content)
		}
	}

	return ""
}

// findParentComponentForSlot finds the parent component tag that contains this slot template
// Structure: <component-name> ... <template #slot-name> ... </template> ... </component-name>
func (p *AdminCompletionProvider) findParentComponentForSlot(node *tree_sitter.Node, content []byte) string {
	// First, find the <template> tag we're in (html_start_tag)
	templateStartTag := p.findAncestorOfKind(node, "html_start_tag")
	if templateStartTag == nil {
		return ""
	}

	// Verify this is a <template> tag
	tagName := p.getTagNameFromStartTag(templateStartTag, content)
	if tagName != "template" {
		return ""
	}

	// Now find the html_tag that contains this template start tag
	templateHtmlTag := p.findAncestorOfKind(templateStartTag, "html_tag")
	if templateHtmlTag == nil {
		// Try finding parent component from context
		return p.findParentComponentFromContext(templateStartTag, content)
	}

	// Find the parent html_tag (the component) - need to go up from the template's html_tag
	parentHtmlTag := p.findAncestorOfKind(templateHtmlTag.Parent(), "html_tag")
	if parentHtmlTag == nil {
		return p.findParentComponentFromContext(templateStartTag, content)
	}

	// Get the tag name from the parent html_tag
	parentStartTag := p.getFirstChildOfKind(parentHtmlTag, "html_start_tag")
	if parentStartTag == nil {
		return ""
	}

	componentName := p.getTagNameFromStartTag(parentStartTag, content)

	// Verify this is a registered component
	if p.adminIndexer == nil {
		return ""
	}
	components, err := p.adminIndexer.GetComponent(componentName)
	if err != nil || len(components) == 0 {
		return ""
	}

	return componentName
}

// findParentComponentFromContext tries to find the parent component by looking at surrounding context
// Since html_tag nodes are siblings (not nested), we need to look at previous siblings to find
// the unclosed parent component tag
func (p *AdminCompletionProvider) findParentComponentFromContext(node *tree_sitter.Node, content []byte) string {
	// First, find the html_tag that contains our template
	templateHtmlTag := p.findAncestorOfKind(node, "html_tag")
	if templateHtmlTag == nil {
		return ""
	}

	// Now look at previous siblings to find the parent component's start tag
	// We need to track tag depth to find the correct parent
	tagStack := []string{}

	// Walk backwards through siblings
	current := templateHtmlTag.PrevSibling()
	for current != nil {
		if current.Kind() == "html_tag" {
			// Check if it's a start tag or end tag
			startTag := p.getFirstChildOfKind(current, "html_start_tag")
			endTag := p.getFirstChildOfKind(current, "html_end_tag")

			if endTag != nil {
				// It's an end tag - push to stack (we're going backwards)
				tagName := p.getTagNameFromEndTag(endTag, content)
				tagStack = append(tagStack, tagName)
			} else if startTag != nil {
				tagName := p.getTagNameFromStartTag(startTag, content)

				// Check if this start tag matches a pending end tag
				if len(tagStack) > 0 && tagStack[len(tagStack)-1] == tagName {
					// Pop from stack - this tag is closed
					tagStack = tagStack[:len(tagStack)-1]
				} else {
					// This is an unclosed start tag - it's our parent!
					// Check if it's a registered component
					if p.adminIndexer != nil {
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

// getTagNameFromEndTag extracts the tag name from an html_end_tag node
func (p *AdminCompletionProvider) getTagNameFromEndTag(endTag *tree_sitter.Node, content []byte) string {
	for i := uint(0); i < endTag.ChildCount(); i++ {
		child := endTag.Child(i)
		if child.Kind() == "html_tag_name" {
			return string(child.Utf8Text(content))
		}
	}
	return ""
}

// getFirstChildOfKind returns the first child of the given kind
func (p *AdminCompletionProvider) getFirstChildOfKind(node *tree_sitter.Node, kind string) *tree_sitter.Node {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child.Kind() == kind {
			return child
		}
	}
	return nil
}

// getSlotCompletions returns completion items for slot names of a component
func (p *AdminCompletionProvider) getSlotCompletions(componentName string) []protocol.CompletionItem {
	if p.adminIndexer == nil {
		return []protocol.CompletionItem{}
	}

	components, err := p.adminIndexer.GetComponentWithDefinition(componentName)
	if err != nil || len(components) == 0 {
		return []protocol.CompletionItem{}
	}

	var items []protocol.CompletionItem
	seenSlots := make(map[string]bool)

	for _, comp := range components {
		for _, slot := range comp.Slots {
			if seenSlots[slot.Name] {
				continue
			}
			seenSlots[slot.Name] = true

			// Create snippet that completes the slot and closes the template tag
			// Result: #slotName>$0</template>
			snippet := slot.Name + ">$0</template>"

			item := protocol.CompletionItem{
				Label:            slot.Name,
				Kind:             int(protocol.PropertyCompletion),
				Detail:           "slot",
				InsertText:       snippet,
				InsertTextFormat: int(protocol.SnippetTextFormat),
			}

			// Add documentation
			doc := "**Slot:** `" + slot.Name + "`\n\n"
			doc += "**Component:** `" + componentName + "`"
			item.Documentation.Kind = "markdown"
			item.Documentation.Value = doc

			items = append(items, item)
		}
	}

	return items
}
