package definition

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/extension"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/theme"
	"github.com/shopware/shopware-lsp/internal/twig"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type TwigDefinitionProvider struct {
	twigIndexer  *twig.TwigIndexer
	iconProvider *theme.IconProvider
}

func NewTwigDefinitionProvider(projectRoot string, lspServer *lsp.Server) *TwigDefinitionProvider {
	twigIndexer, _ := lspServer.GetIndexer("twig.indexer")
	extensionIndexer, _ := lspServer.GetIndexer("extension.indexer")

	iconProvider := theme.NewIconProvider(projectRoot, extensionIndexer.(*extension.ExtensionIndexer))

	return &TwigDefinitionProvider{
		twigIndexer:  twigIndexer.(*twig.TwigIndexer),
		iconProvider: iconProvider,
	}
}

func (p *TwigDefinitionProvider) GetDefinition(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if params.Node == nil {
		return []protocol.Location{}
	}

	switch strings.ToLower(filepath.Ext(params.TextDocument.URI)) {
	case ".php":
		return p.phpDefinitions(ctx, params)
	case ".twig":
		return p.twigDefinitions(ctx, params)
	default:
		return []protocol.Location{}
	}
}

func (p *TwigDefinitionProvider) twigDefinitions(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if treesitterhelper.TwigStringInTagPattern("extends", "sw_extends", "include", "sw_include").Matches(params.Node, []byte(params.DocumentContent)) {
		itemValue := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)

		files, _ := p.twigIndexer.GetTwigFilesByRelPath(itemValue)

		var locations []protocol.Location
		for _, file := range files {
			if file.Path == strings.TrimPrefix(params.TextDocument.URI, lsp.FileURIPrefix) {
				continue
			}

			locations = append(locations, protocol.Location{
				URI: fmt.Sprintf(lsp.FileURIFormat, file.Path),
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      0,
						Character: 0,
					},
					End: protocol.Position{
						Line:      0,
						Character: 0,
					},
				},
			})
		}

		return locations
	}

	if params.Node.Kind() == "function" {
		functionName := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)
		parentNode := params.Node.Parent()

		if parentNode != nil && parentNode.Kind() == "filter_expression" {
			filters, _ := p.twigIndexer.GetTwigFilter(functionName)

			var locations []protocol.Location
			for _, filter := range filters {
				locations = append(locations, protocol.Location{
					URI: fmt.Sprintf(lsp.FileURIFormat, filter.FilePath),
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      int(filter.Line) - 1,
							Character: 0,
						},
						End: protocol.Position{
							Line:      int(filter.Line) - 1,
							Character: 0,
						},
					},
				})
			}

			return locations

		} else {
			functions, _ := p.twigIndexer.GetTwigFunction(functionName)

			var locations []protocol.Location
			for _, function := range functions {
				locations = append(locations, protocol.Location{
					URI: fmt.Sprintf(lsp.FileURIFormat, function.FilePath),
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      int(function.Line) - 1,
							Character: 0,
						},
						End: protocol.Position{
							Line:      int(function.Line) - 1,
							Character: 0,
						},
					},
				})
			}

			return locations
		}
	}

	// Go-to-definition for block names: navigate to the parent block.
	// The identifier may be inside a proper "block" node, or inside an "ERROR" node
	// when the tree-sitter grammar fails to parse blocks containing HTML tags.
	if treesitterhelper.IsTwigBlockIdentifier(params.Node, params.DocumentContent) {
		blockName := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)

		// Walk up to get root node for parsing
		root := params.Node
		for root.Parent() != nil {
			root = root.Parent()
		}

		filePath := strings.TrimPrefix(params.TextDocument.URI, lsp.FileURIPrefix)
		twigFile, err := twig.ParseTwig(filePath, root, params.DocumentContent)
		if err != nil || twigFile == nil || twigFile.ExtendsFile == "" {
			return []protocol.Location{}
		}

		// Strategy: Use the block hash index to find the original Storefront
		// template where this block is actually defined. The block may live in
		// a completely different template than the one referenced by ExtendsFile
		// (e.g. block defined in box-standard.html.twig but overridden in
		// price-unit.html.twig).
		if loc := p.findStorefrontBlockLocation(blockName); loc != nil {
			return []protocol.Location{*loc}
		}

		// Fallback: search files that share the same relPath as ExtendsFile.
		// This handles cases where no block hash entry exists (non-Storefront
		// blocks, or blocks that haven't been indexed yet).
		parentFiles, _ := p.twigIndexer.GetTwigFilesByRelPath(twigFile.ExtendsFile)

		var otherLocation *protocol.Location

		for _, parentFile := range parentFiles {
			if parentFile.Path == filePath {
				continue
			}

			if parentBlock, ok := parentFile.Blocks[blockName]; ok {
				loc := protocol.Location{
					URI: fmt.Sprintf(lsp.FileURIFormat, parentFile.Path),
					Range: protocol.Range{
						Start: protocol.Position{Line: parentBlock.Line - 1, Character: 0},
						End:   protocol.Position{Line: parentBlock.Line - 1, Character: 0},
					},
				}
				if twig.IsStorefrontTemplate(parentFile.Path) {
					return []protocol.Location{loc}
				}
				if otherLocation == nil {
					otherLocation = &loc
				}
			}
		}

		if otherLocation != nil {
			return []protocol.Location{*otherLocation}
		}

		return []protocol.Location{}
	}

	if treesitterhelper.TwigStringInTagPattern("sw_icon").Matches(params.Node, []byte(params.DocumentContent)) {
		text := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)

		cfg := treesitterhelper.ExtractSwIconObjectToMap(params.Node.Parent(), params.DocumentContent)

		pack, ok := cfg["pack"]
		if !ok {
			pack = "default"
		}

		icon := p.iconProvider.GetIcon(pack, text)

		if icon != "" {
			locations := []protocol.Location{
				{
					URI: fmt.Sprintf(lsp.FileURIFormat, icon),
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      0,
							Character: 0,
						},
						End: protocol.Position{
							Line:      0,
							Character: 0,
						},
					},
				},
			}

			return locations
		}

	}

	return []protocol.Location{}
}

// findStorefrontBlockLocation uses the block hash index to locate the original
// Storefront template where a block is defined. This is necessary because a block
// may be defined in a different template than the one referenced by ExtendsFile
// (e.g. component_product_box_price is defined in box-standard.html.twig but
// overridden in price-unit.html.twig).
func (p *TwigDefinitionProvider) findStorefrontBlockLocation(blockName string) *protocol.Location {
	blockHashes, err := p.twigIndexer.GetTwigBlockHashes(blockName)
	if err != nil || len(blockHashes) == 0 {
		return nil
	}

	originalHash := twig.FindOriginalStorefrontHash(blockHashes)
	if originalHash == nil {
		return nil
	}

	// Look up the actual TwigFile to get the block's exact line number.
	files, _ := p.twigIndexer.GetTwigFilesByRelPath(originalHash.RelativePath)
	for _, f := range files {
		if f.Path == originalHash.AbsolutePath {
			if block, ok := f.Blocks[blockName]; ok {
				return &protocol.Location{
					URI: fmt.Sprintf(lsp.FileURIFormat, f.Path),
					Range: protocol.Range{
						Start: protocol.Position{Line: block.Line - 1, Character: 0},
						End:   protocol.Position{Line: block.Line - 1, Character: 0},
					},
				}
			}
			break
		}
	}

	// File found in hash but block not in Blocks map (parser issue) — fall back
	// to file start position.
	return &protocol.Location{
		URI: fmt.Sprintf(lsp.FileURIFormat, originalHash.AbsolutePath),
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: 0, Character: 0},
		},
	}
}

func (p *TwigDefinitionProvider) phpDefinitions(ctx context.Context, params *protocol.DefinitionParams) []protocol.Location {
	if treesitterhelper.IsPHPThisMethodCall("renderStorefront").Matches(params.Node, params.DocumentContent) {
		files, _ := p.twigIndexer.GetTwigFilesByRelPath(treesitterhelper.GetNodeText(params.Node, params.DocumentContent))

		var locations []protocol.Location
		for _, file := range files {
			locations = append(locations, protocol.Location{
				URI: fmt.Sprintf(lsp.FileURIFormat, file.Path),
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      0,
						Character: 0,
					},
					End: protocol.Position{
						Line:      0,
						Character: 0,
					},
				},
			})
		}

		return locations
	}

	return []protocol.Location{}
}
