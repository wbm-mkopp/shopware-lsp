package definition

import (
	"context"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/symfony"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type serviceXMLDefinitionProvider struct {
	serviceIndex *symfony.ServiceIndex
}

func NewServiceXMLDefinitionProvider(lsp *lsp.Server) *serviceXMLDefinitionProvider {
	serviceIndex, _ := lsp.GetIndexer("symfony.service")

	return &serviceXMLDefinitionProvider{
		serviceIndex: serviceIndex.(*symfony.ServiceIndex),
	}
}

func (p *serviceXMLDefinitionProvider) GetDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if !strings.HasSuffix(params.TextDocument.URI, ".xml") {
		return []protocol.Location{}
	}

	if params.Node == nil {
		return []protocol.Location{}
	}

	// Check if we're in a service ID context
	if treesitterhelper.SymfonyServiceIsServiceTag(params.Node, params.DocumentContent) {
		// Get the service ID at the current position
		serviceID := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)
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

	if treesitterhelper.SymfonyServiceIsArgumentTag(params.Node, params.DocumentContent) {
		serviceID := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)
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

	return []protocol.Location{}
}
