package query

import (
	"testing"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/parser/php/parser"
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStringVariableInputs(t *testing.T) {
	for _, tc := range []struct {
		literal string
		names   []string
		valid   bool
	}{
		{`"Hello ($name), {$user->name}"`, []string{"$name", "$user"}, true},
		{`'$literal'`, nil, true},
		{`b'$literal'`, nil, true},
		{`"\$literal and $real"`, []string{"$real"}, true},
		{"<<<'TXT'\n$literal\nTXT", nil, true},
		{"<<<TXT\n$real\nTXT", []string{"$real"}, true},
		{`"${dynamic}"`, nil, false},
		{`"{$object->method()}"`, nil, false},
		{`"{$object[__FUNCTION__]}"`, nil, false},
	} {
		t.Run(tc.literal, func(t *testing.T) {
			source := "<?php echo " + tc.literal + ";"
			parsed := parser.Parse(source)
			require.Equal(t, source, parsed.Tree.Root.Text())
			found := false
			for element := range parsed.Tree.Root.Descendants() {
				if token, ok := element.(*cst.Token); ok && token.Kind() == syntax.TkString {
					found = true
					names, valid := StringVariables(token)
					assert.Equal(t, tc.valid, valid)
					if valid {
						assert.Equal(t, tc.names, names)
					}
				}
			}
			require.True(t, found)
		})
	}
}
