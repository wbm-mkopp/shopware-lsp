package symfony

import (
	"context"
	"log"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// SymfonyCompletionProvider provides completions for Symfony services and tags
type SymfonyCompletionProvider struct {
	serviceIndex *ServiceIndex
	server       *lsp.Server
}

// NewServiceCompletionProvider creates a new service completion provider
func NewServiceCompletionProvider(serviceIndex *ServiceIndex, server *lsp.Server) *SymfonyCompletionProvider {
	return &SymfonyCompletionProvider{
		serviceIndex: serviceIndex,
		server:       server,
	}
}

// GetCompletions returns completion items based on the provider type
func (p *SymfonyCompletionProvider) GetCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	// Check if we're in an XML file
	uri := params.TextDocument.URI
	if !strings.HasSuffix(strings.ToLower(uri), ".xml") {
		log.Printf("Not showing completions for non-XML file: %s", uri)
		return []protocol.CompletionItem{}
	}

	if params.Node == nil {
		return []protocol.CompletionItem{}
	}

	if !p.isServiceIDContext(params.Node, params.DocumentContent) {
		return []protocol.CompletionItem{}
	}

	// Get all services from the index
	serviceIDs := p.serviceIndex.GetAllServices()

	// Convert to completion items
	items := make([]protocol.CompletionItem, 0, len(serviceIDs))
	for _, serviceID := range serviceIDs {
		item := protocol.CompletionItem{
			Label: serviceID,
			Kind:  6, // 6 = Class
		}

		// Try to get detailed service information
		if service, found := p.serviceIndex.GetServiceByID(serviceID); found {
			// Add class information to documentation
			documentation := "Symfony service ID\n\n"

			// Add class information
			if service.Class != "" {
				documentation += "**Class:** `" + service.Class + "`\n\n"
			}

			// Add tags information if available
			if len(service.Tags) > 0 {
				documentation += "**Tags:**\n"
				for tag := range service.Tags {
					documentation += "- " + tag + "\n"
				}
			}

			item.Documentation.Kind = "markdown"
			item.Documentation.Value = documentation
		} else {
			// Default documentation
			item.Documentation.Kind = "markdown"
			item.Documentation.Value = "Symfony service ID"
		}

		items = append(items, item)
	}

	return items
}

// GetTriggerCharacters returns the characters that trigger this completion provider
func (p *SymfonyCompletionProvider) GetTriggerCharacters() []string {
	return []string{"\""}
}

func (p *SymfonyCompletionProvider) isServiceIDContext(node *tree_sitter.Node, docText string) bool {
	if node.Kind() == "AttValue" && node.Parent() != nil && node.Parent().Kind() == "Attribute" {
		attrNode := node.Parent()

		// Get the attribute name
		nameNode := treesitterhelper.GetFirstNodeOfKind(attrNode, "Name")
		if nameNode == nil {
			return false
		}

		attrName := nameNode.Utf8Text([]byte(docText))
		if attrName != "id" {
			return false
		}

		// Get the parent element
		parentElement := attrNode.Parent()
		if parentElement == nil {
			return false
		}

		// Check if the parent element has a type="service" attribute
		attrValues := treesitterhelper.GetXmlAttributeValues(parentElement, docText)
		if attrValues == nil || attrValues["type"] != "\"service\"" {
			return false
		}

		// Check if the parent element is an argument element
		elementNameNode := treesitterhelper.GetFirstNodeOfKind(parentElement, "Name")
		if elementNameNode == nil {
			return false
		}

		elementName := elementNameNode.Utf8Text([]byte(docText))
		return elementName == "argument"
	}

	return false
}
