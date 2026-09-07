package app

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/uriutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPHPExtractMethodRoutingAndConfiguration(t *testing.T) {
	for _, mode := range []string{"enabled", "domain-off", "framework"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("SHOPWARE_LSP_CACHE_DIR", t.TempDir())
			root := t.TempDir()
			server := lsp.NewServer(nil, root, "test")
			server.SetWorkspaceFactory(func(ctx context.Context, root string, current *lsp.Server) (lsp.WorkspaceRuntime, error) {
				return NewWorkspace(ctx, root, current)
			})
			client, published := startAdministrationLifecycleClient(t, server)
			options := map[string]any{}
			if mode == "domain-off" {
				options["configuration"] = map[string]any{"domains": map[string]bool{"php": false}}
			}
			if mode == "framework" {
				options["shopwareClient"] = map[string]any{"protocolVersion": lsp.ClientProtocolVersion, "presentationProfile": "framework", "supportedCommands": []string{}}
			}
			var initialized map[string]any
			require.NoError(t, client.Call(context.Background(), "initialize", map[string]any{"rootUri": uriutil.FileURI(root), "initializationOptions": options}, &initialized))
			uri := uriutil.FileURI(filepath.Join(root, "Live.php"))
			notifyAdministrationDocumentOpen(t, client, uri, 1, "<?php class Demo { public function run($a) {\nreturn $a;\n} }")
			waitForAdministrationDiagnostics(t, published, uri)
			params := map[string]any{"textDocument": map[string]any{"uri": uri}, "range": protocol.Range{Start: protocol.Position{Line: 1, Character: 0}, End: protocol.Position{Line: 1, Character: 0}}, "context": map[string]any{"only": []string{"refactor.extract"}, "diagnostics": []protocol.Diagnostic{}}}
			var actions []protocol.CodeAction
			require.NoError(t, client.Call(context.Background(), "textDocument/codeAction", params, &actions))
			if mode != "enabled" {
				assert.Empty(t, actions)
				return
			}
			require.Len(t, actions, 1)
			assert.Equal(t, "Extract method 'extractedMethod'", actions[0].Title)
			require.NotNil(t, actions[0].Edit)
			assert.EqualValues(t, 1, *actions[0].Edit.DocumentChanges[0].TextDocument.Version)
			require.NoError(t, client.Notify(context.Background(), "textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": uri, "version": 2}, "contentChanges": []map[string]any{{"text": "<?php class Demo { public function run($b) {\nreturn $b;\n} private function extractedMethod() {} }"}}}))
			waitForAdministrationDiagnostics(t, published, uri)
			actions = nil
			require.NoError(t, client.Call(context.Background(), "textDocument/codeAction", params, &actions))
			require.Len(t, actions, 1)
			assert.Equal(t, "Extract method 'extractedMethod2'", actions[0].Title)
			assert.EqualValues(t, 2, *actions[0].Edit.DocumentChanges[0].TextDocument.Version)
		})
	}
}
