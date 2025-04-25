package completion

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/shopware/shopware-lsp/internal/symfony"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

// SymfonyCompletionProvider provides completions for Symfony services and tags
type SymfonyCompletionProvider struct {
	serviceIndex *symfony.ServiceIndex
	server       *lsp.Server
	phpIndex     *php.PHPIndex
}

// NewServiceCompletionProvider creates a new service completion provider
func NewServiceCompletionProvider(server *lsp.Server) *SymfonyCompletionProvider {
	symfonyIndexer, _ := server.GetIndexer("symfony.service")
	phpIndexer, _ := server.GetIndexer("php.index")

	return &SymfonyCompletionProvider{
		serviceIndex: symfonyIndexer.(*symfony.ServiceIndex),
		phpIndex:     phpIndexer.(*php.PHPIndex),
		server:       server,
	}
}

// GetCompletions returns completion items based on the provider type
func (p *SymfonyCompletionProvider) GetCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if params.Node == nil {
		return []protocol.CompletionItem{}
	}

	fileExt := strings.ToLower(filepath.Ext(params.TextDocument.URI))

	switch fileExt {
	case ".yaml", ".yml":
		return p.yamlCompletions(ctx, params)
	case ".xml":
		return p.xmlCompletion(ctx, params)
	default:
		return []protocol.CompletionItem{}
	}
}

func (p *SymfonyCompletionProvider) xmlCompletion(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {

	// Check if we're in an XML file
	uri := params.TextDocument.URI
	if !strings.HasSuffix(strings.ToLower(uri), ".xml") {
		return []protocol.CompletionItem{}
	}

	// <argument type="service" id="<caret>"/>
	if treesitterhelper.SymfonyServiceIsServiceTag(params.Node, params.DocumentContent) {
		currentServiceId := treesitterhelper.SymfonyGetCurrentServiceIdFromArgument(params.Node, params.DocumentContent)

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

	// <argument type="tagged" tag="<caret>"/>
	if treesitterhelper.SymfonyServiceIsArgumentTag(params.Node, params.DocumentContent) {
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

	// <argument>%<caret>%</argument>
	if treesitterhelper.SymfonyServiceIsParameterReference(params.Node, params.DocumentContent) {
		items := make([]protocol.CompletionItem, 0)
		parameters := p.serviceIndex.GetAllParameters()

		for _, paramName := range parameters {
			item := protocol.CompletionItem{
				Label:      paramName.Name,
				InsertText: paramName.Name + "%",
				Kind:       21, // 21 = Constant
			}

			// Try to get parameter value for documentation
			if value, found := p.serviceIndex.GetParameterByName(paramName.Name); found {
				item.Documentation.Kind = "markdown"
				item.Documentation.Value = "**Parameter:** `" + paramName.Name + "`\n\n**Value:** `" + value.Value + "`"
			}

			items = append(items, item)
		}
		return items
	}

	// <tag name="<caret>"/>
	if treesitterhelper.SymfonyServiceIsTagElement(params.Node, params.DocumentContent) {
		tags := p.serviceIndex.GetAllTags()
		items := make([]protocol.CompletionItem, 0)
		for _, tag := range tags {
			item := protocol.CompletionItem{
				Label: tag,
				Kind:  6, // 6 = Class
			}
			items = append(items, item)
		}

		return items
	}

	// <service id="<caret>">
	if treesitterhelper.SymfonyServiceIsServiceId(params.Node, params.DocumentContent) {
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

func (p *SymfonyCompletionProvider) yamlCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if treesitterhelper.IsYamlServiceId(params.Node, params.DocumentContent) || treesitterhelper.IsYamlClassPropertyInServiceToType().Matches(params.Node, params.DocumentContent) {
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

	if treesitterhelper.IsYamlArgumentServiceId(params.Node, params.DocumentContent) {
		// Get all services from the index
		serviceIDs := p.serviceIndex.GetAllServices()

		// Convert to completion items
		items := make([]protocol.CompletionItem, 0)
		for _, serviceID := range serviceIDs {
			item := protocol.CompletionItem{
				Label:      serviceID,
				Kind:       6, // 6 = Class
				InsertText: fmt.Sprintf("@%s", serviceID),
			}
			items = append(items, item)
		}

		return items
	}

	return []protocol.CompletionItem{}
}

// GetTriggerCharacters returns the characters that trigger this completion provider
func (p *SymfonyCompletionProvider) GetTriggerCharacters() []string {
	return []string{"\"", "%"}
}
