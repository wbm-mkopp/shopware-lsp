package lsp

import (
	"context"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
)

// CodeActionProvider is an interface for providing code actions
type CodeActionProvider interface {
	// GetCodeActions returns code actions for the given parameters
	GetCodeActions(ctx context.Context, params *protocol.CodeActionParams) []protocol.CodeAction
	// GetCodeActionKinds returns the kinds of code actions this provider can provide
	GetCodeActionKinds() []protocol.CodeActionKind
}
