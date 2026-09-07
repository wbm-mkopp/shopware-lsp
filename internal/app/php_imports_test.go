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

func TestPHPImportDiagnosticsActionsAndConfiguration(t *testing.T) {
	for _, mode := range []string{"enabled", "rule-off", "domain-off", "framework", "severity"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("SHOPWARE_LSP_CACHE_DIR", t.TempDir())
			root := t.TempDir()
			server := lsp.NewServer(nil, root, "test")
			server.SetWorkspaceFactory(func(ctx context.Context, root string, current *lsp.Server) (lsp.WorkspaceRuntime, error) {
				return NewWorkspace(ctx, root, current)
			})
			client, published := startAdministrationLifecycleClient(t, server)
			options := map[string]any{}
			switch mode {
			case "rule-off":
				options["configuration"] = map[string]any{"diagnostics": map[string]any{"rules": map[string]string{"php.unusedImport": "off"}}}
			case "severity":
				options["configuration"] = map[string]any{"diagnostics": map[string]any{"rules": map[string]string{"php.unusedImport": "warning"}}}
			case "domain-off":
				options["configuration"] = map[string]any{"domains": map[string]bool{"php": false}}
			case "framework":
				options["shopwareClient"] = map[string]any{"protocolVersion": lsp.ClientProtocolVersion, "presentationProfile": "framework", "supportedCommands": []string{}}
			}
			var initialized map[string]any
			require.NoError(t, client.Call(context.Background(), "initialize", map[string]any{"rootUri": uriutil.FileURI(root), "initializationOptions": options, "capabilities": map[string]any{"textDocument": map[string]any{"codeAction": map[string]any{"dataSupport": true, "resolveSupport": map[string]any{"properties": []string{"edit"}}}}}}, &initialized))
			uri := uriutil.FileURI(filepath.Join(root, "Live.php"))
			source := "<?php\nuse Vendor\\Unused;\nclass Live {}\n"
			notifyAdministrationDocumentOpen(t, client, uri, 1, source)
			publication := waitForAdministrationDiagnostics(t, published, uri)[uri]
			var diagnostic *protocol.Diagnostic
			for _, item := range publication.Diagnostics {
				if item.Code == "php.unusedImport" {
					copy := item
					diagnostic = &copy
				}
			}
			if mode == "enabled" || mode == "severity" {
				require.NotNil(t, diagnostic)
				severity := protocol.DiagnosticSeverityHint
				if mode == "severity" {
					severity = protocol.DiagnosticSeverityWarning
				}
				assert.Equal(t, severity, diagnostic.Severity)
			} else {
				assert.Nil(t, diagnostic)
			}
			params := map[string]any{"textDocument": map[string]any{"uri": uri}, "range": protocol.Range{Start: protocol.Position{Line: 1}, End: protocol.Position{Line: 1, Character: 18}}, "context": map[string]any{"only": []string{"source.organizeImports"}, "diagnostics": []protocol.Diagnostic{}}}
			var actions []protocol.CodeAction
			require.NoError(t, client.Call(context.Background(), "textDocument/codeAction", params, &actions))
			if mode == "domain-off" || mode == "framework" {
				assert.Empty(t, actions)
				return
			}
			require.Len(t, actions, 1)
			assert.Equal(t, "Organize Imports", actions[0].Title)
			require.NotNil(t, actions[0].Edit)
			if diagnostic == nil {
				return
			}
			actions = nil
			params["context"] = map[string]any{"only": []string{"quickfix"}, "diagnostics": []protocol.Diagnostic{*diagnostic}}
			require.NoError(t, client.Call(context.Background(), "textDocument/codeAction", params, &actions))
			require.Len(t, actions, 1)
			assert.Equal(t, "Remove unused import 'Unused'", actions[0].Title)
			require.Nil(t, actions[0].Edit)
			require.NotNil(t, actions[0].Data)
			var resolved protocol.CodeAction
			require.NoError(t, client.Call(context.Background(), "codeAction/resolve", actions[0], &resolved))
			require.NotNil(t, resolved.Edit)
			require.NoError(t, client.Notify(context.Background(), "textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": uri, "version": 2}, "contentChanges": []map[string]any{{"text": source + "new Unused;\n"}}}))
			changed := waitForAdministrationDiagnostics(t, published, uri)[uri]
			for _, item := range changed.Diagnostics {
				assert.NotEqual(t, "php.unusedImport", item.Code)
			}
			require.NoError(t, client.Call(context.Background(), "codeAction/resolve", actions[0], &resolved))
			require.NotNil(t, resolved.Disabled)
		})
	}
}
