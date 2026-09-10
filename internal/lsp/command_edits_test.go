package lsp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"github.com/shopware/shopware-lsp/internal/uriutil"
	"github.com/stretchr/testify/require"
)

func TestCommandWorkspaceEditRejectsChangedOpenTarget(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "snippet.json")
	uri := uriutil.FileURI(path)
	require.NoError(t, os.WriteFile(path, []byte(`{}`), 0o644))
	server := NewServer(nil, root, "test")
	t.Cleanup(func() { require.NoError(t, server.CloseAll()) })
	server.documentManager.OpenDocument(uri, `{"open":true}`, 5)
	snapshot, err := server.ResolveDocument(context.Background(), uri)
	require.NoError(t, err)
	require.Equal(t, `{"open":true}`, snapshot.Document.SourceString())
	plan := rewrite.WorkspacePlan{Documents: []rewrite.DocumentPlan{rewrite.NewDocumentPlan(uri, snapshot.Version, snapshot.Document.SourceString(), []rewrite.Edit{{Range: cst.TextRange{End: 13}, NewText: `{"open":false}`}})}}
	_, err = server.WorkspaceEdit(context.Background(), plan)
	require.NoError(t, err)
	server.documentManager.UpdateDocument(uri, `{"open":true,"new":1}`, 6)
	_, err = server.WorkspaceEdit(context.Background(), plan)
	require.ErrorIs(t, err, rewrite.ErrStaleHandle)
}

func TestCommandCreateRejectsUnsavedOpenTarget(t *testing.T) {
	root := t.TempDir()
	uri := uriutil.FileURI(filepath.Join(root, "new.json"))
	server := NewServer(nil, root, "test")
	t.Cleanup(func() { require.NoError(t, server.CloseAll()) })
	plan := rewrite.WorkspacePlan{Creates: []rewrite.CreateFilePlan{{URI: uri, Content: `{}`}}}
	server.documentManager.OpenDocument(uri, `{"unsaved":true}`, 1)
	_, err := server.WorkspaceEdit(context.Background(), plan)
	require.ErrorContains(t, err, "already open")
}
