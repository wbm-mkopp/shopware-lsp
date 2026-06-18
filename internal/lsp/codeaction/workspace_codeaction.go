package codeaction

import (
	"context"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
)

const ForceReindexCommand = "shopware.forceReindex"

type WorkspaceCodeActionProvider struct{}

func NewWorkspaceCodeActionProvider() *WorkspaceCodeActionProvider {
	return &WorkspaceCodeActionProvider{}
}

func (p *WorkspaceCodeActionProvider) GetCodeActionKinds() []protocol.CodeActionKind {
	return []protocol.CodeActionKind{
		protocol.CodeActionSource,
	}
}

func (p *WorkspaceCodeActionProvider) GetCodeActions(_ context.Context, params *protocol.CodeActionParams) []protocol.CodeAction {
	if !matchesCodeActionKind(params.Context.Only, protocol.CodeActionSource) {
		return nil
	}

	return []protocol.CodeAction{
		{
			Title: "Shopware: Force Reindex",
			Kind:  protocol.CodeActionSource,
			Command: &protocol.CommandAction{
				Title:   "Shopware: Force Reindex",
				Command: ForceReindexCommand,
			},
		},
	}
}

func matchesCodeActionKind(only []string, kind protocol.CodeActionKind) bool {
	if len(only) == 0 {
		return true
	}

	kindStr := string(kind)
	for _, requested := range only {
		if requested == kindStr || strings.HasPrefix(kindStr, requested+".") || strings.HasPrefix(requested, kindStr) {
			return true
		}
	}
	return false
}
