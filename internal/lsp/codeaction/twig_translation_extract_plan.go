package codeaction

import (
	"context"
	"fmt"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"github.com/shopware/shopware-lsp/internal/translation"
	"github.com/shopware/shopware-lsp/internal/uriutil"
)

// The final plan is built after locale selection, with no disk writes or
// positions carried across the interactive prompt.
func (p *TwigTranslationExtractProvider) extractionEdit(ctx context.Context, params twigTranslationExtractionRequest, selection twigTranslationSelection, replacement string, targets []twigTranslationExtractionTarget) (*protocol.WorkspaceEdit, error) {
	source, err := p.host.ResolveDocument(ctx, params.FileURI)
	if err != nil {
		return nil, err
	}
	if source.Document.Source != params.Source || params.Version != nil && (source.Version == nil || *source.Version != *params.Version) {
		return nil, rewrite.ErrStaleHandle
	}
	builder := rewrite.NewBuilder(source.Document.Source)
	if err := builder.ReplaceRange(cst.TextRange{Start: selection.start, End: selection.end}, replacement); err != nil {
		return nil, err
	}
	edits, err := builder.Finish()
	if err != nil {
		return nil, err
	}
	plan := rewrite.WorkspacePlan{Documents: []rewrite.DocumentPlan{rewrite.NewDocumentPlan(params.FileURI, source.Version, source.Document.Source, edits)}}
	allowed := make(map[string]bool, len(targets))
	for _, target := range targets {
		allowed[target.FileURI] = true
	}
	for _, uri := range params.TargetURIs {
		if !allowed[uri] {
			return nil, fmt.Errorf("invalid or duplicate translation target")
		}
		delete(allowed, uri)
		target, err := p.host.ResolveDocument(ctx, uri)
		if err != nil {
			return nil, err
		}
		file, err := uriutil.Path(uri)
		if err != nil {
			return nil, err
		}
		insertion, ok := translation.InsertionForSource(file, target.Document.Source, params.Key, selection.text)
		if !ok {
			return nil, fmt.Errorf("translation target does not support insertion")
		}
		document, err := translationInsertionPlan(uri, target, insertion)
		if err != nil {
			return nil, err
		}
		plan.Documents = append(plan.Documents, document)
	}
	return p.host.WorkspaceEdit(ctx, plan)
}

func translationInsertionPlan(uri string, target lsp.DocumentSnapshot, insertion translation.Insertion) (rewrite.DocumentPlan, error) {
	offset := target.Document.LineIndex.OffsetUTF16(uint32(insertion.Line), uint32(insertion.Character))
	builder := rewrite.NewBuilder(target.Document.Source)
	if err := builder.Insert(offset, insertion.NewText); err != nil {
		return rewrite.DocumentPlan{}, err
	}
	edits, err := builder.Finish()
	if err != nil {
		return rewrite.DocumentPlan{}, err
	}
	return rewrite.NewDocumentPlan(uri, target.Version, target.Document.Source, edits), nil
}
