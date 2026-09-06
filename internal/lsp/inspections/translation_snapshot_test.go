package inspections

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/shopware/shopware-lsp/internal/indexer"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"github.com/shopware/shopware-lsp/internal/translation"
	"github.com/shopware/shopware-lsp/internal/uriutil"
	"github.com/stretchr/testify/require"
)

func TestTranslationFixPreservesUnsavedTarget(t *testing.T) {
	idx, err := translation.NewIndex(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, idx.Close()) })
	path := filepath.Join(t.TempDir(), "translations", "messages.en.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	disk := []byte("known: Known\n")
	require.NoError(t, os.WriteFile(path, disk, 0644))
	require.NoError(t, idx.Index(indexer.NewParsedFile(path, disk)))
	doc := lsp.NewTextDocument("file:///source.twig", "{{ 'missing'|trans }}", 1)
	anchor, err := rewrite.NewElementHandle(doc.URI, doc.Version, doc.SyntaxLanguage, doc.SyntaxTree.Root)
	require.NoError(t, err)
	uri := uriutil.FileURI(path)
	version := 2
	target := lsp.NewTextDocument(uri, "known: Unsaved\n", version)
	payload, err := json.Marshal(addTranslationPayload{Domain: "messages", Key: "missing", File: path})
	require.NoError(t, err)
	plan, err := (addTranslationFix{index: idx}).Build(context.Background(), lsp.FixContext{Document: doc, Anchor: anchor, FixPayload: payload, Documents: staticDocumentResolver{uri: {Document: target, Version: &version}}})
	require.NoError(t, err)
	require.Len(t, plan.Documents, 1)
	require.Equal(t, &version, plan.Documents[0].Version)
	updated, err := plan.Documents[0].Apply()
	require.NoError(t, err)
	require.Equal(t, "known: Unsaved\n'missing': 'missing'\n", updated)
	unchanged, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, disk, unchanged)
}
