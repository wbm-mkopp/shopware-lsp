package lsp

import (
	"context"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// DiagnosticsProvider is an interface for providing diagnostics for a document
type DiagnosticsProvider interface {
	// GetDiagnostics returns diagnostics for a document
	GetDiagnostics(ctx context.Context, uri string, rootNode *tree_sitter.Node, content []byte) ([]protocol.Diagnostic, error)
}
