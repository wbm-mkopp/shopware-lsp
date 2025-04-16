package symfony

import (
	"context"
	"log"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
)

// SymfonyCompletionProvider provides completions for Symfony services and tags
type SymfonyCompletionProvider struct {
	serviceIndex *ServiceIndex
	server       *lsp.Server
	phpIndex     *php.PHPIndex
}

// NewServiceCompletionProvider creates a new service completion provider
func NewServiceCompletionProvider(server *lsp.Server) *SymfonyCompletionProvider {
	symfonyIndexer, _ := server.GetIndexer("symfony.service")
	phpIndexer, _ := server.GetIndexer("php.index")

	return &SymfonyCompletionProvider{
		serviceIndex: symfonyIndexer.(*ServiceIndex),
		phpIndex:     phpIndexer.(*php.PHPIndex),
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

	if isArgumentServiceContext(params.Node, params.DocumentContent) {
		currentServiceId := getParentServiceId(params.Node, params.DocumentContent)

		// Get all services from the index
		serviceIDs := p.serviceIndex.GetAllServices()

		// Convert to completion items
		items := make([]protocol.CompletionItem, 0)
		for _, serviceID := range serviceIDs {
			if serviceID == currentServiceId {
				continue
			}

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

	if isArgumentTagContext(params.Node, params.DocumentContent) {
		items := make([]protocol.CompletionItem, 0)
		tags := p.serviceIndex.GetAllTags()
		for _, tag := range tags {
			item := protocol.CompletionItem{
				Label: tag,
				Kind:  6, // 6 = Class
			}
			items = append(items, item)
		}
		return items
	}

	if isServiceIdContext(params.Node, params.DocumentContent) {
		classNames := p.phpIndex.GetClassNames()

		items := make([]protocol.CompletionItem, 0)
		for _, className := range classNames {
			item := protocol.CompletionItem{
				Label: className,
				Kind:  6, // 6 = Class
			}
			items = append(items, item)
		}
		return items
	}

	return []protocol.CompletionItem{}
}

// GetTriggerCharacters returns the characters that trigger this completion provider
func (p *SymfonyCompletionProvider) GetTriggerCharacters() []string {
	return []string{"\""}
}
