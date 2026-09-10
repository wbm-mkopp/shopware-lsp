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

// Exercise production registration and routing with source that exists only in
// the editor. Close must remove it; re-opening must use the new source snapshot.
func TestPHPEditorBasicsLifecycleAndConfiguration(t *testing.T) {
	for _, mode := range []string{"enabled", "disabled", "framework"} {
		t.Run(mode, func(t *testing.T) {
			t.Setenv("SHOPWARE_LSP_CACHE_DIR", t.TempDir())
			root := t.TempDir()
			server := lsp.NewServer(nil, root, "test")
			server.SetWorkspaceFactory(func(ctx context.Context, root string, current *lsp.Server) (lsp.WorkspaceRuntime, error) {
				return NewWorkspace(ctx, root, current)
			})
			client, published := startAdministrationLifecycleClient(t, server)
			options := map[string]any{}
			if mode == "framework" {
				options["shopwareClient"] = map[string]any{"protocolVersion": lsp.ClientProtocolVersion, "presentationProfile": "framework", "supportedCommands": []string{}}
			}
			if mode == "disabled" {
				options["configuration"] = map[string]any{"features": map[string]bool{"documentSymbols": false, "documentHighlights": false, "foldingRanges": false, "selectionRanges": false}}
			}
			var initialized map[string]any
			require.NoError(t, client.Call(context.Background(), "initialize", map[string]any{"rootUri": uriutil.FileURI(root), "initializationOptions": options}, &initialized))
			uri := uriutil.FileURI(filepath.Join(root, "Live.php"))
			source := "<?php\nclass First {\n function run($value) {\n  return $value;\n }\n}\n"
			notifyAdministrationDocumentOpen(t, client, uri, 1, source)
			waitForAdministrationDiagnostics(t, published, uri)
			check := func(expectedName string, active bool) {
				t.Helper()
				base := map[string]any{"textDocument": map[string]any{"uri": uri}}
				var symbols []protocol.DocumentSymbol
				require.NoError(t, client.Call(context.Background(), "textDocument/documentSymbol", base, &symbols))
				var folds []protocol.FoldingRange
				require.NoError(t, client.Call(context.Background(), "textDocument/foldingRange", base, &folds))
				base["position"] = protocol.Position{Line: 3, Character: 10}
				var highlights []protocol.DocumentHighlight
				require.NoError(t, client.Call(context.Background(), "textDocument/documentHighlight", base, &highlights))
				base["positions"] = []protocol.Position{{Line: 3, Character: 10}}
				var selections []protocol.SelectionRange
				require.NoError(t, client.Call(context.Background(), "textDocument/selectionRange", base, &selections))
				if active {
					require.Len(t, symbols, 1)
					assert.Equal(t, expectedName, symbols[0].Name)
					require.Len(t, highlights, 2)
					assert.NotEmpty(t, folds)
					require.Len(t, selections, 1)
					assert.NotNil(t, selections[0].Parent)
				} else {
					assert.Empty(t, symbols)
					assert.Empty(t, highlights)
					assert.Empty(t, folds)
					assert.Empty(t, selections)
				}
			}
			check("First", mode == "enabled")
			changed := "<?php\nclass Changed {\n function run($other) {\n  return $other;\n }\n}\n"
			require.NoError(t, client.Notify(context.Background(), "textDocument/didChange", map[string]any{"textDocument": map[string]any{"uri": uri, "version": 2}, "contentChanges": []map[string]any{{"text": changed}}}))
			waitForAdministrationDiagnostics(t, published, uri)
			check("Changed", mode == "enabled")
			require.NoError(t, client.Notify(context.Background(), "textDocument/didClose", map[string]any{"textDocument": map[string]any{"uri": uri}}))
			waitForAdministrationDiagnostics(t, published, uri)
			check("", false)
			notifyAdministrationDocumentOpen(t, client, uri, 3, source)
			waitForAdministrationDiagnostics(t, published, uri)
			check("First", mode == "enabled")
		})
	}
}
