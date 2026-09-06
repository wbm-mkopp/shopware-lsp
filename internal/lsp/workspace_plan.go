package lsp

import (
	"context"
	"errors"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"os"
)

// WorkspaceEditHost gives generators access to validated workspace snapshots and edits.
type WorkspaceEditHost interface {
	DocumentResolver
	WorkspaceEdit(context.Context, rewrite.WorkspacePlan) (*protocol.WorkspaceEdit, error)
}

// ResolveOptionalDocument treats a missing in-workspace file as a create target.
func ResolveOptionalDocument(ctx context.Context, host WorkspaceEditHost, uri string) (DocumentSnapshot, error) {
	snapshot, err := host.ResolveDocument(ctx, uri)
	if errors.Is(err, os.ErrNotExist) {
		return DocumentSnapshot{}, nil
	}
	return snapshot, err
}

// AddDocumentReplacement adds a create or versioned replacement to a shared plan.
func AddDocumentReplacement(plan *rewrite.WorkspacePlan, uri string, snapshot DocumentSnapshot, content string) error {
	if snapshot.Document == nil {
		plan.Creates = append(plan.Creates, rewrite.CreateFilePlan{URI: uri, Content: content})
		return nil
	}
	source := snapshot.Document.SourceString()
	builder := rewrite.NewBuilder(source)
	if err := builder.ReplaceRange(cst.TextRange{End: uint32(len(source))}, content); err != nil {
		return err
	}
	edits, err := builder.Finish()
	if err != nil {
		return err
	}
	plan.Documents = append(plan.Documents, rewrite.NewDocumentPlan(uri, snapshot.Version, source, edits))
	return nil
}
