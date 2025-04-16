package symfony

import (
	"context"
	"log"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
)

// SymfonyGotoDefinitionProvider provides goto definition for Symfony services
type SymfonyGotoDefinitionProvider struct {
	serviceIndex *ServiceIndex
	server       *lsp.Server
	phpIndex     *php.PHPIndex
}

// NewGotoDefinitionProvider creates a new goto definition provider for Symfony services
func NewGotoDefinitionProvider(server *lsp.Server) *SymfonyGotoDefinitionProvider {
	indexer, _ := server.GetIndexer("symfony.service")
	phpIndexer, _ := server.GetIndexer("php.index")

	return &SymfonyGotoDefinitionProvider{
		serviceIndex: indexer.(*ServiceIndex),
		phpIndex:     phpIndexer.(*php.PHPIndex),
		server:       server,
	}
}

// GetDefinition returns the location of the definition for a service ID
func (p *SymfonyGotoDefinitionProvider) GetDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	// Check if we're in an XML file
	uri := params.TextDocument.URI
	if !strings.HasSuffix(strings.ToLower(uri), ".xml") {
		log.Printf("Not providing definitions for non-XML file: %s", uri)
		return []protocol.Location{}
	}

	if params.Node == nil {
		return []protocol.Location{}
	}

	// Check if we're in a service ID context
	if isArgumentServiceContext(params.Node, params.DocumentContent) {
		// Get the service ID at the current position
		serviceID := getCurrentAttributeValue(params.Node, params.DocumentContent)
		if serviceID == "" {
			return []protocol.Location{}
		}

		// Try to find the service definition
		service, found := p.serviceIndex.GetServiceByID(serviceID)
		if !found {
			// Check if it's an alias
			alias, aliasFound := p.serviceIndex.GetAliasByID(serviceID)
			if !aliasFound {
				return []protocol.Location{}
			}

			// Create a location for the alias
			return []protocol.Location{
				{
					URI: "file://" + alias.Path,
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      alias.Line - 1, // LSP uses 0-based line numbers
							Character: 0,
						},
						End: protocol.Position{
							Line:      alias.Line - 1,
							Character: 0,
						},
					},
				},
			}
		}

		// Create a location for the service
		return []protocol.Location{
			{
				URI: "file://" + service.Path,
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      service.Line - 1, // LSP uses 0-based line numbers
						Character: 0,
					},
					End: protocol.Position{
						Line:      service.Line - 1,
						Character: 0,
					},
				},
			},
		}
	}

	if isArgumentTagContext(params.Node, params.DocumentContent) {
		serviceID := getCurrentAttributeValue(params.Node, params.DocumentContent)
		if serviceID == "" {
			return []protocol.Location{}
		}

		services := p.serviceIndex.GetServicesByTag(serviceID)

		var locations []protocol.Location
		for _, serviceName := range services {
			service, found := p.serviceIndex.GetServiceByID(serviceName)
			if !found {
				continue
			}

			locations = append(locations, protocol.Location{
				URI: "file://" + service.Path,
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      service.Line - 1, // LSP uses 0-based line numbers
						Character: 0,
					},
					End: protocol.Position{
						Line:      service.Line - 1,
						Character: 0,
					},
				},
			})
		}

		return locations
	}

	if isServiceIdContext(params.Node, params.DocumentContent) {
		serviceID := getCurrentAttributeValue(params.Node, params.DocumentContent)
		if serviceID == "" {
			return []protocol.Location{}
		}

		var locations []protocol.Location

		phpClass := p.phpIndex.GetClass(serviceID)

		if phpClass != nil {
			locations = append(locations, protocol.Location{
				URI: "file://" + phpClass.Path,
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      phpClass.Line - 1, // LSP uses 0-based line numbers
						Character: 0,
					},
					End: protocol.Position{
						Line:      phpClass.Line - 1,
						Character: 0,
					},
				},
			})
		}

		return locations
	}

	return []protocol.Location{}
}
