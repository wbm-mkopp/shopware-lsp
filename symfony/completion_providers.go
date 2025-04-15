package symfony

import (
	"context"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
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
	compType     CompletionType
}

// NewServiceCompletionProvider creates a new service completion provider
func NewServiceCompletionProvider(serviceIndex *ServiceIndex) *SymfonyCompletionProvider {
	return &SymfonyCompletionProvider{
		serviceIndex: serviceIndex,
		compType:     ServiceCompletion,
	}
}

// NewTagCompletionProvider creates a new tag completion provider
func NewTagCompletionProvider(serviceIndex *ServiceIndex) *SymfonyCompletionProvider {
	return &SymfonyCompletionProvider{
		serviceIndex: serviceIndex,
		compType:     TagCompletion,
	}
}

// GetCompletions returns completion items based on the provider type
func (p *SymfonyCompletionProvider) GetCompletions(ctx context.Context, params *protocol.CompletionParams) []protocol.CompletionItem {
	if p.serviceIndex == nil {
		return []protocol.CompletionItem{}
	}

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
		return []string{"@", "'", "\""}
	case TagCompletion:
		return []string{"!", "#"}
	default:
		return []string{}
	}
}

// Index builds or updates the provider's index
func (p *SymfonyCompletionProvider) Index() error {
	if p.serviceIndex == nil {
		return nil
	}

	// Both service and tag completions use the same index
	return p.serviceIndex.BuildIndex()
}
