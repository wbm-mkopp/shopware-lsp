package phpanalysis

import (
	"context"

	"github.com/shopware/shopware-lsp/internal/language"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/php"
	phpimports "github.com/shopware/shopware-lsp/internal/php/imports"
	"github.com/shopware/shopware-lsp/internal/php/suppression"
)

// Imports is shared by diagnostics and explicit source actions on one snapshot.
func Imports(ctx context.Context, index *php.PHPIndex, document *lsp.TextDocument) (phpimports.Analysis, error) {
	if err := ctx.Err(); err != nil {
		return phpimports.Analysis{}, err
	}
	if document == nil || document.SyntaxLanguage != language.PHP || len(document.ParseErrors) > 0 {
		return phpimports.Analysis{}, nil
	}
	state, err := ForDocument(index, document)
	if err != nil || state == nil {
		return phpimports.Analysis{}, err
	}
	// Cancelled results must not be memoized on the immutable document.
	result, err := phpimports.Analyze(ctx, document.SyntaxTree.Root, state.Document)
	if err != nil {
		return phpimports.Analysis{}, err
	}
	suppressed := suppression.Parse(document.Source)
	for si := range result.Scopes {
		for di := range result.Scopes[si].Declarations {
			declaration := &result.Scopes[si].Declarations[di]
			for ii := range declaration.Items {
				item := &declaration.Items[ii]
				if suppressed.Suppresses(item.Range.Start, "php.unusedImport") || suppressed.Suppresses(declaration.Node.RangeTrimmedTrivia().Start, "php.unusedImport") {
					item.Used = true
				}
			}
		}
	}
	return result, nil
}
