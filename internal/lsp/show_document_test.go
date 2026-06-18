package lsp

import (
	"encoding/json"
	"testing"

	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseExtendBlockSuccess(t *testing.T) {
	uri, line, ok := parseExtendBlockSuccess(map[string]any{
		"uri":  "file:///project/custom/plugins/MyPlugin/Resources/views/storefront/foo.html.twig",
		"line": 12,
	})
	assert.True(t, ok)
	assert.Equal(t, "file:///project/custom/plugins/MyPlugin/Resources/views/storefront/foo.html.twig", uri)
	assert.Equal(t, 12, line)

	_, _, ok = parseExtendBlockSuccess(protocol.NewLspError("failed", "block.already_exists"))
	assert.False(t, ok)

	_, _, ok = parseExtendBlockSuccess(map[string]any{"line": 1})
	assert.False(t, ok)
}

func TestParseExtendBlockLine(t *testing.T) {
	assert.Equal(t, 5, parseExtendBlockLine(5))
	assert.Equal(t, 5, parseExtendBlockLine(int64(5)))
	assert.Equal(t, 5, parseExtendBlockLine(float64(5)))
	assert.Equal(t, 1, parseExtendBlockLine("invalid"))
}

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
