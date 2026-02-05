package diagnostics

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/snippet"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type SnippetDiagnosticsProvider struct {
	snippetIndex *snippet.SnippetIndexer
}

func NewSnippetDiagnosticsProvider(lspServer *lsp.Server) *SnippetDiagnosticsProvider {
	snippetIndexer, _ := lspServer.GetIndexer("snippet.indexer")
	return &SnippetDiagnosticsProvider{
		snippetIndex: snippetIndexer.(*snippet.SnippetIndexer),
	}
}

func (s *SnippetDiagnosticsProvider) GetDiagnostics(ctx context.Context, uri string, rootNode *tree_sitter.Node, content []byte) ([]protocol.Diagnostic, error) {
	switch strings.ToLower(filepath.Ext(uri)) {
	case ".twig":
		return s.twigDiagnostics(ctx, uri, rootNode, content)
	case ".js", ".ts":
		return s.jsDiagnostics(ctx, uri, rootNode, content)
	default:
		return []protocol.Diagnostic{}, nil
	}
}

func (s *SnippetDiagnosticsProvider) twigDiagnostics(ctx context.Context, uri string, rootNode *tree_sitter.Node, content []byte) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic

	// Check if this is an admin file
	isAdminFile := strings.Contains(uri, "/Resources/app/administration/")

	if isAdminFile {
		// Check for admin snippet pattern: {{ $tc('key') }} or {{ $t('key') }}
		matches := treesitterhelper.FindAll(rootNode, treesitterhelper.TwigAdminSnippetPattern(), content)

		for _, match := range matches {
			snippetText := treesitterhelper.GetNodeText(match, content)

			// Skip empty strings
			if snippetText == "" {
				continue
			}

			snippets, _ := s.snippetIndex.GetAdminSnippet(snippetText)

			if len(snippets) == 0 {
				diagnostics = append(diagnostics, protocol.Diagnostic{
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      int(match.StartPosition().Row),
							Character: int(match.StartPosition().Column),
						},
						End: protocol.Position{
							Line:      int(match.EndPosition().Row),
							Character: int(match.EndPosition().Column),
						},
					},
					Message:  fmt.Sprintf("Admin snippet '%s' not found", snippetText),
					Source:   "shopware",
					Severity: protocol.DiagnosticSeverityError,
					Code:     "admin.snippet.missing",
					Data: map[string]any{
						"snippetText": snippetText,
					},
				})
			}
		}
	} else {
		// Check for frontend snippet pattern: {{ 'key'|trans }}
		matches := treesitterhelper.FindAll(rootNode, treesitterhelper.TwigTransPattern(), content)

		for _, match := range matches {
			snippetText := treesitterhelper.GetNodeText(match, content)

			// Skip empty strings
			if snippetText == "" {
				continue
			}

			snippets, _ := s.snippetIndex.GetFrontendSnippet(snippetText)

			if len(snippets) == 0 {
				diagnostics = append(diagnostics, protocol.Diagnostic{
					Range: protocol.Range{
						Start: protocol.Position{
							Line:      int(match.StartPosition().Row),
							Character: int(match.StartPosition().Column),
						},
						End: protocol.Position{
							Line:      int(match.EndPosition().Row),
							Character: int(match.EndPosition().Column),
						},
					},
					Message:  fmt.Sprintf("Snippet '%s' not found", snippetText),
					Source:   "shopware",
					Severity: protocol.DiagnosticSeverityError,
					Code:     "frontend.snippet.missing",
					Data: map[string]any{
						"snippetText": snippetText,
					},
				})
			}
		}
	}

	return diagnostics, nil
}

func (s *SnippetDiagnosticsProvider) jsDiagnostics(ctx context.Context, uri string, rootNode *tree_sitter.Node, content []byte) ([]protocol.Diagnostic, error) {
	var diagnostics []protocol.Diagnostic

	// Check for admin snippet pattern: this.$tc('key') or this.$t('key')
	matches := treesitterhelper.FindAll(rootNode, treesitterhelper.JSAdminSnippetPattern(), content)

	for _, match := range matches {
		snippetText := treesitterhelper.GetNodeText(match, content)

		// Skip empty strings
		if snippetText == "" {
			continue
		}

		snippets, _ := s.snippetIndex.GetAdminSnippet(snippetText)

		if len(snippets) == 0 {
			diagnostics = append(diagnostics, protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{
						Line:      int(match.StartPosition().Row),
						Character: int(match.StartPosition().Column),
					},
					End: protocol.Position{
						Line:      int(match.EndPosition().Row),
						Character: int(match.EndPosition().Column),
					},
				},
				Message:  fmt.Sprintf("Admin snippet '%s' not found", snippetText),
				Source:   "shopware",
				Severity: protocol.DiagnosticSeverityError,
				Code:     "admin.snippet.missing",
				Data: map[string]any{
					"snippetText": snippetText,
				},
			})
		}
	}

	return diagnostics, nil
}
