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

	twigFile, _ := twig.ParseTwig(strings.TrimPrefix(params.TextDocument.URI, "file://"), document.Tree.RootNode(), document.Text)

	if twigFile == nil || twigFile.ExtendsFile == "" {
		return []protocol.CodeLens{}
	}

	if twigFile.ExtendsFile != twigFile.RelPath {
		allOtherFiles, _ := p.twigIndexer.GetTwigFilesByRelPath(twigFile.RelPath)

		blockOverwrites := make(map[string][]string)

		for _, file := range allOtherFiles {
			if file.Path == twigFile.Path {
				continue
			}

			for _, block := range file.Blocks {
				if _, ok := blockOverwrites[block.Name]; !ok {
					blockOverwrites[block.Name] = []string{}
				}

				blockOverwrites[block.Name] = append(blockOverwrites[block.Name], fmt.Sprintf("file://%s#%d", file.Path, block.Line))
			}
		}

		var lenses []protocol.CodeLens

		for _, block := range twigFile.Blocks {
			if len(blockOverwrites[block.Name]) > 0 {
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
						Title:     fmt.Sprintf("%d block overwrites", len(blockOverwrites[block.Name])),
						Command:   "shopware.openReferences",
						Arguments: []any{blockOverwrites[block.Name]},
					},
				})
			}
		}

		return lenses
	}

	extendedFiles, _ := p.twigIndexer.GetTwigFilesByRelPath(twigFile.ExtendsFile)

	var parentFile *twig.TwigFile
	for i := range extendedFiles {
		if extendedFiles[i].Path != twigFile.Path {
			parentFile = &extendedFiles[i]
			break
		}
	}

	if parentFile == nil {
		return []protocol.CodeLens{}
	}

	var lenses []protocol.CodeLens

	for _, block := range twigFile.Blocks {
		parentBlock, ok := parentFile.Blocks[block.Name]

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
				Arguments: []any{fmt.Sprintf("file://%s#%d", parentFile.Path, parentBlock.Line)},
			},
		})
	}

	return lenses
}

func (p *TwigCodeLensProvider) ResolveCodeLens(ctx context.Context, codeLens *protocol.CodeLens) (*protocol.CodeLens, error) {
	return codeLens, nil
}
