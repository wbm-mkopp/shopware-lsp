package codelens

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/twig"
)

type TwigCodeLensProvider struct {
	twigIndexer *twig.TwigIndexer
	lspServer   *lsp.Server
}

func NewTwigCodeLensProvider(lspServer *lsp.Server) *TwigCodeLensProvider {
	twigIndexer, _ := lspServer.GetIndexer("twig.indexer")
	return &TwigCodeLensProvider{
		twigIndexer: twigIndexer.(*twig.TwigIndexer),
		lspServer:   lspServer,
	}
}

func (p *TwigCodeLensProvider) GetCodeLenses(ctx context.Context, params *protocol.CodeLensParams) []protocol.CodeLens {
	if !strings.HasSuffix(strings.ToLower(params.TextDocument.URI), ".twig") {
		return []protocol.CodeLens{}
	}

	document, _ := p.lspServer.DocumentManager().GetDocument(params.TextDocument.URI)

	if document == nil || document.Tree == nil {
		return []protocol.CodeLens{}
	}

	twigFile, _ := twig.ParseTwig(strings.TrimPrefix(params.TextDocument.URI, "file://"), document.Tree.RootNode(), []byte(document.Text))

	if twigFile == nil {
		return []protocol.CodeLens{}
	}

	if twigFile.ExtendsFile == "" {
		return []protocol.CodeLens{}
	}

	extendedFiles, _ := p.twigIndexer.GetTwigFilesByRelPath(twigFile.ExtendsFile)

	if len(extendedFiles) == 0 {
		return []protocol.CodeLens{}
	}

	var lenses []protocol.CodeLens

	for _, block := range twigFile.Blocks {
		parentBlock, ok := extendedFiles[0].Blocks[block.Name]

		if !ok {
			continue
		}

		lenses = append(lenses, protocol.CodeLens{
			Range: protocol.Range{
				Start: protocol.Position{
					Line:      block.Line - 1,
					Character: 0,
				},
				End: protocol.Position{
					Line:      block.Line - 1,
					Character: 0,
				},
			},
			Command: &protocol.Command{
				Title:     "Goto Parent Block",
				Command:   "vscode.open",
				Arguments: []any{fmt.Sprintf("file://%s#%d", extendedFiles[0].Path, parentBlock.Line)},
			},
		})
	}

	return lenses
}

func (p *TwigCodeLensProvider) ResolveCodeLens(ctx context.Context, codeLens *protocol.CodeLens) (*protocol.CodeLens, error) {
	return codeLens, nil
}
