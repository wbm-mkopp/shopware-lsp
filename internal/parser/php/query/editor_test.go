package query

import (
	"github.com/shopware/shopware-lsp/internal/parser/php/parser"
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestConstantNamesExcludeTypesAndValues(t *testing.T) {
	source := "<?php class C { const string A = OTHER, B = Factory::NAME; }"
	parsed := parser.Parse(source)
	require.Equal(t, source, parsed.Tree.Root.Text())
	declarations := Nodes(parsed.Tree.Root, syntax.PhpClassConstDeclaration)
	require.Len(t, declarations, 1)
	names := ConstantNames(declarations[0])
	require.Len(t, names, 2)
	assert.Equal(t, "A", NameValue(names[0]))
	assert.Equal(t, "B", NameValue(names[1]))
}

func TestVariableWriteTargets(t *testing.T) {
	for _, tc := range []struct {
		source string
		writes []string
	}{
		{"$value = $read;", []string{"value"}},
		{"$value += $read;", []string{"value"}},
		{"$value++; ++$other;", []string{"value", "other"}},
		{"$array[$key] = $read;", []string{"array"}},
		{"[$a, $b] = $read;", []string{"a", "b"}},
		{"foreach ($read as $key => $value) {}", []string{"key", "value"}},
		{"$read->value = 1;", nil},
	} {
		t.Run(tc.source, func(t *testing.T) {
			source := "<?php " + tc.source
			parsed := parser.Parse(source)
			require.Equal(t, source, parsed.Tree.Root.Text())
			var names []string
			for _, node := range Nodes(parsed.Tree.Root, syntax.PhpVariable) {
				if VariableIsWrite(node) {
					names = append(names, VariableName(node))
				}
			}
			assert.Equal(t, tc.writes, names)
		})
	}
}
