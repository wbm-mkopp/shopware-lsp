package lsp

import (
	"context"
	"crypto/sha256"
	"fmt"
	"sort"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/rewrite"
)

const legacyActionSchema = "validated-workspace-edit-v1"

type legacyActionSnapshot struct {
	URI     string `json:"uri"`
	Version *int   `json:"version"`
	Digest  string `json:"digest"`
}
type legacyActionEnvelope struct {
	Type      string                  `json:"type"`
	Snapshots []legacyActionSnapshot  `json:"snapshots"`
	Edit      *protocol.WorkspaceEdit `json:"edit"`
}

// normalizeProviderAction brings legacy text-edit providers through the same
// snapshot and validation boundary as typed inspection fixes.
func (s *Server) normalizeProviderAction(ctx context.Context, action protocol.CodeAction) protocol.CodeAction {
	if action.Disabled != nil {
		return disabledCodeAction(action, action.Disabled.Reason)
	}
	if action.Edit == nil {
		return action
	}
	plan, snapshots, err := s.providerEditPlan(ctx, action.Edit)
	if err != nil {
		return disabledCodeAction(action, "The generated edit is invalid: "+err.Error())
	}
	edit, err := s.WorkspaceEdit(ctx, plan)
	if err != nil {
		return disabledCodeAction(action, "The generated edit is no longer valid")
	}
	action.Edit = edit
	action.Data = legacyActionEnvelope{Type: legacyActionSchema, Snapshots: snapshots, Edit: edit}
	return action
}

func (s *Server) resolveProviderAction(ctx context.Context, action protocol.CodeAction, data legacyActionEnvelope) protocol.CodeAction {
	for _, expected := range data.Snapshots {
		snapshot, err := s.ResolveDocument(ctx, expected.URI)
		if err != nil || !sameOptionalVersion(snapshot.Version, expected.Version) || sourceDigest(snapshot.Document.Source) != expected.Digest {
			return disabledCodeAction(action, "The target document changed; request code actions again")
		}
	}
	action.Edit = data.Edit
	return s.normalizeProviderAction(ctx, action)
}

func sourceDigest(source string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(source))) }

func (s *Server) providerEditPlan(ctx context.Context, edit *protocol.WorkspaceEdit) (rewrite.WorkspacePlan, []legacyActionSnapshot, error) {
	plan := rewrite.WorkspacePlan{}
	changes := make(map[string][]protocol.TextEdit, len(edit.Changes))
	for uri, edits := range edit.Changes {
		changes[uri] = edits
	}
	versions := make(map[string]*int)
	for _, change := range edit.DocumentChanges {
		// Resource operations must be emitted through the typed rewrite/command API.
		if change.Kind != "" || change.TextDocument == nil {
			return plan, nil, fmt.Errorf("resource operation requires a workspace plan")
		}
		uri := change.TextDocument.URI
		if _, exists := changes[uri]; exists {
			return plan, nil, fmt.Errorf("duplicate document edit")
		}
		changes[uri] = change.Edits
		versions[uri] = change.TextDocument.Version
	}
	uris := make([]string, 0, len(changes))
	for uri := range changes {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	var snapshots []legacyActionSnapshot
	for _, uri := range uris {
		snapshot, err := s.ResolveDocument(ctx, uri)
		if err != nil {
			return plan, nil, err
		}
		if version, provided := versions[uri]; provided && !sameOptionalVersion(version, snapshot.Version) {
			return plan, nil, rewrite.ErrStaleHandle
		}
		document := snapshot.Document
		builder := rewrite.NewBuilder(document.Source)
		for _, edit := range changes[uri] {
			start, err := strictPositionOffset(document.LineIndex, edit.Range.Start)
			if err != nil {
				return plan, nil, err
			}
			end, err := strictPositionOffset(document.LineIndex, edit.Range.End)
			if err != nil {
				return plan, nil, err
			}
			if err := builder.ReplaceRange(cst.TextRange{Start: start, End: end}, edit.NewText); err != nil {
				return plan, nil, err
			}
		}
		edits, err := builder.Finish()
		if err != nil {
			return plan, nil, err
		}
		plan.Documents = append(plan.Documents, rewrite.NewDocumentPlan(uri, snapshot.Version, document.Source, edits))
		snapshots = append(snapshots, legacyActionSnapshot{URI: uri, Version: snapshot.Version, Digest: sourceDigest(document.Source)})
	}
	return plan, snapshots, nil
}

func strictPositionOffset(index *cst.LineIndex, position protocol.Position) (uint32, error) {
	if position.Line < 0 || position.Character < 0 {
		return 0, fmt.Errorf("invalid edit position")
	}
	offset := index.OffsetUTF16(uint32(position.Line), uint32(position.Character))
	line, character := index.PositionUTF16(offset)
	if int(line) != position.Line || int(character) != position.Character {
		return 0, fmt.Errorf("invalid UTF-16 edit position")
	}
	return offset, nil
}
