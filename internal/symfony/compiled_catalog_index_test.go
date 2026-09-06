package symfony

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/shopware/shopware-lsp/internal/indexer"
	"github.com/stretchr/testify/require"
)

func TestCompiledContainerScannerPersistenceAndRemoval(t *testing.T) {
	root, cache := t.TempDir(), t.TempDir()
	path := filepath.Join(root, "var", "cache", "dev_test", "Shopware_Core_KernelDevDebugContainer.xml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(`<container><services><service id="compiled.only" class="App\First"/></services></container>`), 0o644))
	open := func() (*ServiceIndex, *CompiledCatalogIndex, *indexer.FileScanner, func()) {
		store, err := indexer.NewStore(filepath.Join(cache, "indexes.db"))
		require.NoError(t, err)
		services, err := NewServiceIndex(root, cache, store)
		require.NoError(t, err)
		generated, err := NewCompiledContainerIndex(root, cache, services, store)
		require.NoError(t, err)
		scanner, err := indexer.NewFileScanner(root, filepath.Join(cache, "scanner.db"), store)
		require.NoError(t, err)
		scanner.AddIndexer(generated)
		closeAll := func() {
			require.NoError(t, scanner.Close())
			require.NoError(t, generated.Close())
			require.NoError(t, services.Close())
			require.NoError(t, store.Close())
		}
		return services, generated, scanner, closeAll
	}
	services, generated, scanner, closeAll := open()
	require.False(t, services.containerWatcher.ContainerExists(), "construction does not read generated files")
	require.True(t, generated.ShouldEnterDirectory(filepath.Join(root, "var", "cache", "dev_test")))
	require.False(t, generated.ShouldEnterDirectory(filepath.Join(root, "var", "cache", "dev_test", "pools")))
	require.NoError(t, scanner.IndexAll(context.Background()))
	service, found, err := services.GetServiceByID("compiled.only")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "App\\First", service.Class)
	closeAll()
	services, _, scanner, closeAll = open()
	defer closeAll()
	service, found, err = services.GetServiceByID("compiled.only")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "App\\First", service.Class)
	require.NoError(t, os.WriteFile(path, []byte(`<container><services><service id="compiled.changed" class="App\Second"/></services></container>`), 0o644))
	require.NoError(t, scanner.IndexFiles(context.Background(), []string{path}))
	_, found, err = services.GetServiceByID("compiled.only")
	require.NoError(t, err)
	require.False(t, found)
	_, found, err = services.GetServiceByID("compiled.changed")
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, os.Remove(path))
	require.NoError(t, scanner.IndexAll(context.Background()))
	_, found, err = services.GetServiceByID("compiled.changed")
	require.NoError(t, err)
	require.False(t, found)
	require.False(t, services.containerWatcher.ContainerExists())
	require.NoError(t, os.WriteFile(path, []byte(`<container><services><service id="compiled.recreated"/></services></container>`), 0o644))
	require.NoError(t, scanner.IndexFiles(context.Background(), []string{path}))
	_, found, err = services.GetServiceByID("compiled.recreated")
	require.NoError(t, err)
	require.True(t, found)
}
