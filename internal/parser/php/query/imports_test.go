package query

import (
	"testing"

	"github.com/shopware/shopware-lsp/internal/parser/php/parser"
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUseItemsRangesAndNormalization(t *testing.T) {
	source := "<?php use Vendor\\{Thing /* retained */ as Alias, function run, const FLAG,};"
	parsed := parser.Parse(source)
	require.Equal(t, source, parsed.Tree.Root.Text())
	nodes := Nodes(parsed.Tree.Root, syntax.PhpUseDeclaration)
	require.Len(t, nodes, 1)
	list, ok := UseItems(nodes[0])
	require.True(t, ok)
	require.Len(t, list.Items, 3)
	assert.Equal(t, "use Vendor\\Thing as Alias;", list.Items[0].Text)
	assert.Equal(t, "use function Vendor\\run;", list.Items[1].Text)
	assert.Equal(t, "use const Vendor\\FLAG;", list.Items[2].Text)
	for i, want := range []string{"Thing", "run", "FLAG"} {
		rng := list.Items[i].NameRange
		assert.Equal(t, want, source[rng.Start:rng.End])
	}
	rng := list.Items[0].Range
	assert.Equal(t, "Thing /* retained */ as Alias", source[rng.Start:rng.End])
}

func TestUseItemsRejectsIncompleteDeclarations(t *testing.T) {
	for _, source := range []string{
		"<?php use Vendor\\Thing",
		"<?php use Vendor\\{Thing,",
		"<?php use Vendor\\Thing as;",
		"<?php use Vendor\\Thing,;",
	} {
		t.Run(source, func(t *testing.T) {
			parsed := parser.Parse(source)
			require.Equal(t, source, parsed.Tree.Root.Text())
			for _, node := range Nodes(parsed.Tree.Root, syntax.PhpUseDeclaration) {
				_, ok := UseItems(node)
				assert.False(t, ok)
			}
		})
	}
}
