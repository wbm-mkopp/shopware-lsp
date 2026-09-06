package lsp

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/projectconfig"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"github.com/shopware/shopware-lsp/internal/uriutil"
	"github.com/stretchr/testify/require"
)

func TestInspectionActionsRecheckCurrentPolicy(t *testing.T) {
	disabled := false
	for name, config := range map[string]projectconfig.DiagnosticsConfig{
		"inspection": {Inspections: map[string]bool{"test.invalid-value": false}},
		"rule":       {Rules: map[string]projectconfig.Severity{"test.invalid-value": projectconfig.SeverityOff}},
		"all":        {Enabled: &disabled},
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			s := NewServer(nil, root, "test")
			t.Cleanup(func() { require.NoError(t, s.CloseAll()) })
			s.RegisterInspection(testInspection{})
			s.codeActionResolveSupport = true
			uri := uriutil.FileURI(filepath.Join(root, "test.yaml"))
			s.documentManager.OpenDocument(uri, "value: bad\n", 1)
			doc, _ := s.documentManager.GetDocument(uri)
			diagnostics := s.collectDiagnostics(context.Background(), doc)
			require.Len(t, diagnostics, 1)
			params := &protocol.CodeActionParams{Range: diagnostics[0].Range, Context: protocol.CodeActionContext{Diagnostics: diagnostics}}
			params.TextDocument.URI = uri
			actions := s.codeAction(context.Background(), params)
			require.Len(t, actions, 1)
			action := actions[0]
			action.Command = &protocol.CommandAction{Command: "stale.command"}
			response := s.replaceEditorConfiguration(context.Background(), projectconfig.Partial{Diagnostics: &config})
			require.Empty(t, response.Error)
			require.Empty(t, s.collectDiagnostics(context.Background(), doc))
			resolved := s.resolveCodeAction(context.Background(), action)
			require.NotNil(t, resolved.Disabled)
			require.Nil(t, resolved.Edit)
			require.Nil(t, resolved.Command)
			require.Empty(t, s.codeAction(context.Background(), params))
		})
	}
}

func TestWorkspacePlanRejectsOpenOutsideWorkspace(t *testing.T) {
	s := NewServer(nil, t.TempDir(), "test")
	t.Cleanup(func() { require.NoError(t, s.CloseAll()) })
	uri := uriutil.FileURI(filepath.Join(t.TempDir(), "outside.yaml"))
	s.documentManager.OpenDocument(uri, "value: bad\n", 1)
	version := 1
	builder := rewrite.NewBuilder("value: bad\n")
	require.NoError(t, builder.Insert(0, "# changed\n"))
	edits, err := builder.Finish()
	require.NoError(t, err)
	plan := rewrite.WorkspacePlan{Documents: []rewrite.DocumentPlan{rewrite.NewDocumentPlan(uri, &version, "value: bad\n", edits)}}
	require.ErrorContains(t, s.validateWorkspacePlan(context.Background(), plan), "outside the workspace")
	_, err = (serverDocumentResolver{server: s}).ResolveDocument(context.Background(), uri)
	require.ErrorContains(t, err, "outside the workspace")
}
