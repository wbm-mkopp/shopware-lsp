package lsp

import (
	"context"
	"fmt"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"github.com/shopware/shopware-lsp/internal/uriutil"
)

// ResolveDocument supplies command adapters with the same snapshots as fixes.
func (s *Server) ResolveDocument(ctx context.Context, uri string) (DocumentSnapshot, error) {
	path, err := uriutil.Path(uri)
	if err != nil || !pathWithinRoot(s.rootPath, path) {
		return DocumentSnapshot{}, fmt.Errorf("document %q is outside the workspace", uri)
	}
	return (serverDocumentResolver{server: s}).ResolveDocument(ctx, uri)
}

// WorkspaceEdit validates a command plan before exposing it to any frontend.
func (s *Server) WorkspaceEdit(ctx context.Context, plan rewrite.WorkspacePlan) (*protocol.WorkspaceEdit, error) {
	if err := s.validateWorkspacePlan(ctx, plan); err != nil {
		return nil, err
	}
	return plan.WorkspaceEdit()
}

func (s *Server) ResourcePaths(ctx context.Context, directory string, limit int) ([]string, error) {
	if !pathWithinRoot(s.rootPath, directory) {
		return nil, fmt.Errorf("resource directory is outside the workspace")
	}
	return s.fileScanner.ResourcePaths(ctx, directory, limit)
}
