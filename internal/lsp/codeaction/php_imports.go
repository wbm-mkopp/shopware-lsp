package codeaction

import (
	"context"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/phpanalysis"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/php"
	phpimports "github.com/shopware/shopware-lsp/internal/php/imports"
	"github.com/shopware/shopware-lsp/internal/rewrite"
)

type PHPImportsProvider struct{ index *php.PHPIndex }

func NewPHPImportsProvider(index *php.PHPIndex) *PHPImportsProvider {
	return &PHPImportsProvider{index: index}
}
func (*PHPImportsProvider) GetCodeActionKinds() []protocol.CodeActionKind {
	return []protocol.CodeActionKind{protocol.CodeActionSourceOrganizeImports}
}
func (p *PHPImportsProvider) GetCodeActions(ctx context.Context, request *lsp.CodeActionRequest) []protocol.CodeAction {
	if request == nil || request.Document == nil {
		return nil
	}
	analysis, err := phpanalysis.Imports(ctx, p.index, request.Document)
	if err != nil {
		return nil
	}
	edits, err := phpimports.Organize(ctx, request.Document.Source, analysis)
	if err != nil || len(edits) == 0 {
		return nil
	}
	version := request.Document.Version
	plan := rewrite.WorkspacePlan{Documents: []rewrite.DocumentPlan{rewrite.NewDocumentPlan(request.Document.URI, &version, request.Document.Source, edits)}}
	edit, err := plan.WorkspaceEdit()
	if err != nil {
		return nil
	}
	return []protocol.CodeAction{{Title: "Organize Imports", Kind: protocol.CodeActionSourceOrganizeImports, Edit: edit}}
}
