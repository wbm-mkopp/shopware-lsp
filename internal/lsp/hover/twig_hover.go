package hover

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/extension"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/theme"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type TwigHoverProvider struct {
	iconProvider *theme.IconProvider
	projectRoot  string
}

func NewTwigHoverProvider(projectRoot string, lspServer *lsp.Server) *TwigHoverProvider {
	extensionIndexer, _ := lspServer.GetIndexer("extension.indexer")

	iconProvider := theme.NewIconProvider(projectRoot, extensionIndexer.(*extension.ExtensionIndexer))

	return &TwigHoverProvider{
		iconProvider: iconProvider,
		projectRoot:  projectRoot,
	}
}

func (p *TwigHoverProvider) GetHover(ctx context.Context, params *protocol.HoverParams) (*protocol.Hover, error) {
	if params.Node == nil {
		return nil, nil
	}

	// Only process .twig files
	if !strings.HasSuffix(strings.ToLower(params.TextDocument.URI), ".twig") {
		return nil, nil
	}

	// Check if hovering over sw_icon
	if treesitterhelper.TwigStringInTagPattern("sw_icon").Matches(params.Node, []byte(params.DocumentContent)) {
		iconName := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)
		
		// Extract icon configuration from parent node
		cfg := treesitterhelper.ExtractSwIconObjectToMap(params.Node.Parent(), params.DocumentContent)
		
		pack, ok := cfg["pack"]
		if !ok {
			pack = "default"
		}
		
		// Get the icon path
		iconPath := p.iconProvider.GetIcon(pack, iconName)
		
		if iconPath != "" {
			// Create markdown content with icon preview
			// For VSCode and other editors, we need to use file:// URIs for local images
			var imageUri string
			if strings.HasPrefix(iconPath, "/") {
				// Absolute path - convert to file URI
				imageUri = fmt.Sprintf("file://%s", iconPath)
			} else {
				// Try to create a relative path from the current document
				docDir := filepath.Dir(strings.TrimPrefix(params.TextDocument.URI, "file://"))
				relPath, err := filepath.Rel(docDir, iconPath)
				if err != nil {
					// If relative path fails, use absolute file URI
					imageUri = fmt.Sprintf("file://%s", iconPath)
				} else {
					imageUri = relPath
				}
			}
			
			// Make display path relative to project root
			displayPath, err := filepath.Rel(p.projectRoot, iconPath)
			if err != nil {
				// If we can't make it relative, use the original path
				displayPath = iconPath
			}
			
			markdownContent := fmt.Sprintf("**Icon**: `%s`\n\n**Pack**: `%s`\n\n**Preview**:\n\n![%s](%s)\n\n**Path**: `%s`", 
				iconName, 
				pack, 
				iconName,
				imageUri,
				displayPath,
			)
			
			return &protocol.Hover{
				Contents: protocol.MarkupContent{
					Kind:  protocol.Markdown,
					Value: markdownContent,
				},
				Range: &protocol.Range{
					Start: protocol.Position{
						Line:      params.Position.Line,
						Character: params.Position.Character,
					},
					End: protocol.Position{
						Line:      params.Position.Line,
						Character: params.Position.Character + len(iconName),
					},
				},
			}, nil
		}
	}

	return nil, nil
}