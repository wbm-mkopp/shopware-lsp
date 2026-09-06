package lsp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/uriutil"
	"github.com/stretchr/testify/require"
)

type legacyTestActionProvider struct{ action protocol.CodeAction }

func (p legacyTestActionProvider) GetCodeActionKinds() []protocol.CodeActionKind {
	return []protocol.CodeActionKind{protocol.CodeActionQuickFix}
}
func (p legacyTestActionProvider) GetCodeActions(context.Context, *CodeActionRequest) []protocol.CodeAction {
	return []protocol.CodeAction{p.action}
}

func TestLegacyActionsAreVersionedAndRejectStaleTargets(t *testing.T) {
	root := t.TempDir()
	s := NewServer(nil, root, "test")
	t.Cleanup(func() { require.NoError(t, s.CloseAll()) })
	s.codeActionResolveSupport = true
	uri := uriutil.FileURI(filepath.Join(root, "test.yaml"))
	s.documentManager.OpenDocument(uri, "value: bad\n", 7)
	action := protocol.CodeAction{Title: "Replace", Kind: protocol.CodeActionQuickFix, Edit: &protocol.WorkspaceEdit{Changes: map[string][]protocol.TextEdit{uri: {{Range: protocol.Range{Start: protocol.Position{Character: 7}, End: protocol.Position{Character: 10}}, NewText: "good"}}}}}
	s.RegisterActionProvider(legacyTestActionProvider{action: action})
	params := &protocol.CodeActionParams{}
	params.TextDocument.URI = uri
	actions := s.codeAction(context.Background(), params)
	require.Len(t, actions, 1)
	require.Nil(t, actions[0].Disabled)
	require.Empty(t, actions[0].Edit.Changes)
	require.Equal(t, 7, *actions[0].Edit.DocumentChanges[0].TextDocument.Version)
	require.Nil(t, s.resolveCodeAction(context.Background(), actions[0]).Disabled)
	s.documentManager.UpdateDocument(uri, "value: different\n", 8)
	resolved := s.resolveCodeAction(context.Background(), actions[0])
	require.NotNil(t, resolved.Disabled)
	require.Nil(t, resolved.Edit)
}

func TestLegacyActionsRejectChangedClosedTarget(t *testing.T) {
	root := t.TempDir()
	s := NewServer(nil, root, "test")
	t.Cleanup(func() { require.NoError(t, s.CloseAll()) })
	path := filepath.Join(root, "test.yaml")
	uri := uriutil.FileURI(path)
	require.NoError(t, os.WriteFile(path, []byte("value: old\n"), 0o644))
	action := s.normalizeProviderAction(context.Background(), protocol.CodeAction{Edit: &protocol.WorkspaceEdit{Changes: map[string][]protocol.TextEdit{uri: {{NewText: "# comment\n"}}}}})
	require.Nil(t, action.Disabled)
	require.NoError(t, os.WriteFile(path, []byte("value: changed\n"), 0o644))
	resolved := s.resolveCodeAction(context.Background(), action)
	require.NotNil(t, resolved.Disabled)
	require.Nil(t, resolved.Edit)
}

func TestLegacyActionsValidateRangesAndContainment(t *testing.T) {
	root := t.TempDir()
	s := NewServer(nil, root, "test")
	t.Cleanup(func() { require.NoError(t, s.CloseAll()) })
	uri := uriutil.FileURI(filepath.Join(root, "test.yaml"))
	outside := uriutil.FileURI(filepath.Join(t.TempDir(), "outside.yaml"))
	s.documentManager.OpenDocument(uri, "value: 😀\n", 1)
	s.documentManager.OpenDocument(outside, "value: 😀\n", 1)
	for name, action := range map[string]protocol.CodeAction{
		"outside":   {Edit: &protocol.WorkspaceEdit{Changes: map[string][]protocol.TextEdit{outside: {{NewText: "# outside\n"}}}}},
		"surrogate": {Edit: &protocol.WorkspaceEdit{Changes: map[string][]protocol.TextEdit{uri: {{Range: protocol.Range{Start: protocol.Position{Character: 8}, End: protocol.Position{Character: 8}}, NewText: "x"}}}}},
		"overlap":   {Edit: &protocol.WorkspaceEdit{Changes: map[string][]protocol.TextEdit{uri: {{Range: protocol.Range{End: protocol.Position{Character: 5}}, NewText: "x"}, {Range: protocol.Range{End: protocol.Position{Character: 4}}, NewText: "y"}}}}},
	} {
		t.Run(name, func(t *testing.T) {
			result := s.normalizeProviderAction(context.Background(), action)
			require.NotNil(t, result.Disabled)
			require.Nil(t, result.Edit)
		})
	}
}

func BenchmarkLegacyActionValidation(b *testing.B) {
	s := NewServer(nil, b.TempDir(), "benchmark")
	b.Cleanup(func() { require.NoError(b, s.CloseAll()) })
	uri := uriutil.FileURI(filepath.Join(s.rootPath, "services.yaml"))
	source := "value: bad\n" + strings.Repeat("# configuration comment\n", 500)
	s.documentManager.OpenDocument(uri, source, 1)
	action := protocol.CodeAction{Edit: &protocol.WorkspaceEdit{Changes: map[string][]protocol.TextEdit{uri: {{Range: protocol.Range{Start: protocol.Position{Character: 7}, End: protocol.Position{Character: 10}}, NewText: "good"}}}}}
	b.ReportAllocs()
	for b.Loop() {
		result := s.normalizeProviderAction(context.Background(), action)
		if result.Disabled != nil {
			b.Fatal(result.Disabled.Reason)
		}
	}
}
