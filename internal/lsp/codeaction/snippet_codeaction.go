package codeaction

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/snippet"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

// SnippetCodeActionProvider provides code actions for snippet diagnostics
type SnippetCodeActionProvider struct {
	snippetIndex *snippet.SnippetIndexer
}

// NewSnippetCodeActionProvider creates a new SnippetCodeActionProvider
func NewSnippetCodeActionProvider(lspServer *lsp.Server) *SnippetCodeActionProvider {
	snippetIndexer, ok := lspServer.GetIndexer("snippet.indexer")
	if !ok {
		return &SnippetCodeActionProvider{}
	}
	return &SnippetCodeActionProvider{
		snippetIndex: snippetIndexer.(*snippet.SnippetIndexer),
	}
}

// GetCodeActionKinds returns the kinds of code actions this provider can provide
func (s *SnippetCodeActionProvider) GetCodeActionKinds() []protocol.CodeActionKind {
	return []protocol.CodeActionKind{
		protocol.CodeActionQuickFix,
	}
}

// GetCodeActions returns code actions for snippet diagnostics
func (s *SnippetCodeActionProvider) GetCodeActions(ctx context.Context, params *protocol.CodeActionParams) []protocol.CodeAction {
	if !strings.HasSuffix(strings.ToLower(filepath.Ext(params.TextDocument.URI)), ".twig") {
		return []protocol.CodeAction{}
	}

	var codeActions []protocol.CodeAction

	// Check if this is an admin file
	isAdminFile := strings.Contains(params.TextDocument.URI, "/Resources/app/administration/")

	if params.Range.Start.Line == params.Range.End.Line && params.Range.Start.Character == params.Range.End.Character {
		// No selection, so we can't create a snippet from selection
		codeActions = append(codeActions, protocol.CodeAction{
			Title: "Insert Snippet",
			Kind:  protocol.CodeActionQuickFix,
			Command: &protocol.CommandAction{
				Title:   "Insert Snippet",
				Command: "shopware.insertSnippet",
			},
		})
	}

	if params.Range.Start.Line != params.Range.End.Line || params.Range.Start.Character != params.Range.End.Character {
		// There is a text selection
		selectedText := treesitterhelper.GetTextForRange(params.DocumentContent, params.Range)
		if selectedText != "" {
			if isAdminFile {
				codeActions = append(codeActions, protocol.CodeAction{
					Title: "Create admin snippet from selection",
					Kind:  protocol.CodeActionQuickFix,
					Command: &protocol.CommandAction{
						Title:     "Create Admin Snippet from Selection",
						Command:   "shopware.createAdminSnippetFromSelection",
						Arguments: []any{params.TextDocument.URI, selectedText},
					},
				})
			} else {
				codeActions = append(codeActions, protocol.CodeAction{
					Title: "Create snippet from selection",
					Kind:  protocol.CodeActionQuickFix,
					Command: &protocol.CommandAction{
						Title:     "Create Snippet from Selection",
						Command:   "shopware.createSnippetFromSelection",
						Arguments: []any{params.TextDocument.URI, selectedText},
					},
				})
			}
		}
	}

	// Process snippet-related diagnostics
	for _, diagnostic := range params.Context.Diagnostics {
		// Handle frontend snippet missing
		if diagnostic.Code == "frontend.snippet.missing" {
			data := diagnostic.Data.(map[string]interface{})
			snippetKey := data["snippetText"].(string)

			commandAction := protocol.CodeAction{
				Title: fmt.Sprintf("Create snippet '%s'", snippetKey),
				Kind:  protocol.CodeActionQuickFix,
				Diagnostics: []protocol.Diagnostic{
					diagnostic,
				},
				Command: &protocol.CommandAction{
					Title:     "Create Snippet",
					Command:   "shopware.createSnippet",
					Arguments: []interface{}{snippetKey, params.TextDocument.URI},
				},
			}

			codeActions = append(codeActions, commandAction)
		}

		// Handle admin snippet missing
		if diagnostic.Code == "admin.snippet.missing" {
			data := diagnostic.Data.(map[string]interface{})
			snippetKey := data["snippetText"].(string)

			commandAction := protocol.CodeAction{
				Title: fmt.Sprintf("Create admin snippet '%s'", snippetKey),
				Kind:  protocol.CodeActionQuickFix,
				Diagnostics: []protocol.Diagnostic{
					diagnostic,
				},
				Command: &protocol.CommandAction{
					Title:     "Create Admin Snippet",
					Command:   "shopware.createAdminSnippet",
					Arguments: []interface{}{snippetKey, params.TextDocument.URI},
				},
			}

			codeActions = append(codeActions, commandAction)
		}
	}

	return codeActions
}
