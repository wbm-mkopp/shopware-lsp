package codeaction

import (
	"context"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"github.com/shopware/shopware-lsp/internal/uriutil"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/stretchr/testify/require"
)

func applyCodeActionEdit(
	t *testing.T,
	source string,
	action protocol.CodeAction,
	uri string,
	document *lsp.TextDocument,
) string {
	t.Helper()
	require.NotNil(t, action.Edit)
	require.NotNil(t, document)
	edits := append([]protocol.TextEdit(nil), action.Edit.Changes[uri]...)
	require.NotEmpty(t, edits)
	type offsetEdit struct {
		start uint32
		end   uint32
		text  string
	}
	offsets := make([]offsetEdit, 0, len(edits))
	for _, edit := range edits {
		offsets = append(offsets, offsetEdit{
			start: document.LineIndex.OffsetUTF16(
				uint32(edit.Range.Start.Line),
				uint32(edit.Range.Start.Character),
			),
			end: document.LineIndex.OffsetUTF16(
				uint32(edit.Range.End.Line),
				uint32(edit.Range.End.Character),
			),
			text: edit.NewText,
		})
	}
	sort.SliceStable(offsets, func(left, right int) bool {
		if offsets[left].start == offsets[right].start {
			return offsets[left].end > offsets[right].end
		}
		return offsets[left].start > offsets[right].start
	})
	updated := source
	for _, edit := range offsets {
		require.LessOrEqual(t, edit.start, edit.end)
		require.LessOrEqual(t, int(edit.end), len(updated))
		updated = updated[:edit.start] + edit.text + updated[edit.end:]
	}
	return updated
}

func applyGeneratedWorkspaceEdit(t *testing.T, edit *protocol.WorkspaceEdit) {
	t.Helper()
	require.NotNil(t, edit)
	for _, change := range edit.DocumentChanges {
		if change.Kind == protocol.CreateFileOperation {
			path, err := uriutil.Path(change.URI)
			require.NoError(t, err)
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
			require.NoError(t, os.WriteFile(path, nil, 0o644))
			continue
		}
		require.NotNil(t, change.TextDocument)
		uri := change.TextDocument.URI
		path, err := uriutil.Path(uri)
		require.NoError(t, err)
		source, err := os.ReadFile(path)
		require.NoError(t, err)
		action := protocol.CodeAction{Edit: &protocol.WorkspaceEdit{Changes: map[string][]protocol.TextEdit{uri: change.Edits}}}
		updated := applyCodeActionEdit(t, string(source), action, uri, lsp.NewTextDocument(uri, string(source), 0))
		require.NoError(t, os.WriteFile(path, []byte(updated), 0o644))
	}
}

type generationTestHost struct {
	snapshots map[string]lsp.DocumentSnapshot
	plan      rewrite.WorkspacePlan
}

func (h *generationTestHost) ResolveDocument(_ context.Context, uri string) (lsp.DocumentSnapshot, error) {
	if snapshot, found := h.snapshots[uri]; found {
		return snapshot, nil
	}
	path, err := uriutil.Path(uri)
	if err != nil {
		return lsp.DocumentSnapshot{}, err
	}
	source, err := os.ReadFile(path)
	if err != nil {
		return lsp.DocumentSnapshot{}, err
	}
	return lsp.DocumentSnapshot{Document: lsp.NewTextDocument(uri, string(source), 0)}, nil
}
func (h *generationTestHost) WorkspaceEdit(_ context.Context, plan rewrite.WorkspacePlan) (*protocol.WorkspaceEdit, error) {
	h.plan = plan
	return plan.WorkspaceEdit()
}
