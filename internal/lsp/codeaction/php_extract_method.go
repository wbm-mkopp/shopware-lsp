package codeaction

import (
	"context"
	"strings"

	"github.com/shopware/shopware-lsp/internal/language"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/phpanalysis"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/parser/php/query"
	"github.com/shopware/shopware-lsp/internal/php"
	"github.com/shopware/shopware-lsp/internal/php/refactoring"
	"github.com/shopware/shopware-lsp/internal/php/resolver"
	"github.com/shopware/shopware-lsp/internal/php/semantic"
	"github.com/shopware/shopware-lsp/internal/php/types"
	"github.com/shopware/shopware-lsp/internal/rewrite"
)

type PHPExtractMethodProvider struct{ index *php.PHPIndex }

func NewPHPExtractMethodProvider(index *php.PHPIndex) *PHPExtractMethodProvider {
	return &PHPExtractMethodProvider{index: index}
}
func (*PHPExtractMethodProvider) GetCodeActionKinds() []protocol.CodeActionKind {
	return []protocol.CodeActionKind{protocol.CodeActionRefactorExtract}
}
func (p *PHPExtractMethodProvider) GetCodeActions(ctx context.Context, request *lsp.CodeActionRequest) []protocol.CodeAction {
	if ctx.Err() != nil || request == nil || request.CodeActionParams == nil || request.Document == nil {
		return nil
	}
	doc := request.Document
	if doc.SyntaxLanguage != language.PHP || doc.SyntaxTree == nil || doc.LineIndex == nil || len(doc.ParseErrors) > 0 {
		return nil
	}
	rng := request.Range
	if rng.Start.Line < 0 || rng.Start.Character < 0 || rng.End.Line < 0 || rng.End.Character < 0 {
		return nil
	}
	selection := cst.TextRange{Start: doc.LineIndex.OffsetUTF16(uint32(rng.Start.Line), uint32(rng.Start.Character)), End: doc.LineIndex.OffsetUTF16(uint32(rng.End.Line), uint32(rng.End.Character))}
	selected := query.SelectMethodStatements(ctx, doc.SyntaxTree.Root, doc.Source, selection)
	if selected == nil {
		return nil
	}
	state, err := phpanalysis.ForDocument(p.index, doc)
	if err != nil || state == nil {
		return nil
	}
	available, label := methodNameChecker(ctx, state, selected)
	if available == nil {
		return nil
	}
	result, err := refactoring.ExtractMethod(ctx, doc.SyntaxTree.Root, doc.Source, selection, available)

	if err != nil || result == nil {
		return nil
	}
	version := doc.Version
	plan := rewrite.WorkspacePlan{Documents: []rewrite.DocumentPlan{rewrite.NewDocumentPlan(doc.URI, &version, doc.Source, result.Edits)}}
	edit, err := plan.WorkspaceEdit()
	if err != nil {
		return nil
	}
	return []protocol.CodeAction{{Title: label + " '" + result.Name + "'", Kind: protocol.CodeActionRefactorExtract, Edit: edit}}
}

func methodHierarchyKnown(ctx context.Context, snapshot *semantic.Snapshot, class semantic.Symbol, seen map[semantic.SymbolID]bool) bool {
	if ctx.Err() != nil {
		return false
	}
	if seen[class.ID] {
		return true
	}
	seen[class.ID] = true
	for _, group := range [][]string{class.Extends(), class.Implements(), class.Traits()} {
		for _, name := range group {
			parents := snapshot.Classes(strings.TrimPrefix(name, "\\"))
			if len(parents) != 1 || !methodHierarchyKnown(ctx, snapshot, parents[0], seen) {
				return false
			}
		}
	}
	return true
}

func methodNameChecker(ctx context.Context, state *phpanalysis.State, selected *query.MethodSelection) (refactoring.MethodNameAvailable, string) {
	if selected.Class == nil {
		for _, symbol := range state.Document.Symbols {
			if symbol.Kind != semantic.FunctionSymbol || symbol.Range.Start > selected.Range.Start || symbol.Range.End < selected.Range.End {
				continue
			}
			namespace := ""
			if i := strings.LastIndexByte(symbol.FullyQualified, '\\'); i >= 0 {
				namespace = symbol.FullyQualified[:i+1]
			}
			return func(_ *cst.Node, name string) bool { return len(state.Snapshot.FunctionViews(namespace+name)) == 0 }, "Extract function"
		}
		return nil, ""
	}
	for _, class := range state.Document.Symbols {
		if class.Kind != semantic.ClassSymbol || class.Range.Start > selected.Range.Start || class.Range.End < selected.Range.End {
			continue
		}
		if !methodHierarchyKnown(ctx, state.Snapshot, class, map[semantic.SymbolID]bool{}) {
			return nil, ""
		}
		members := resolver.MemberResolver{Snapshot: state.Snapshot}
		return func(_ *cst.Node, name string) bool {
			return len(members.Methods(types.Named(class.FullyQualified), name)) == 0
		}, "Extract method"
	}
	return nil, ""
}
