package commands

import (
	"context"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/rewrite"
)

type EditHost interface {
	ResourcePaths(context.Context, string, int) ([]string, error)
	ResolveDocument(context.Context, string) (lsp.DocumentSnapshot, error)
	WorkspaceEdit(context.Context, rewrite.WorkspacePlan) (*protocol.WorkspaceEdit, error)
}

type EditResponse struct {
	Edit *protocol.WorkspaceEdit `json:"edit"`
}

func targetSnapshot(ctx context.Context, host EditHost, uri string) (lsp.DocumentSnapshot, error) {
	return lsp.ResolveOptionalDocument(ctx, host, uri)
}
func addReplacement(plan *rewrite.WorkspacePlan, uri string, snapshot lsp.DocumentSnapshot, content string) error {
	return lsp.AddDocumentReplacement(plan, uri, snapshot, content)
}
