package app

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProjectCacheIdentity(t *testing.T) {
	t.Setenv("SHOPWARE_LSP_CACHE_DIR", t.TempDir())
	root := t.TempDir()
	one, err := projectCacheFolder(filepath.Join(root, "a_b"))
	require.NoError(t, err)
	two, err := projectCacheFolder(filepath.Join(root, "a", "b"))
	require.NoError(t, err)
	require.NotEqual(t, one, two)
	again, err := projectCacheFolder(filepath.Join(root, "a_b") + string(filepath.Separator) + ".")
	require.NoError(t, err)
	require.Equal(t, one, again)
}
