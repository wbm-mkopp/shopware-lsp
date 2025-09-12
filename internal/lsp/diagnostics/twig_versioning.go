package diagnostics

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

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
	// Only process Twig files
	if filepath.Ext(uri) != ".twig" {
		return []protocol.Diagnostic{}, nil
	}

	if p.twigIndexer == nil {
		return []protocol.Diagnostic{}, nil
	}

	// Skip Storefront templates - they don't need versioning comments
	if p.isStorefrontTemplate(uri) {
		return []protocol.Diagnostic{}, nil
	}

	// Parse the current file directly to get its blocks
	currentFile, err := twig.ParseTwig(uri, rootNode, content)
	if err != nil {
		return nil, err
	}

	var diagnostics []protocol.Diagnostic

	// Only check blocks that exist in the current file
	for _, block := range currentFile.Blocks {
		// Check if this block has a version comment
		if block.VersionComment != nil {
			// Look up the original block hash from the storefront templates
			allBlockHashes, err := p.twigIndexer.GetTwigBlockHashes(block.Name)
			if err != nil {
				continue
			}

			// Find the original Storefront template block hash
			var originalHash *twig.TwigBlockHash
			for _, hash := range allBlockHashes {
				if strings.Contains(hash.RelativePath, "storefront/") {
					originalHash = &hash
					break
				}
			}

			if originalHash == nil {
				// No original block found - could be missing versioning comment
				if !p.isStorefrontTemplate(uri) {
					diagnostics = append(diagnostics, protocol.Diagnostic{
						Range: protocol.Range{
							Start: protocol.Position{Line: int(block.Line - 1), Character: 0},
							End:   protocol.Position{Line: int(block.Line - 1), Character: 100},
						},
						Severity: protocol.DiagnosticSeverityWarning,
						Source:   "shopware-lsp",
						Message:  fmt.Sprintf("The block '%s' does not have a versioning comment", block.Name),
					})
				}
				continue
			}

			// Check if the hash in the comment matches the original hash
			if originalHash.Hash != block.VersionComment.Hash {
				diagnostics = append(diagnostics, protocol.Diagnostic{
					Range: protocol.Range{
						Start: protocol.Position{Line: int(block.VersionComment.Line - 1), Character: 0},
						End:   protocol.Position{Line: int(block.VersionComment.Line - 1), Character: 100},
				},
					Severity: protocol.DiagnosticSeverityWarning,
					Source:   "shopware-lsp",
					Message:  "The upstream block has been changed, please update the block",
				})
			}
		} else {
			// No version comment present, check if one should be there
			allBlockHashesForCheck, err := p.twigIndexer.GetTwigBlockHashes(block.Name)
			if err != nil {
				continue
			}

			// Find the original Storefront template block hash
			var originalHashForCheck *twig.TwigBlockHash
			for _, hash := range allBlockHashesForCheck {
				if strings.Contains(hash.RelativePath, "storefront/") {
					originalHashForCheck = &hash
					break
				}
			}

			// If we have an original block and this is not a storefront template, suggest adding version comment
			if originalHashForCheck != nil && !p.isStorefrontTemplate(uri) {
				diagnostics = append(diagnostics, protocol.Diagnostic{
					Range: protocol.Range{
						Start: protocol.Position{Line: int(block.Line - 1), Character: 0},
						End:   protocol.Position{Line: int(block.Line - 1), Character: 100},
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

// isStorefrontTemplate checks if the URI is a Storefront template (where versioning comments are not needed)
func (p *TwigVersioningDiagnosticsProvider) isStorefrontTemplate(uri string) bool {
	return strings.Contains(uri, "src/Storefront/Resources/views/storefront") ||
		   strings.Contains(uri, "vendor/shopware/storefront/Resources/views/storefront")
}
