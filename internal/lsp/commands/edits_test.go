package commands

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/shopware/shopware-lsp/internal/extension"
	"github.com/shopware/shopware-lsp/internal/indexer"
	"os"
	"path/filepath"
	"testing"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"github.com/shopware/shopware-lsp/internal/uriutil"
	"github.com/stretchr/testify/require"
)

type editTestHost struct {
	snapshots map[string]lsp.DocumentSnapshot
	plan      rewrite.WorkspacePlan
	reject    string
}

func (h *editTestHost) ResolveDocument(_ context.Context, uri string) (lsp.DocumentSnapshot, error) {
	if uri == h.reject {
		return lsp.DocumentSnapshot{}, errors.New("outside workspace")
	}
	snapshot, ok := h.snapshots[uri]
	if !ok {
		return snapshot, os.ErrNotExist
	}
	return snapshot, nil
}
func (h *editTestHost) WorkspaceEdit(_ context.Context, plan rewrite.WorkspacePlan) (*protocol.WorkspaceEdit, error) {
	h.plan = plan
	return plan.WorkspaceEdit()
}
func (h *editTestHost) ResourcePaths(context.Context, string, int) ([]string, error) { return nil, nil }

func TestSnippetPlanPreservesUnsavedTargetAndDefersCreation(t *testing.T) {
	root := t.TempDir()
	existing := filepath.Join(root, "en.json")
	created := filepath.Join(root, "de.json")
	require.NoError(t, os.WriteFile(existing, []byte(`{"disk":true}`), 0o644))
	uri := uriutil.FileURI(existing)
	version := 8
	host := &editTestHost{snapshots: map[string]lsp.DocumentSnapshot{uri: {Document: lsp.NewTextDocument(uri, `{"unsaved":"ä"}`, version), Version: &version}}}
	provider := NewSnippetCommandProvider(nil, host)
	value, err := provider.createSnippets(context.Background(), "new", []SnippetFile{{Path: existing, Value: "English"}, {Path: created, Value: "Deutsch"}})
	require.NoError(t, err)
	require.NotNil(t, value.(EditResponse).Edit)
	require.Len(t, host.plan.Documents, 1)
	require.Equal(t, &version, host.plan.Documents[0].Version)
	updated, err := host.plan.Documents[0].Apply()
	require.NoError(t, err)
	require.JSONEq(t, `{"unsaved":"ä","new":"English"}`, updated)
	require.Len(t, host.plan.Creates, 1)
	require.JSONEq(t, `{"new":"Deutsch"}`, host.plan.Creates[0].Content)
	disk, err := os.ReadFile(existing)
	require.NoError(t, err)
	require.JSONEq(t, `{"disk":true}`, string(disk))
	_, err = os.Stat(created)
	require.ErrorIs(t, err, os.ErrNotExist)
}
func TestSnippetPlanFailureDoesNotWriteEarlierFiles(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "en.json")
	rejected := filepath.Join(root, "outside.json")
	require.NoError(t, os.WriteFile(path, []byte(`{}`), 0o644))
	host := &editTestHost{reject: uriutil.FileURI(rejected)}
	_, err := NewSnippetCommandProvider(nil, host).createSnippets(context.Background(), "key", []SnippetFile{{Path: path, Value: "first"}, {Path: rejected, Value: "second"}})
	require.Error(t, err)
	require.Empty(t, host.plan.Documents)
	require.Empty(t, host.plan.Creates)
	disk, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "{}", string(disk))
}

func TestTwigBlockPlanPreservesUnsavedTemplate(t *testing.T) {
	root := t.TempDir()
	extensions, err := extension.NewExtensionIndexer(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, extensions.Close()) })
	plugin := filepath.Join(root, "src", "DemoPlugin.php")
	require.NoError(t, extensions.Index(indexer.NewParsedFile(plugin, []byte(`<?php
use Shopware\Core\Framework\Plugin;
class DemoPlugin extends Plugin {}
`))))
	target := filepath.Join(root, "src", "Resources", "views", "storefront", "page.html.twig")
	uri := uriutil.FileURI(target)
	version := 3
	host := &editTestHost{snapshots: map[string]lsp.DocumentSnapshot{uri: {Document: lsp.NewTextDocument(uri, "{# unsaved ä #}\n", version), Version: &version}}}
	provider := NewTwigCommandProvider(root, extensions, nil, host)
	raw := json.RawMessage(`{"textUri":"` + uriutil.FileURI(filepath.Join(root, "vendor", "Resources", "views", "storefront", "page.html.twig")) + `","blockName":"page_content","extension":"DemoPlugin"}`)
	result, err := provider.extendBlock(context.Background(), &raw)
	require.NoError(t, err)
	require.NotNil(t, result.(map[string]any)["edit"])
	require.Len(t, host.plan.Documents, 1)
	require.Equal(t, &version, host.plan.Documents[0].Version)
	content, err := host.plan.Documents[0].Apply()
	require.NoError(t, err)
	require.Contains(t, content, "{# unsaved ä #}")
	require.Contains(t, content, "{% block page_content %}")
	_, err = os.Stat(target)
	require.ErrorIs(t, err, os.ErrNotExist)
}
