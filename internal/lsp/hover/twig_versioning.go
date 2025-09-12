package hover

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/twig"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type TwigVersioningHoverProvider struct {
	twigIndexer *twig.TwigIndexer
}

func NewTwigVersioningHoverProvider(lspServer *lsp.Server) *TwigVersioningHoverProvider {
	twigIndexer, _ := lspServer.GetIndexer("twig.indexer")
	return &TwigVersioningHoverProvider{
		twigIndexer: twigIndexer.(*twig.TwigIndexer),
	}
}

func (p *TwigVersioningHoverProvider) GetHover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	// Only process Twig files
	if !strings.HasSuffix(strings.ToLower(params.TextDocument.URI), ".twig") {
		return nil, nil
	}

	if params.Node == nil {
		return nil, nil
	}

	if p.twigIndexer == nil {
		return nil, nil
	}

	// Check if we're hovering over a block identifier or a version comment
	if p.isBlockIdentifier(params.Node, string(params.DocumentContent)) {
		return p.hoverBlockIdentifier(params.Node, string(params.DocumentContent), params.TextDocument.URI)
	}

	if p.isVersionComment(params.Node, string(params.DocumentContent)) {
		return p.hoverVersionComment(params.Node, string(params.DocumentContent), params.TextDocument.URI)
	}

	return nil, nil
}

// isBlockIdentifier checks if the target node is a block identifier
func (p *TwigVersioningHoverProvider) isBlockIdentifier(node *tree_sitter.Node, content string) bool {
	if node.Kind() != "identifier" {
		return false
	}

	// Check if the parent is a block
	parent := node.Parent()
	return parent != nil && parent.Kind() == "block"
}

// isVersionComment checks if the target node is part of a version comment
func (p *TwigVersioningHoverProvider) isVersionComment(node *tree_sitter.Node, content string) bool {
	if node.Kind() != "comment" {
		return false
	}

	commentText := string(node.Utf8Text([]byte(content)))
	return strings.Contains(commentText, "shopware-block:")
}

// hoverBlockIdentifier provides hover information for block identifiers
func (p *TwigVersioningHoverProvider) hoverBlockIdentifier(node *tree_sitter.Node, content string, uri string) (*protocol.Hover, error) {
	blockName := string(node.Utf8Text([]byte(content)))
	
	// Get block hash information from the original Storefront template
	// Don't search by current path - search for any Storefront template containing this block
	allBlockHashes, err := p.twigIndexer.GetTwigBlockHashes(blockName)
	if err != nil {
		return nil, err
	}

	// Find the original Storefront template block hash
	var originalHash *twig.TwigBlockHash
	for _, hash := range allBlockHashes {
		// Look for blocks from Storefront templates
		if strings.Contains(hash.RelativePath, "storefront/") {
			originalHash = &hash
			break
		}
	}

	var hoverText strings.Builder
	hoverText.WriteString(fmt.Sprintf("**Block:** `%s`\n\n", blockName))

	if originalHash != nil {
		hoverText.WriteString(fmt.Sprintf("**Original Hash:** `%s`\n\n", originalHash.Hash))
		hoverText.WriteString(fmt.Sprintf("**Template Path:** `%s`\n\n", originalHash.RelativePath))
		
		// Check if current block has version comment
		twigFiles, err := p.twigIndexer.GetTwigFilesByRelPath(twig.ConvertToRelativePath(uri))
		if err == nil && len(twigFiles) > 0 {
			if block, exists := twigFiles[0].Blocks[blockName]; exists && block.VersionComment != nil {
				if block.VersionComment.Hash == originalHash.Hash {
					hoverText.WriteString("✅ **Status:** Block version is up to date\n\n")
				} else {
					hoverText.WriteString("⚠️ **Status:** Block version is outdated\n\n")
					hoverText.WriteString(fmt.Sprintf("**Current Version:** `%s`\n\n", block.VersionComment.Version))
				}
			} else {
				hoverText.WriteString("❗ **Status:** No version comment found\n\n")
			}
		}
	} else {
		hoverText.WriteString("ℹ️ **Status:** No original block found in Storefront templates\n\n")
	}

	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: hoverText.String(),
		},
	}, nil
}

// hoverVersionComment provides hover information for version comments
func (p *TwigVersioningHoverProvider) hoverVersionComment(node *tree_sitter.Node, content string, uri string) (*protocol.Hover, error) {
	commentText := string(node.Utf8Text([]byte(content)))
	
	// Parse the version comment
	versionComment := twig.ParseVersionComment(commentText, int(node.Range().StartPoint.Row)+1)
	if versionComment == nil {
		return nil, nil
	}

	var hoverText strings.Builder
	hoverText.WriteString("**Shopware Block Version Comment**\n\n")
	hoverText.WriteString(fmt.Sprintf("**Hash:** `%s`\n\n", versionComment.Hash))
	hoverText.WriteString(fmt.Sprintf("**Version:** `%s`\n\n", versionComment.Version))

	// Try to find the corresponding block name by looking at following nodes
	blockName := p.findBlockNameAfterComment(node, content)
	if blockName != "" {
		// Find the original Storefront template block hash
		allBlockHashes, err := p.twigIndexer.GetTwigBlockHashes(blockName)
		if err == nil {
			var originalHash *twig.TwigBlockHash
			for _, hash := range allBlockHashes {
				if strings.Contains(hash.RelativePath, "storefront/") {
					originalHash = &hash
					break
				}
			}
			
			if originalHash != nil {
				if versionComment.Hash == originalHash.Hash {
					hoverText.WriteString("✅ **Status:** Version comment matches original block\n\n")
				} else {
					hoverText.WriteString("⚠️ **Status:** Version comment is outdated\n\n")
					hoverText.WriteString(fmt.Sprintf("**Expected Hash:** `%s`\n\n", originalHash.Hash))
				}
			}
		}
	}

	return &protocol.Hover{
		Contents: protocol.MarkupContent{
			Kind:  protocol.Markdown,
			Value: hoverText.String(),
		},
	}, nil
}

// findBlockNameAfterComment tries to find the block name that follows a version comment
func (p *TwigVersioningHoverProvider) findBlockNameAfterComment(commentNode *tree_sitter.Node, content string) string {
	parent := commentNode.Parent()
	if parent == nil {
		return ""
	}

	// Look for block nodes after the comment
	for i := 0; i < int(parent.NamedChildCount()); i++ {
		child := parent.NamedChild(uint(i))
		if child == commentNode {
			// Found the comment, look for the next block
			for j := i + 1; j < int(parent.NamedChildCount()); j++ {
				nextChild := parent.NamedChild(uint(j))
				if nextChild.Kind() == "block" {
					// Find the identifier in the block
					for k := 0; k < int(nextChild.NamedChildCount()); k++ {
						blockChild := nextChild.NamedChild(uint(k))
						if blockChild.Kind() == "identifier" {
							return string(blockChild.Utf8Text([]byte(content)))
						}
					}
				}
			}
			break
		}
	}

	return ""
}
