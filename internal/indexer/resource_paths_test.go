package indexer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResourcePathsUsePersistedIndex(t *testing.T) {
	root := t.TempDir()
	cache := filepath.Join(t.TempDir(), "scanner.db")
	directory := filepath.Join(root, "config_%")
	path := filepath.Join(directory, "routes.yaml")
	require.NoError(t, os.MkdirAll(directory, 0o755))
	require.NoError(t, os.WriteFile(path, []byte("routes: {}"), 0o644))
	scanner, err := NewFileScanner(root, cache)
	require.NoError(t, err)
	require.NoError(t, scanner.IndexAll(context.Background()))
	require.NoError(t, scanner.Close())
	scanner, err = NewFileScanner(root, cache)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, scanner.Close()) })
	// A request returns the indexed generation even when disk has changed.
	require.NoError(t, os.Remove(path))
	paths, err := scanner.ResourcePaths(context.Background(), directory, 10)
	require.NoError(t, err)
	require.Equal(t, []string{path}, paths)
	require.NoError(t, scanner.RemoveFiles(context.Background(), []string{path}))
	paths, err = scanner.ResourcePaths(context.Background(), directory, 10)
	require.NoError(t, err)
	require.Empty(t, paths)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = scanner.ResourcePaths(ctx, directory, 10)
	require.ErrorIs(t, err, context.Canceled)
}

func BenchmarkResourcePaths(b *testing.B) {
	scanner, err := NewFileScanner(b.TempDir(), filepath.Join(b.TempDir(), "scanner.db"))
	require.NoError(b, err)
	defer func() { require.NoError(b, scanner.Close()) }()
	tx, err := scanner.db.Begin()
	require.NoError(b, err)
	for i := range 10000 {
		_, err = tx.Exec("INSERT INTO file_hashes(path,size,mtime) VALUES(?,?,?)", fmt.Sprintf("/project/config/%05d.yaml", i), 1, 1)
		require.NoError(b, err)
	}
	require.NoError(b, tx.Commit())
	b.ResetTimer()
	for b.Loop() {
		paths, err := scanner.ResourcePaths(context.Background(), "/project/config", 500)
		require.NoError(b, err)
		require.Len(b, paths, 500)
	}
}
