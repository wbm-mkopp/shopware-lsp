package lsp

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFocusExtendedBlockArgs(t *testing.T) {
	uri, line, err := parseFocusExtendedBlockArgs([]json.RawMessage{
		json.RawMessage(`"file:///project/custom/plugins/MyPlugin/Resources/views/storefront/foo.html.twig"`),
		json.RawMessage(`5`),
	})
	require.NoError(t, err)
	assert.Equal(t, "file:///project/custom/plugins/MyPlugin/Resources/views/storefront/foo.html.twig", uri)
	assert.Equal(t, 5, line)

	_, _, err = parseFocusExtendedBlockArgs([]json.RawMessage{json.RawMessage(`"file:///foo.html.twig"`)})
	assert.Error(t, err)
}
