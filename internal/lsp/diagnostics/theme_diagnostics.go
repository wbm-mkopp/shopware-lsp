package diagnostics

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
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// IconProvider is an interface for getting icon information
type IconProvider interface {
	GetIconPacks() []string
	GetIcons(pack string) []string
	GetIcon(pack, icon string) string
}

type ThemeDiagnosticsProvider struct {
	iconProvider IconProvider
}

func NewThemeDiagnosticsProvider(projectRoot string, lspServer *lsp.Server) *ThemeDiagnosticsProvider {
	extensionIndexer, _ := lspServer.GetIndexer("extension.indexer")
	iconProvider := theme.NewIconProvider(projectRoot, extensionIndexer.(*extension.ExtensionIndexer))
	
	return &ThemeDiagnosticsProvider{
		iconProvider: iconProvider,
	}
}

func (t *ThemeDiagnosticsProvider) GetDiagnostics(ctx context.Context, uri string, rootNode *tree_sitter.Node, content []byte) ([]protocol.Diagnostic, error) {
	switch strings.ToLower(filepath.Ext(uri)) {
	case ".twig":
		return t.twigDiagnostics(ctx, uri, rootNode, content)
	default:
		return []protocol.Diagnostic{}, nil
	}
}

func (t *ThemeDiagnosticsProvider) twigDiagnostics(ctx context.Context, uri string, rootNode *tree_sitter.Node, content []byte) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic

	// Find all sw_icon tags
	swIconTags := treesitterhelper.FindAll(rootNode, treesitterhelper.And(
		treesitterhelper.NodeKind("tag"),
		treesitterhelper.HasChild(
			treesitterhelper.And(
				treesitterhelper.NodeKind("keyword"),
				treesitterhelper.NodeText("sw_icon"),
			),
		),
	), content)

	for _, tagNode := range swIconTags {
		// Find the first string that's not in a pair (the icon name)
		var iconNameNode *tree_sitter.Node
		for i := 0; i < int(tagNode.ChildCount()); i++ {
			child := tagNode.Child(uint(i))
			if child != nil && child.Kind() == "string" {
				// Check if this string is part of a pair
				parent := child.Parent()
				if parent == nil || parent.Kind() != "pair" {
					iconNameNode = child
					break
				}
			}
		}

		if iconNameNode == nil {
			continue
		}

		iconName := strings.Trim(treesitterhelper.GetNodeText(iconNameNode, content), "'\"")
		
		// Extract configuration from the tag
		cfg := treesitterhelper.ExtractSwIconObjectToMap(tagNode, content)
		pack, ok := cfg["pack"]
		if !ok {
			pack = "default"
		}

		// Check if the icon exists
		iconPath := t.iconProvider.GetIcon(pack, iconName)
		if iconPath == "" {
			diagnostics = append(diagnostics, protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      int(iconNameNode.StartPosition().Row),
						Character: int(iconNameNode.StartPosition().Column),
					},
					End: protocol.Position{
						Line:      int(iconNameNode.EndPosition().Row),
						Character: int(iconNameNode.EndPosition().Column),
					},
				},
				Message:  fmt.Sprintf("Icon '%s' not found in pack '%s'", iconName, pack),
				Source:   "shopware",
				Severity: protocol.DiagnosticSeverityError,
				Code:     "theme.icon.missing",
				Data: map[string]any{
					"iconName": iconName,
					"pack":     pack,
				},
			})
		}
	}

	return diagnostics, nil
}