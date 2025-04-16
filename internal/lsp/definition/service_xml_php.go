package definition

import (
	"context"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/shopware/shopware-lsp/internal/symfony"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type serviceXMLPHPDefinitionProvider struct {
	serviceIndex *symfony.ServiceIndex
	phpIndex     *php.PHPIndex
}

func NewServiceXMLPHPDefinitionProvider(server *lsp.Server) *serviceXMLPHPDefinitionProvider {
	serviceIndex, _ := server.GetIndexer("symfony.service")
	phpIndex, _ := server.GetIndexer("php.index")

	return &serviceXMLPHPDefinitionProvider{
		serviceIndex: serviceIndex.(*symfony.ServiceIndex),
		phpIndex:     phpIndex.(*php.PHPIndex),
	}
}

func (p *serviceXMLPHPDefinitionProvider) GetDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if !strings.HasSuffix(params.TextDocument.URI, ".xml") {
		return []protocol.Location{}
	}

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
