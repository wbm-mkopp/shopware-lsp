package diagnostics

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/twig"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type TwigVersioningDiagnosticsProvider struct {
	twigIndexer *twig.TwigIndexer
}

func NewTwigVersioningDiagnosticsProvider(lspServer *lsp.Server) *TwigVersioningDiagnosticsProvider {
	twigIndexer, _ := lspServer.GetIndexer("twig.indexer")
	return &TwigVersioningDiagnosticsProvider{
		twigIndexer: twigIndexer.(*twig.TwigIndexer),
	}
}

func (p *TwigVersioningDiagnosticsProvider) GetDiagnostics(ctx context.Context, uri string, rootNode *tree_sitter.Node, content []byte) ([]protocol.Diagnostic, error) {
	if filepath.Ext(uri) != ".twig" {
		return []protocol.Diagnostic{}, nil
	}

	if p.twigIndexer == nil {
		return []protocol.Diagnostic{}, nil
	}

	if twig.IsStorefrontTemplate(uri) {
		return []protocol.Diagnostic{}, nil
	}

	currentFile, err := twig.ParseTwig(uri, rootNode, content)
	if err != nil {
		return nil, err
	}

	var diagnostics []protocol.Diagnostic

	for _, block := range currentFile.Blocks {
		if block.VersionComment != nil {
			allBlockHashes, err := p.twigIndexer.GetTwigBlockHashes(block.Name)
			if err != nil {
				continue
			}

			originalHash := twig.FindOriginalStorefrontHashForExtends(allBlockHashes, currentFile.ExtendsFile)
			if originalHash == nil {
				diagnostics = append(diagnostics, protocol.Diagnostic{
					Range: protocol.Range{
						Start: protocol.Position{Line: block.Line - 1, Character: 0},
						End:   protocol.Position{Line: block.Line - 1, Character: 100},
					},
					Severity: protocol.DiagnosticSeverityWarning,
					Source:   "shopware-lsp",
					Message:  fmt.Sprintf("The block '%s' does not have a versioning comment", block.Name),
				})
				continue
			}

			if originalHash.Hash != block.VersionComment.Hash {
				diagnostics = append(diagnostics, protocol.Diagnostic{
					Range: protocol.Range{
						Start: protocol.Position{Line: block.VersionComment.Line - 1, Character: 0},
						End:   protocol.Position{Line: block.VersionComment.Line - 1, Character: 100},
					},
					Severity: protocol.DiagnosticSeverityWarning,
					Source:   "shopware-lsp",
					Message:  fmt.Sprintf("The upstream block has been changed, please update the block (expected: %s, got: %s, source: %s)", truncateHash(originalHash.Hash, 12), truncateHash(block.VersionComment.Hash, 12), originalHash.RelativePath),
				})
			}
		} else {
			allBlockHashes, err := p.twigIndexer.GetTwigBlockHashes(block.Name)
			if err != nil {
				continue
			}

			originalHash := twig.FindOriginalStorefrontHashForExtends(allBlockHashes, currentFile.ExtendsFile)
			if originalHash != nil {
				diagnostics = append(diagnostics, protocol.Diagnostic{
					Range: protocol.Range{
						Start: protocol.Position{Line: block.Line - 1, Character: 0},
						End:   protocol.Position{Line: block.Line - 1, Character: 100},
					},
					Severity: protocol.DiagnosticSeverityWarning,
					Source:   "shopware-lsp",
					Message:  fmt.Sprintf("The block '%s' does not have a versioning comment", block.Name),
				})
			}
		}
	}

	return diagnostics, nil
}

func truncateHash(hash string, length int) string {
	if len(hash) <= length {
		return hash
	}
	return hash[:length]
}
