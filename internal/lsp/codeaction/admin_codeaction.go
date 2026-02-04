package codeaction

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopware/shopware-lsp/internal/admin"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// AdminCodeActionProvider provides code actions for Shopware Admin Vue components
type AdminCodeActionProvider struct {
	adminIndexer *admin.AdminComponentIndexer
}

// NewAdminCodeActionProvider creates a new admin code action provider
func NewAdminCodeActionProvider(server *lsp.Server) *AdminCodeActionProvider {
	adminIndexer, _ := server.GetIndexer("admin.component.indexer")

	return &AdminCodeActionProvider{
		adminIndexer: adminIndexer.(*admin.AdminComponentIndexer),
	}
}

// GetCodeActionKinds returns the kinds of code actions this provider can provide
func (p *AdminCodeActionProvider) GetCodeActionKinds() []protocol.CodeActionKind {
	return []protocol.CodeActionKind{
		protocol.CodeActionQuickFix,
	}
}

// GetCodeActions returns code actions for the given parameters
func (p *AdminCodeActionProvider) GetCodeActions(ctx context.Context, params *protocol.CodeActionParams) []protocol.CodeAction {
	// Only process files in administration directory
	if !strings.Contains(params.TextDocument.URI, "Resources/app/administration") {
		return nil
	}

	// Only handle .twig files
	if !strings.HasSuffix(params.TextDocument.URI, ".twig") {
		return nil
	}

	var codeActions []protocol.CodeAction

	// Check diagnostics for missing required props
	for _, diag := range params.Context.Diagnostics {
		// Code comes as interface{} from JSON, so convert to string for comparison
		codeStr, _ := diag.Code.(string)
		if codeStr == "admin.component.missing-required-prop" {
			action := p.createAddPropAction(params, &diag)
			if action != nil {
				codeActions = append(codeActions, *action)
			}
		}
	}

	return codeActions
}

// createAddPropAction creates a code action to add a missing prop
func (p *AdminCodeActionProvider) createAddPropAction(params *protocol.CodeActionParams, diag *protocol.Diagnostic) *protocol.CodeAction {
	data, ok := diag.Data.(map[string]any)
	if !ok {
		return nil
	}

	componentName, _ := data["componentName"].(string)
	propName, _ := data["propName"].(string)
	if componentName == "" || propName == "" {
		return nil
	}

	// Get prop type to determine value format
	propAttr, defaultValue := p.getPropAttributeFormat(componentName, propName)

	// Find the position to insert the prop (before the > of the start tag)
	insertPos := p.findInsertPosition(params, diag)
	if insertPos == nil {
		return nil
	}

	// Build the snippet text with cursor placeholder inside quotes
	snippetText := fmt.Sprintf(" %s=\"%s$0\"", propAttr, defaultValue)

	return &protocol.CodeAction{
		Title:       fmt.Sprintf("Add missing prop '%s'", propName),
		Kind:        protocol.CodeActionQuickFix,
		Diagnostics: []protocol.Diagnostic{*diag},
		Command: &protocol.CommandAction{
			Title:   "Add missing prop",
			Command: "shopware.admin.addProp",
			Arguments: []interface{}{
				params.TextDocument.URI,
				insertPos.Line,
				insertPos.Character,
				snippetText,
			},
		},
	}
}

// getPropAttributeFormat returns the attribute format and default value for a prop
func (p *AdminCodeActionProvider) getPropAttributeFormat(componentName, propName string) (string, string) {
	// Convert camelCase prop name to kebab-case for the attribute
	kebabName := camelToKebab(propName)

	// Try to get the component definition for prop type info
	if p.adminIndexer != nil {
		components, err := p.adminIndexer.GetComponentWithDefinition(componentName)
		if err == nil && len(components) > 0 {
			for _, prop := range components[0].Props {
				if prop.Name == propName {
					return p.formatPropByType(kebabName, prop.Type, prop.Default)
				}
			}
		}
	}

	// Default: plain attribute with empty string
	return kebabName, ""
}

// formatPropByType returns the attribute format based on prop type
func (p *AdminCodeActionProvider) formatPropByType(kebabName, propType, defaultVal string) (string, string) {
	switch strings.ToLower(propType) {
	case "boolean":
		if defaultVal != "" {
			return ":" + kebabName, defaultVal
		}
		return ":" + kebabName, "false"
	case "number":
		if defaultVal != "" {
			return ":" + kebabName, defaultVal
		}
		return ":" + kebabName, "0"
	case "array":
		if defaultVal != "" {
			return ":" + kebabName, defaultVal
		}
		return ":" + kebabName, "[]"
	case "object":
		if defaultVal != "" {
			return ":" + kebabName, defaultVal
		}
		return ":" + kebabName, "{}"
	case "function":
		return ":" + kebabName, "() => {}"
	default:
		// String or unknown type
		return kebabName, ""
	}
}

// findInsertPosition finds the position to insert the prop (before the > or /> of the start tag)
func (p *AdminCodeActionProvider) findInsertPosition(params *protocol.CodeActionParams, diag *protocol.Diagnostic) *protocol.Position {
	if params.Node == nil {
		return nil
	}

	// Find the html_start_tag by walking up from the current node
	startTag := p.findParentStartTag(params.Node)
	if startTag == nil {
		return nil
	}

	// Find the position just before the closing > or />
	endPos := startTag.EndPosition()

	// Look at the end of the tag to find the > position
	tagText := treesitterhelper.GetNodeText(startTag, params.DocumentContent)

	// Find the position before > or />
	insertCol := int(endPos.Column)
	if strings.HasSuffix(tagText, "/>") {
		insertCol = int(endPos.Column) - 2
	} else if strings.HasSuffix(tagText, ">") {
		insertCol = int(endPos.Column) - 1
	}

	return &protocol.Position{
		Line:      int(endPos.Row),
		Character: insertCol,
	}
}

// findParentStartTag walks up the tree to find the html_start_tag ancestor
func (p *AdminCodeActionProvider) findParentStartTag(node *tree_sitter.Node) *tree_sitter.Node {
	return admin.FindParentStartTag(node)
}

// camelToKebab converts camelCase to kebab-case (delegates to shared function)
func camelToKebab(s string) string {
	return admin.CamelToKebab(s)
}
