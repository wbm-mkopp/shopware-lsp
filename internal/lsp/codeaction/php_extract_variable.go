package codeaction

import (
	"context"
	"fmt"

	"github.com/shopware/shopware-lsp/internal/language"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/phpanalysis"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/shopware/shopware-lsp/internal/php/refactoring"
	"github.com/shopware/shopware-lsp/internal/rewrite"
)

type PHPExtractVariableProvider struct{ index *php.PHPIndex }

func NewPHPExtractVariableProvider(indexes ...*php.PHPIndex) *PHPExtractVariableProvider {
	p := &PHPExtractVariableProvider{}
	if len(indexes) > 0 {
		p.index = indexes[0]
	}
	return p
}
func (*PHPExtractVariableProvider) GetCodeActionKinds() []protocol.CodeActionKind {
	return []protocol.CodeActionKind{protocol.CodeActionRefactorExtract}
}
func (p *PHPExtractVariableProvider) GetCodeActions(ctx context.Context, request *lsp.CodeActionRequest) []protocol.CodeAction {
	if request == nil || request.CodeActionParams == nil || request.Document == nil {
		return nil
	}
	doc := request.Document
	if doc.SyntaxLanguage != language.PHP || doc.SyntaxTree == nil || doc.LineIndex == nil || len(doc.ParseErrors) != 0 {
		return nil
	}
	rng := request.Range
	if rng.Start.Line < 0 || rng.Start.Character < 0 || rng.End.Line < 0 || rng.End.Character < 0 {
		return nil
	}
	selection := cst.TextRange{Start: doc.LineIndex.OffsetUTF16(uint32(rng.Start.Line), uint32(rng.Start.Character)), End: doc.LineIndex.OffsetUTF16(uint32(rng.End.Line), uint32(rng.End.Character))}

	option := refactoring.VariableOptions{}
	option.CallReturnsValue = func(call *cst.Node) bool {
		if ctx.Err() != nil {
			return false
		}
		state, err := phpanalysis.ForDocument(p.index, doc)
		return err == nil && state != nil && state.CallReturnsValue(call)
	}
	var actions []protocol.CodeAction
	for _, all := range []bool{false, true} {
		option.AllOccurrences = all
		result, err := refactoring.ExtractVariable(ctx, doc.SyntaxTree.Root, doc.Source, selection, option)
		if err != nil || result == nil {
			continue
		}
		version := doc.Version
		plan := rewrite.WorkspacePlan{Documents: []rewrite.DocumentPlan{rewrite.NewDocumentPlan(doc.URI, &version, doc.Source, result.Edits)}}
		edit, err := plan.WorkspaceEdit()
		if err != nil {
			continue
		}
		title := "Extract variable '" + result.Name + "'"
		if all {
			title += fmt.Sprintf(" (all %d occurrences)", result.Occurrences)
		}
		actions = append(actions, protocol.CodeAction{Title: title, Kind: protocol.CodeActionRefactorExtract, Edit: edit})
	}
	return actions
}
