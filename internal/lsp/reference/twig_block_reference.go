package reference

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/twig"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

// TwigBlockReferenceProvider finds all references to a Twig block,
// including the original Storefront block and blocks in other plugins/extensions.
type TwigBlockReferenceProvider struct {
	twigIndexer *twig.TwigIndexer
}

func NewTwigBlockReferenceProvider(lspServer *lsp.Server) *TwigBlockReferenceProvider {
	twigIndexer, _ := lspServer.GetIndexer("twig.indexer")
	return &TwigBlockReferenceProvider{
		twigIndexer: twigIndexer.(*twig.TwigIndexer),
	}
}

func (p *TwigBlockReferenceProvider) GetReferences(ctx context.Context, params *protocol.ReferenceParams) []protocol.Location {
	if params.Node == nil {
		return nil
	}

	if strings.ToLower(filepath.Ext(params.TextDocument.URI)) != ".twig" {
		return nil
	}

	if !treesitterhelper.IsTwigBlockIdentifier(params.Node, params.DocumentContent) {
		return nil
	}

	blockName := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)

	// Walk up to get root node for parsing
	root := params.Node
	for root.Parent() != nil {
		root = root.Parent()
	}

	filePath := strings.TrimPrefix(params.TextDocument.URI, lsp.FileURIPrefix)
	twigFile, err := twig.ParseTwig(filePath, root, params.DocumentContent)
	if err != nil || twigFile == nil {
		return nil
	}

	// Determine the relPath to search: use ExtendsFile if available (current file
	// is an extension), otherwise use the file's own RelPath (current file is the
	// original or standalone template).
	searchRelPath := twigFile.RelPath
	if twigFile.ExtendsFile != "" {
		searchRelPath = twigFile.ExtendsFile
	}

	allFiles, _ := p.twigIndexer.GetTwigFilesByRelPath(searchRelPath)

	// Track included paths to avoid duplicates when the original Storefront
	// template lives under a different relPath than the ExtendsFile.
	includedPaths := make(map[string]bool)
	includedPaths[filePath] = true // exclude the current file

	var locations []protocol.Location
	for _, file := range allFiles {
		if includedPaths[file.Path] {
			continue
		}
		includedPaths[file.Path] = true

		if block, ok := file.Blocks[blockName]; ok {
			locations = append(locations, protocol.Location{
				URI: fmt.Sprintf(lsp.FileURIFormat, file.Path),
				Range: protocol.Range{
					Start: protocol.Position{Line: block.Line - 1, Character: 0},
					End:   protocol.Position{Line: block.Line - 1, Character: 0},
				},
			})
		}
	}

	// Also include the original Storefront template where the block is defined,
	// which may live under a different relPath (e.g. block defined in
	// box-standard.html.twig but overridden in price-unit.html.twig).
	blockHashes, _ := p.twigIndexer.GetTwigBlockHashes(blockName)
	originalHash := twig.FindOriginalStorefrontHash(blockHashes)

	if originalHash != nil && !includedPaths[originalHash.AbsolutePath] {
		includedPaths[originalHash.AbsolutePath] = true

		// Try to get the exact line from the TwigFile index.
		var added bool
		files, _ := p.twigIndexer.GetTwigFilesByRelPath(originalHash.RelativePath)
		for _, f := range files {
			if f.Path == originalHash.AbsolutePath {
				if block, ok := f.Blocks[blockName]; ok {
					locations = append(locations, protocol.Location{
						URI: fmt.Sprintf(lsp.FileURIFormat, f.Path),
						Range: protocol.Range{
							Start: protocol.Position{Line: block.Line - 1, Character: 0},
							End:   protocol.Position{Line: block.Line - 1, Character: 0},
						},
					})
					added = true
				}
				break
			}
		}

		if !added {
			// Fallback: file-start position when blocks weren't parsed.
			locations = append(locations, protocol.Location{
				URI: fmt.Sprintf(lsp.FileURIFormat, originalHash.AbsolutePath),
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 0},
					End:   protocol.Position{Line: 0, Character: 0},
				},
			})
		}
	}

	return locations
}
