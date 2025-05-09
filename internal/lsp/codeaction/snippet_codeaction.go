package codeaction

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/snippet"
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

	codeActions = append(codeActions, protocol.CodeAction{
		Title: "Insert Snippet",
		Kind:  protocol.CodeActionQuickFix,
		Command: &protocol.CommandAction{
			Title:   "Insert Snippet",
			Command: "shopware.insertSnippet",
		},
	})

	// Process only snippet-related diagnostics
	for _, diagnostic := range params.Context.Diagnostics {
		if diagnostic.Code != "frontend.snippet.missing" {
			continue
		}

		data := diagnostic.Data.(map[string]interface{})

		snippetKey := data["snippetText"].(string)

		// Create command-based code action
		commandAction := protocol.CodeAction{
			Title: fmt.Sprintf("Create snippet %s", snippetKey),
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

	return codeActions
}
