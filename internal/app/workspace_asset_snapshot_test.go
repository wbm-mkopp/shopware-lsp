package app

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shopware/shopware-lsp/internal/asset"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/stretchr/testify/require"
)

// The same asset probes serve the portable lifecycle test and real-world cold/
// restored checks, so adding a probe cannot silently omit restart coverage.
type workspaceAssetSnapshot struct {
	names, packages, encore, importmap, vite                               []string
	administration, theme                                                  []asset.Resource
	administrationUsages, htmlUsages, viteUsages, viteAdministrationUsages []asset.Usage
}

func captureWorkspaceAssets(t *testing.T, workspace *Workspace) workspaceAssetSnapshot {
	t.Helper()
	index := workspaceAssetIndex(t, workspace)
	var snapshot workspaceAssetSnapshot
	var err error
	snapshot.names, err = index.Names()
	require.NoError(t, err)
	snapshot.packages, err = index.PackageNames()
	require.NoError(t, err)
	snapshot.encore, err = index.EntryNames()
	require.NoError(t, err)
	snapshot.importmap, err = index.ImportmapEntryNames()
	require.NoError(t, err)
	snapshot.vite, err = index.ViteEntryNames()
	require.NoError(t, err)
	snapshot.administration, err = index.FindAssetsForPackage("administration/static/img/favicon/favicon-16x16.png", "@Administration")
	require.NoError(t, err)
	snapshot.theme, err = index.FindAssetsForPackage("assets/illustration/404_error.svg", "theme")
	require.NoError(t, err)
	snapshot.administrationUsages, err = index.Usages("@Administration", asset.AssetPackageReference)
	require.NoError(t, err)
	snapshot.htmlUsages, err = index.Usages("_webpack_hot_proxy_/storefront/hot-reloading.js", asset.AssetReference)
	require.NoError(t, err)
	snapshot.viteUsages, err = index.Usages("app", asset.ViteEntryReference)
	require.NoError(t, err)
	snapshot.viteAdministrationUsages, err = index.Usages("administration", asset.ViteEntryReference)
	require.NoError(t, err)
	return snapshot
}

func TestWorkspaceAssetLifecyclePreservesAndRemovesCatalogState(t *testing.T) {
	t.Setenv("SHOPWARE_LSP_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	files := map[string]string{
		"public/bundles/administration/administration/static/img/favicon/favicon-16x16.png": "fixture",
		"public/theme/test/assets/illustration/404_error.svg":                               "<svg/>",
		"public/build/app.css":                             "body{}",
		"public/build/entrypoints.json":                    `{"entrypoints":{"app":{"css":["/build/app.css"]}}}`,
		"src/Administration/Resources/config/services.xml": `<container><services><service id="assets.theme"><tag name="assets.package" package="theme"/></service><service id="assets.asset"><tag name="assets.package" package="asset"/></service></services></container>`,
		"assets/app.js":                                    "export default {};",
		"vite.config.ts":                                   `export default defineConfig({build:{rollupOptions:{input:{app:'./assets/app.js',administration:'./assets/app.js'}}}});`,
		"importmap.php":                                    `<?php return ['app' => ['path' => './assets/app.js', 'entrypoint' => true]];`,
		"templates/page.html.twig": `{{ asset('administration/static/img/favicon/favicon-16x16.png', '@Administration') }}
{{ asset('assets/illustration/404_error.svg', 'theme') }}
<script src="{{ asset('_webpack_hot_proxy_/storefront/hot-reloading.js') }}"></script>
{{ vite_entry_script_tags('app') }}{{ vite_entry_script_tags('administration') }}`,
	}
	for relative, source := range files {
		path := filepath.Join(root, relative)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(source), 0o644))
	}
	ctx := context.Background()
	open := func() *Workspace {
		server := lsp.NewServer(nil, root, "test")
		workspace, err := NewWorkspace(ctx, root, server)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, workspace.Close()); require.NoError(t, server.CloseAll()) })
		return workspace
	}
	workspace := open()
	require.NoError(t, workspace.Scanner().IndexAll(ctx))
	before := captureWorkspaceAssets(t, workspace)
	require.NotEmpty(t, before.names)
	require.ElementsMatch(t, []string{"@Administration", "asset", "theme"}, before.packages)
	require.Equal(t, []string{"app"}, before.encore)
	require.Equal(t, []string{"app"}, before.importmap)
	require.ElementsMatch(t, []string{"app", "administration"}, before.vite)
	require.Len(t, before.administration, 1)
	require.Len(t, before.theme, 1)
	require.Len(t, before.administrationUsages, 1)
	require.Len(t, before.htmlUsages, 1)
	require.Len(t, before.viteUsages, 1)
	require.Len(t, before.viteAdministrationUsages, 1)
	require.NoError(t, workspace.Close())

	reopened := open()
	require.Equal(t, before, captureWorkspaceAssets(t, reopened), "cache restore must not require rescanning")
	require.NoError(t, os.WriteFile(filepath.Join(root, "templates/page.html.twig"), []byte("Changed template"), 0o644))
	for _, relative := range []string{"vite.config.ts", "public/bundles/administration/administration/static/img/favicon/favicon-16x16.png"} {
		require.NoError(t, os.Remove(filepath.Join(root, relative)))
	}
	require.NoError(t, reopened.Scanner().IndexAll(ctx))
	after := captureWorkspaceAssets(t, reopened)
	require.Empty(t, after.vite)
	require.Empty(t, after.administration)
	require.Empty(t, after.administrationUsages)
	require.Empty(t, after.htmlUsages)
	require.Empty(t, after.viteUsages)
	require.Empty(t, after.viteAdministrationUsages)
	require.Equal(t, before.theme, after.theme)
	require.Equal(t, before.encore, after.encore)
	require.Equal(t, before.importmap, after.importmap)
	require.NoError(t, reopened.Close())
	restored := open()
	require.Equal(t, after, captureWorkspaceAssets(t, restored), "updates and removals must survive restart")
}
