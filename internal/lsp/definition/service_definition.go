package definition

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/shopware/shopware-lsp/internal/symfony"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type serviceXMLDefinitionProvider struct {
	serviceIndex *symfony.ServiceIndex
	phpIndex     *php.PHPIndex
}

func NewServiceXMLDefinitionProvider(lsp *lsp.Server) *serviceXMLDefinitionProvider {
	serviceIndex, _ := lsp.GetIndexer("symfony.service")
	phpIndex, _ := lsp.GetIndexer("php.index")

	return &serviceXMLDefinitionProvider{
		serviceIndex: serviceIndex.(*symfony.ServiceIndex),
		phpIndex:     phpIndex.(*php.PHPIndex),
	}
}

func (p *serviceXMLDefinitionProvider) GetDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if params.Node == nil {
		return []protocol.Location{}
	}

	fileExt := strings.ToLower(filepath.Ext(params.TextDocument.URI))

	switch fileExt {
	case ".php":
		return p.phpDefinition(ctx, params)
	case ".yaml", ".yml":
		return p.yamlDefinition(ctx, params)
	case ".xml":
		return p.xmlDefinition(ctx, params)
	default:
		return []protocol.Location{}
	}
}

func (p *serviceXMLDefinitionProvider) xmlDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {

	// <argument type="service" id="<caret>"/>
	if treesitterhelper.SymfonyServiceIsServiceTag(params.Node, params.DocumentContent) {
		// Get the service ID at the current position
		serviceID := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)
		if serviceID == "" {
			return []protocol.Location{}
		}

		// Try to find the service definition
		service, found := p.serviceIndex.GetServiceByID(serviceID)
		if !found {
			return []protocol.Location{}
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

	// <argument type="tagged" tag="x"/>
	if treesitterhelper.SymfonyServiceIsArgumentTag(params.Node, params.DocumentContent) || treesitterhelper.SymfonyServiceIsTagElement(params.Node, params.DocumentContent) {
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

	// <argument>%<caret>%</argument>
	if treesitterhelper.SymfonyServiceIsParameterReference(params.Node, params.DocumentContent) {
		nodeText := params.Node.Utf8Text([]byte(params.DocumentContent))

		// Find parameter references in the format %parameter_name%
		// This is a simplistic approach - in a real implementation you would want to
		// be more precise about the cursor position to identify the exact parameter
		startIdx := strings.Index(nodeText, "%")
		if startIdx == -1 {
			return []protocol.Location{}
		}

		endIdx := strings.Index(nodeText[startIdx+1:], "%")
		if endIdx == -1 {
			return []protocol.Location{}
		}

		paramName := nodeText[startIdx+1 : startIdx+1+endIdx]

		// Find parameter locations
		// Currently we don't store line numbers for parameters, so we can't
		// provide exact locations. This would require enhancing the Parameter struct
		// to store line numbers and updating the parser.

		// Check if the parameter exists
		parameter, found := p.serviceIndex.GetParameterByName(paramName)
		if !found {
			return []protocol.Location{}
		}

		return []protocol.Location{
			{
				URI: "file://" + parameter.Path,
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      parameter.Line - 1, // LSP uses 0-based line numbers
						Character: 0,
					},
					End: protocol.Position{
						Line:      parameter.Line - 1,
						Character: 0,
					},
				},
			},
		}
	}

	// <service id="<caret>">
	if treesitterhelper.SymfonyServiceIsServiceId(params.Node, params.DocumentContent) {
		nodeText := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)

		phpClass := p.phpIndex.GetClass(nodeText)
		if phpClass != nil {
			return []protocol.Location{
				{
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
				},
			}
		}
	}

	return []protocol.Location{}
}

func (p *serviceXMLDefinitionProvider) yamlDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if treesitterhelper.IsYamlServiceId(params.Node, params.DocumentContent) || treesitterhelper.IsYamlClassPropertyInService().Matches(params.Node, params.DocumentContent) {
		value := treesitterhelper.GetYAMLValue(params.Node, params.DocumentContent)
		phpClass := p.phpIndex.GetClass(value)
		if phpClass != nil {
			return []protocol.Location{
				{
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
				},
			}
		}
	}

	if treesitterhelper.IsYamlArgumentServiceId(params.Node, params.DocumentContent) {
		value := strings.TrimPrefix(treesitterhelper.GetYAMLValue(params.Node, params.DocumentContent), "@")

		service, found := p.serviceIndex.GetServiceByID(value)

		if found {
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
	}

	return []protocol.Location{}
}

func (p *serviceXMLDefinitionProvider) phpDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if treesitterhelper.SymfonyServiceIsServiceId(params.Node, params.DocumentContent) {
		serviceID := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)
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
