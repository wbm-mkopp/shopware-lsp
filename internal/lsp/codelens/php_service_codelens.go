package codelens

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/shopware/shopware-lsp/internal/symfony"
)

type PHPServiceCodelensProvider struct {
	phpIndex     *php.PHPIndex
	serviceIndex *symfony.ServiceIndex
}

func NewPHPCodeLensProvider(lsp *lsp.Server) *PHPServiceCodelensProvider {
	phpIndex, _ := lsp.GetIndexer("php.index")
	serviceIndex, _ := lsp.GetIndexer("symfony.service")

	return &PHPServiceCodelensProvider{
		phpIndex:     phpIndex.(*php.PHPIndex),
		serviceIndex: serviceIndex.(*symfony.ServiceIndex),
	}
}

func (p *PHPServiceCodelensProvider) GetCodeLenses(ctx context.Context, params *protocol.CodeLensParams) []protocol.CodeLens {
	if !strings.HasSuffix(params.TextDocument.URI, ".php") {
		return []protocol.CodeLens{}
	}

	phpClasses := p.phpIndex.GetClassesOfFile(strings.TrimPrefix(params.TextDocument.URI, "file://"))

	if len(phpClasses) == 0 {
		return []protocol.CodeLens{}
	}

	var lenses []protocol.CodeLens

	for _, phpClass := range phpClasses {
		locations := p.serviceIndex.GetServicesUsageByClassName(phpClass.Name)

		if len(locations) == 0 {
			continue
		}

		var fileLocations []string
		for _, location := range locations {
			fileLocations = append(fileLocations, fmt.Sprintf("file://%s#%d", location.Path, location.Line))
		}

		lenses = append(lenses, protocol.CodeLens{
			Command: &protocol.Command{
				Title:   "Open Service Definition",
				Command: "shopware.openReferences",
				Arguments: []any{
					fileLocations,
				},
			},
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      phpClass.Line - 1,
					Character: 0,
				},
				End: protocol.Position{
					Line:      phpClass.Line - 1,
					Character: 0,
				},
			},
		})
	}

	return lenses
}

func (p *PHPServiceCodelensProvider) ResolveCodeLens(ctx context.Context, params *protocol.CodeLens) (*protocol.CodeLens, error) {
	return params, nil
}
