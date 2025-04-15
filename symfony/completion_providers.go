package symfony

import (
	"context"
	"log"
	"strings"

	tree_sitter_xml "github.com/tree-sitter-grammars/tree-sitter-xml/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	"github.com/shopware/shopware-lsp/lsp"
)

// CompletionType represents the type of completion
type CompletionType int

const (
	// ServiceCompletion provides completions for Symfony services
	ServiceCompletion CompletionType = iota
	// TagCompletion provides completions for Symfony tags
	TagCompletion
)

// SymfonyCompletionProvider provides completions for Symfony services and tags
type SymfonyCompletionProvider struct {
	serviceIndex *ServiceIndex
	server       *lsp.Server
	compType     CompletionType
}

// NewServiceCompletionProvider creates a new service completion provider
func NewServiceCompletionProvider(serviceIndex *ServiceIndex, server *lsp.Server) *SymfonyCompletionProvider {
	return &SymfonyCompletionProvider{
		serviceIndex: serviceIndex,
		server:       server,
		compType:     ServiceCompletion,
	}
}

// NewTagCompletionProvider creates a new tag completion provider
func NewTagCompletionProvider(serviceIndex *ServiceIndex, server *lsp.Server) *SymfonyCompletionProvider {
	return &SymfonyCompletionProvider{
		serviceIndex: serviceIndex,
		server:       server,
		compType:     TagCompletion,
	}
}

// GetCompletions returns completion items based on the provider type
func (p *SymfonyCompletionProvider) GetCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	switch p.compType {
	case ServiceCompletion:
		return p.getServiceCompletions(ctx, params)
	case TagCompletion:
		return p.getTagCompletions(ctx, params)
	default:
		return []protocol.CompletionItem{}
	}
}

// getServiceCompletions returns service completion items
func (p *SymfonyCompletionProvider) getServiceCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	// Check if we're in an XML file
	uri := params.TextDocument.URI
	if !strings.HasSuffix(strings.ToLower(uri), ".xml") {
		log.Printf("Not showing completions for non-XML file: %s", uri)
		return []protocol.CompletionItem{}
	}

	// Get the document text
	documentText, exists := p.server.DocumentManager().GetDocumentText(uri)
	if !exists {
		log.Printf("Document not found: %s", uri)
		return []protocol.CompletionItem{}
	}

	// Use tree-sitter to parse the XML and determine the context
	parser := tree_sitter.NewParser()
	defer parser.Close()

	parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_xml.LanguageXML()))

	tree := parser.Parse([]byte(documentText), nil)
	defer tree.Close()

	// Find the closest element to our cursor position
	position := tree_sitter.Point{
		Row:    uint(params.Position.Line),
		Column: uint(params.Position.Character),
	}

	// Find the node at the cursor position
	node := tree.RootNode().NamedDescendantForPointRange(position, position)

	if node == nil {
		return []protocol.CompletionItem{}
	}

	if node.Kind() == "AttValue" && node.Parent() != nil && node.Parent().Kind() == "Attribute" {
		attrNode := node.Parent()

		nameNode := treesitterhelper.GetFirstNodeOfKind(attrNode, "Name")

		if nameNode == nil {
			return []protocol.CompletionItem{}
		}

		attrName := nameNode.Utf8Text([]byte(documentText))

		if attrName != "id" {
			return []protocol.CompletionItem{}
		}

		parentElement := attrNode.Parent()

		if parentElement == nil {
			return []protocol.CompletionItem{}
		}

		attrValues := treesitterhelper.GetXmlAttributeValues(parentElement, documentText)

		if attrValues == nil {
			return []protocol.CompletionItem{}
		}

		if attrValues["type"] != "\"service\"" {
			return []protocol.CompletionItem{}
		}

		// Check if the parent element is a <service> element
		elementNameNode := treesitterhelper.GetFirstNodeOfKind(parentElement, "Name")
		if elementNameNode == nil {
			log.Printf("No element name found")
			return []protocol.CompletionItem{}
		}

		elementName := elementNameNode.Utf8Text([]byte(documentText))

		if elementName != "argument" {
			return []protocol.CompletionItem{}
		}
	} else {
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

// getTagCompletions returns tag completion items
func (p *SymfonyCompletionProvider) getTagCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	// Get all tags from the index
	tags := p.serviceIndex.GetAllTags()

	// Convert to completion items
	items := make([]protocol.CompletionItem, 0, len(tags))
	for _, tag := range tags {
		item := protocol.CompletionItem{
			Label: tag,
			Kind:  10, // 10 = Enum
		}

		// Add documentation
		services := p.serviceIndex.GetServicesByTag(tag)
		documentation := "Symfony service tag\n\n"

		// Add services information
		if len(services) > 0 {
			documentation += "**Services:**\n"
			for _, serviceID := range services {
				documentation += "- " + serviceID + "\n"
			}
		}

		item.Documentation.Kind = "markdown"
		item.Documentation.Value = documentation

		items = append(items, item)
	}

	return items
}

// GetTriggerCharacters returns the characters that trigger this completion provider
func (p *SymfonyCompletionProvider) GetTriggerCharacters() []string {
	switch p.compType {
	case ServiceCompletion:
		// Return all characters that might be part of a service ID
		return []string{
			"@", ".", "_", "-",
			"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m",
			"n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z",
			"A", "B", "C", "D", "E", "F", "G", "H", "I", "J", "K", "L", "M",
			"N", "O", "P", "Q", "R", "S", "T", "U", "V", "W", "X", "Y", "Z",
			"0", "1", "2", "3", "4", "5", "6", "7", "8", "9",
			"\"", // Also trigger on double quote for when the attribute is empty
		}
	case TagCompletion:
		return []string{"!"}
	default:
		return []string{}
	}
}
