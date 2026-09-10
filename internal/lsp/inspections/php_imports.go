package inspections

import (
	"context"
	"fmt"

	"github.com/shopware/shopware-lsp/internal/language"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/phpanalysis"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/php"
	phpimports "github.com/shopware/shopware-lsp/internal/php/imports"
	"github.com/shopware/shopware-lsp/internal/rewrite"
)

const unusedImportFixID lsp.FixID = "php.remove-unused-import"

type phpImportsInspection struct{ index *php.PHPIndex }

func NewPHPImports(index *php.PHPIndex) lsp.Inspection { return &phpImportsInspection{index: index} }
func (*phpImportsInspection) Definition() lsp.InspectionDefinition {
	return lsp.InspectionDefinition{ID: "php.imports", Languages: []language.ID{language.PHP}, Problems: []lsp.ProblemDefinition{{ID: "php.unusedImport", Source: "shopware-php", DefaultSeverity: protocol.DiagnosticSeverityHint}}}
}
func (i *phpImportsInspection) QuickFixes() []lsp.QuickFix {
	return []lsp.QuickFix{unusedImportFix{index: i.index}}
}
func (i *phpImportsInspection) Inspect(ctx context.Context, document *lsp.TextDocument, reporter lsp.ProblemReporter) error {
	analysis, err := phpanalysis.Imports(ctx, i.index, document)
	if err != nil {
		return err
	}
	for _, scope := range analysis.Scopes {
		if scope.Unsafe {
			continue
		}
		for _, declaration := range scope.Declarations {
			for _, item := range declaration.Items {
				if err := ctx.Err(); err != nil {
					return err
				}
				if item.Used {
					continue
				}
				payload := unusedImportPayload{Range: item.Range, Alias: item.Alias}
				if err := reporter.Report(lsp.Problem{ID: "php.unusedImport", Range: item.NameRange, Element: declaration.Node, Message: fmt.Sprintf("Import %s is never used", item.Alias), Severity: protocol.DiagnosticSeverityHint, Tags: []protocol.DiagnosticTag{protocol.DiagnosticTagUnnecessary}, Payload: payload, Fixes: []lsp.BoundFix{lsp.BindFix(unusedImportFixID, payload)}}); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type unusedImportPayload struct {
	Range cst.TextRange `json:"range"`
	Alias string        `json:"alias"`
}
type unusedImportFix struct{ index *php.PHPIndex }

func (unusedImportFix) ID() lsp.FixID { return unusedImportFixID }
func (unusedImportFix) Present(_ context.Context, fix lsp.FixContext) (lsp.FixPresentation, bool, error) {
	payload, err := lsp.DecodeBoundFixPayload[unusedImportPayload](fix)
	return lsp.FixPresentation{Title: fmt.Sprintf("Remove unused import '%s'", payload.Alias), Kind: protocol.CodeActionQuickFix, Preferred: true, Resolution: lsp.FixLazy}, payload.Alias != "", err
}
func (f unusedImportFix) Build(ctx context.Context, fix lsp.FixContext) (rewrite.WorkspacePlan, error) {
	if _, err := fix.Anchor.Resolve(fix.Document.URI, fix.Document.Version, fix.Document.SyntaxLanguage, fix.Document.SyntaxTree); err != nil {
		return rewrite.WorkspacePlan{}, err
	}
	payload, err := lsp.DecodeBoundFixPayload[unusedImportPayload](fix)
	if err != nil {
		return rewrite.WorkspacePlan{}, err
	}
	analysis, err := phpanalysis.Imports(ctx, f.index, fix.Document)
	if err != nil {
		return rewrite.WorkspacePlan{}, err
	}
	for _, scope := range analysis.Scopes {
		if scope.Unsafe {
			continue
		}
		for _, declaration := range scope.Declarations {
			for _, item := range declaration.Items {
				if item.Range != payload.Range || item.Used {
					continue
				}
				edits, err := phpimports.Remove(fix.Document.Source, declaration, func(candidate phpimports.Item) bool { return candidate.Range == payload.Range })
				if err != nil {
					return rewrite.WorkspacePlan{}, err
				}
				version := fix.Document.Version
				return rewrite.WorkspacePlan{Documents: []rewrite.DocumentPlan{rewrite.NewDocumentPlan(fix.Document.URI, &version, fix.Document.Source, edits)}}, nil
			}
		}
	}
	return rewrite.WorkspacePlan{}, fmt.Errorf("unused import is no longer available")
}
