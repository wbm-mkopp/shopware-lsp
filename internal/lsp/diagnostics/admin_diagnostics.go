package diagnostics

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

// AdminDiagnosticsProvider provides diagnostics for Shopware Admin Vue components
type AdminDiagnosticsProvider struct {
	adminIndexer *admin.AdminComponentIndexer
}

// NewAdminDiagnosticsProvider creates a new admin diagnostics provider
func NewAdminDiagnosticsProvider(lspServer *lsp.Server) *AdminDiagnosticsProvider {
	adminIndexer, _ := lspServer.GetIndexer("admin.component.indexer")

	return &AdminDiagnosticsProvider{
		adminIndexer: adminIndexer.(*admin.AdminComponentIndexer),
	}
}

// GetDiagnostics returns diagnostics for admin component files
func (p *AdminDiagnosticsProvider) GetDiagnostics(ctx context.Context, uri string, rootNode *tree_sitter.Node, content []byte) ([]protocol.Diagnostic, error) {
	// Safety check for nil node
	if rootNode == nil {
		return []protocol.Diagnostic{}, nil
	}

	// Only process files in administration directory
	if !strings.Contains(uri, "Resources/app/administration") {
		return []protocol.Diagnostic{}, nil
	}

	ext := strings.ToLower(filepath.Ext(uri))

	// Handle JS/TS files
	if ext == ".js" || ext == ".ts" {
		return p.jsDiagnostics(ctx, uri, rootNode, content)
	}

	// Handle Twig files
	if ext == ".twig" {
		return p.twigDiagnostics(ctx, uri, rootNode, content)
	}

	return []protocol.Diagnostic{}, nil
}

func (p *AdminDiagnosticsProvider) jsDiagnostics(_ context.Context, _ string, rootNode *tree_sitter.Node, content []byte) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic

	// Find all Component.extend calls
	extendCalls := treesitterhelper.FindAll(rootNode, admin.JSComponentCallPattern, content)

	for _, callNode := range extendCalls {
		// Check if this is an extend call (not register)
		memberExpr := treesitterhelper.GetFirstNodeOfKind(callNode, "member_expression")
		if memberExpr == nil {
			continue
		}

		memberText := string(memberExpr.Utf8Text(content))
		if !strings.HasSuffix(memberText, ".extend") {
			continue
		}

		// Get the arguments
		argsNode := treesitterhelper.GetFirstNodeOfKind(callNode, "arguments")
		if argsNode == nil {
			continue
		}

		// Find the second string argument (parent component name)
		parentNameNode := p.getSecondStringArg(argsNode, content)
		if parentNameNode == nil {
			continue
		}

		parentName := extractStringContent(parentNameNode, content)
		if parentName == "" {
			continue
		}

		// Check if parent component exists
		components, err := p.adminIndexer.GetComponent(parentName)
		if err != nil || len(components) == 0 {
			diagnostics = append(diagnostics, protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      int(parentNameNode.StartPosition().Row),
						Character: int(parentNameNode.StartPosition().Column),
					},
					End: protocol.Position{
						Line:      int(parentNameNode.EndPosition().Row),
						Character: int(parentNameNode.EndPosition().Column),
					},
				},
				Message:  fmt.Sprintf("Parent component '%s' is not registered", parentName),
				Source:   "shopware",
				Severity: protocol.DiagnosticSeverityWarning,
				Code:     "admin.component.parent-not-found",
				Data: map[string]any{
					"componentName": parentName,
				},
			})
		}
	}

	return diagnostics, nil
}

// getSecondStringArg returns the second string argument from an arguments node
func (p *AdminDiagnosticsProvider) getSecondStringArg(argsNode *tree_sitter.Node, content []byte) *tree_sitter.Node {
	stringCount := 0

	for i := uint(0); i < argsNode.ChildCount(); i++ {
		child := argsNode.Child(i)
		if child.Kind() == "string" {
			stringCount++
			if stringCount == 2 {
				return child
			}
		}
	}

	return nil
}

// extractStringContent extracts the content from a string node
func extractStringContent(node *tree_sitter.Node, content []byte) string {
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

// twigDiagnostics checks Twig templates for missing required props on components
func (p *AdminDiagnosticsProvider) twigDiagnostics(_ context.Context, uri string, rootNode *tree_sitter.Node, content []byte) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic

	// Find all html_start_tag nodes
	p.findHTMLStartTags(rootNode, content, &diagnostics)

	// Check for invalid block references in component overrides
	p.checkBlockReferences(uri, rootNode, content, &diagnostics)

	return diagnostics, nil
}

// checkBlockReferences checks if blocks referenced in an override template exist in the parent component
func (p *AdminDiagnosticsProvider) checkBlockReferences(uri string, rootNode *tree_sitter.Node, content []byte, diagnostics *[]protocol.Diagnostic) {
	// Get the file path from URI
	filePath := strings.TrimPrefix(uri, "file://")

	// Find which component this template belongs to
	comp, err := p.adminIndexer.GetComponentByTemplatePath(filePath)
	if err != nil || comp == nil {
		return
	}

	// Only check if this component extends another
	if comp.ExtendsComponent == "" {
		return
	}

	// Get the parent component with its definition (including blocks)
	parentComps, err := p.adminIndexer.GetComponentWithDefinition(comp.ExtendsComponent)
	if err != nil || len(parentComps) == 0 {
		return
	}

	parentComp := parentComps[0]

	// Build a set of valid block names from parent (and parent's parents recursively)
	validBlocks := p.collectParentBlocks(parentComp)

	// Find all block tags in the current template
	p.findBlockTags(rootNode, content, validBlocks, diagnostics)
}

// collectParentBlocks collects all block names from a component and its parents recursively
func (p *AdminDiagnosticsProvider) collectParentBlocks(comp admin.VueComponent) map[string]bool {
	blocks := make(map[string]bool)

	// Add blocks from this component
	for _, block := range comp.Blocks {
		blocks[block.Name] = true
	}

	// If this component extends another, get those blocks too
	if comp.ExtendsComponent != "" {
		parentComps, err := p.adminIndexer.GetComponentWithDefinition(comp.ExtendsComponent)
		if err == nil && len(parentComps) > 0 {
			parentBlocks := p.collectParentBlocks(parentComps[0])
			for name := range parentBlocks {
				blocks[name] = true
			}
		}
	}

	return blocks
}

// findBlockTags finds all {% block %} tags and checks if they exist in valid blocks
func (p *AdminDiagnosticsProvider) findBlockTags(node *tree_sitter.Node, content []byte, validBlocks map[string]bool, diagnostics *[]protocol.Diagnostic) {
	if node == nil {
		return
	}

	// Check if this is a block tag
	if node.Kind() == "block" {
		blockName := p.getBlockName(node, content)
		if blockName != "" && !validBlocks[blockName] {
			// Find the identifier node for precise error location
			identNode := p.getBlockIdentifierNode(node)
			if identNode != nil {
				*diagnostics = append(*diagnostics, protocol.Diagnostic{
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      int(identNode.StartPosition().Row),
							Character: int(identNode.StartPosition().Column),
						},
						End: protocol.Position{
							Line:      int(identNode.EndPosition().Row),
							Character: int(identNode.EndPosition().Column),
						},
					},
					Message:  fmt.Sprintf("Block '%s' does not exist in parent component", blockName),
					Source:   "shopware",
					Severity: protocol.DiagnosticSeverityError,
					Code:     "admin.component.block-not-found",
					Data: map[string]any{
						"blockName": blockName,
					},
				})
			}
		}
	}

	// Recurse into children
	for i := uint(0); i < node.ChildCount(); i++ {
		p.findBlockTags(node.Child(i), content, validBlocks, diagnostics)
	}
}

// getBlockName extracts the block name from a block node
func (p *AdminDiagnosticsProvider) getBlockName(blockNode *tree_sitter.Node, content []byte) string {
	for i := uint(0); i < blockNode.ChildCount(); i++ {
		child := blockNode.Child(i)
		if child.Kind() == "identifier" {
			return string(child.Utf8Text(content))
		}
	}
	return ""
}

// getBlockIdentifierNode returns the identifier node from a block node
func (p *AdminDiagnosticsProvider) getBlockIdentifierNode(blockNode *tree_sitter.Node) *tree_sitter.Node {
	for i := uint(0); i < blockNode.ChildCount(); i++ {
		child := blockNode.Child(i)
		if child.Kind() == "identifier" {
			return child
		}
	}
	return nil
}

// findHTMLStartTags recursively finds all html_start_tag nodes and checks for missing required props
func (p *AdminDiagnosticsProvider) findHTMLStartTags(node *tree_sitter.Node, content []byte, diagnostics *[]protocol.Diagnostic) {
	if node == nil {
		return
	}

	if node.Kind() == "html_start_tag" {
		p.checkComponentProps(node, content, diagnostics)
	}

	// Recurse into children
	for i := uint(0); i < node.ChildCount(); i++ {
		p.findHTMLStartTags(node.Child(i), content, diagnostics)
	}
}

// checkComponentProps checks if a component tag has all required props
// <sw-button<caret>> - checks that all required props are present
func (p *AdminDiagnosticsProvider) checkComponentProps(startTag *tree_sitter.Node, content []byte, diagnostics *[]protocol.Diagnostic) {
	// Get the tag name
	tagName := p.getTagName(startTag, content)
	if tagName == "" {
		return
	}

	// Skip non-component tags (standard HTML elements and template)
	if !admin.IsComponentTag(tagName) {
		return
	}

	// Get the component definition
	components, err := p.adminIndexer.GetComponentWithDefinition(tagName)
	if err != nil || len(components) == 0 {
		return // Component not found - could add a diagnostic for this too
	}

	comp := components[0]

	// Get the attributes present on the tag
	presentAttrs := p.getTagAttributes(startTag, content)

	// Check for missing required props
	for _, prop := range comp.Props {
		if !prop.Required {
			continue
		}

		// Check if prop is present (also check Vue binding variants)
		if p.isPropPresent(prop.Name, presentAttrs) {
			continue
		}

		// Get the tag name node for the diagnostic range
		tagNameNode := p.getTagNameNode(startTag)
		if tagNameNode == nil {
			continue
		}

		*diagnostics = append(*diagnostics, protocol.Diagnostic{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      int(tagNameNode.StartPosition().Row),
					Character: int(tagNameNode.StartPosition().Column),
				},
				End: protocol.Position{
					Line:      int(tagNameNode.EndPosition().Row),
					Character: int(tagNameNode.EndPosition().Column),
				},
			},
			Message:  fmt.Sprintf("Missing required prop '%s' on component '%s'", prop.Name, tagName),
			Source:   "shopware",
			Severity: protocol.DiagnosticSeverityWarning,
			Code:     "admin.component.missing-required-prop",
			Data: map[string]any{
				"componentName": tagName,
				"propName":      prop.Name,
			},
		})
	}
}

// getTagName extracts the tag name from an html_start_tag node
func (p *AdminDiagnosticsProvider) getTagName(startTag *tree_sitter.Node, content []byte) string {
	return admin.GetTagNameFromStartTag(startTag, content)
}

// getTagNameNode returns the html_tag_name node from an html_start_tag
func (p *AdminDiagnosticsProvider) getTagNameNode(startTag *tree_sitter.Node) *tree_sitter.Node {
	for i := uint(0); i < startTag.ChildCount(); i++ {
		child := startTag.Child(i)
		if child.Kind() == "html_tag_name" {
			return child
		}
	}
	return nil
}

// getTagAttributes extracts all attribute names from an html_start_tag
func (p *AdminDiagnosticsProvider) getTagAttributes(startTag *tree_sitter.Node, content []byte) map[string]bool {
	attrs := make(map[string]bool)

	for i := uint(0); i < startTag.ChildCount(); i++ {
		child := startTag.Child(i)
		if child.Kind() == "html_attribute" {
			attrName := p.getAttributeName(child, content)
			if attrName != "" {
				attrs[attrName] = true
			}
		}
	}

	return attrs
}

// getAttributeName extracts the attribute name from an html_attribute node
func (p *AdminDiagnosticsProvider) getAttributeName(attrNode *tree_sitter.Node, content []byte) string {
	for i := uint(0); i < attrNode.ChildCount(); i++ {
		child := attrNode.Child(i)
		if child.Kind() == "html_attribute_name" || child.Kind() == "vue_directive" {
			return string(child.Utf8Text(content))
		}
	}
	return ""
}

// isPropPresent checks if a prop is present in the attributes
// It checks for the prop name directly, as well as Vue binding variants (:prop, v-bind:prop)
// Also handles camelCase to kebab-case conversion (positionIdentifier -> position-identifier)
func (p *AdminDiagnosticsProvider) isPropPresent(propName string, attrs map[string]bool) bool {
	// Get both camelCase and kebab-case versions
	kebabName := camelToKebab(propName)

	// Check both variants
	namesToCheck := []string{propName, kebabName}

	for _, name := range namesToCheck {
		// Direct attribute
		if attrs[name] {
			return true
		}

		// Vue shorthand binding :propName
		if attrs[":"+name] {
			return true
		}

		// Vue v-bind:propName
		if attrs["v-bind:"+name] {
			return true
		}
	}

	return false
}

// camelToKebab converts camelCase to kebab-case (delegates to shared function)
func camelToKebab(s string) string {
	return admin.CamelToKebab(s)
}
