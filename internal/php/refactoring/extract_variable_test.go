package refactoring

import (
	"context"
	"strings"
	"testing"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/parser/php/parser"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractVariable(t *testing.T) {
	for _, tc := range []struct{ name, source, selected, want string }{
		{"assignment", "<?php\n$x = build();", "build()", "<?php\n$extracted = build();\n$x = $extracted;"},
		{"return and indent", "<?php function f() {\n    return build();\n}", "build()", "<?php function f() {\n    $extracted = build();\n    return $extracted;\n}"},
		{"first operand", "<?php\nreturn build() + other();", "build()", "<?php\n$extracted = build();\nreturn $extracted + other();"},
		{"whole expression", "<?php\nreturn first() + second();", "first() + second()", "<?php\n$extracted = first() + second();\nreturn $extracted;"},
		{"collision", "<?php function f($extracted) { $extracted2 = 1; return build(); }", "build()", "<?php function f($extracted) { $extracted2 = 1; $extracted3 = build();\nreturn $extracted3; }"},
		{"CRLF", "<?php\r\n$x = build();", "build()", "<?php\r\n$extracted = build();\r\n$x = $extracted;"},
		{"comments", "<?php\n// explanation\nreturn build(/* keep */);", "build(/* keep */)", "<?php\n// explanation\n$extracted = build(/* keep */);\nreturn $extracted;"},
		{"standalone", "<?php\nbuild();", "build()", "<?php\n$extracted = build();\n$extracted;"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := parser.Parse(tc.source)
			require.Equal(t, tc.source, tree.Tree.Root.Text())
			start := uint32(strings.Index(tc.source, tc.selected))
			result, err := ExtractVariable(context.Background(), tree.Tree.Root, tc.source, cst.TextRange{Start: start, End: start + uint32(len(tc.selected))})
			require.NoError(t, err)
			require.NotNil(t, result)
			got, err := rewrite.Apply(tc.source, result.Edits)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
			assert.Empty(t, parser.Parse(got).Errors)
		})
	}
}

func TestExtractVariableRejectsUnsafeContexts(t *testing.T) {
	for _, source := range []string{
		"<?php return SELECTED() ?? 1;",
		"<?php consume(SELECTED());",
		"<?php $x =& SELECTED();",
		"<?php function &f() { return SELECTED(); }",
		"<?php class C { public mixed $p { &get { return SELECTED(); } } }",
		"<?php return (yield SELECTED());",
		"<?php $x = get_defined_vars(); return SELECTED();",
		"<?php $$name = 1; return SELECTED();",
	} {
		t.Run(source, func(t *testing.T) {
			tree := parser.Parse(source)
			start := uint32(strings.Index(source, "SELECTED()"))
			result, err := ExtractVariable(context.Background(), tree.Tree.Root, source, cst.TextRange{Start: start, End: start + 10})
			require.NoError(t, err)
			assert.Nil(t, result)
		})
	}
}

func TestExtractVariableCaretAndCancellation(t *testing.T) {
	source := "<?php return build();"
	tree := parser.Parse(source)
	result, err := ExtractVariable(context.Background(), tree.Tree.Root, source, cst.TextRange{Start: 15, End: 15})
	require.NoError(t, err)
	require.NotNil(t, result)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ExtractVariable(ctx, tree.Tree.Root, source, cst.TextRange{Start: 15, End: 15})
	require.ErrorIs(t, err, context.Canceled)
}

func BenchmarkExtractVariable(b *testing.B) {
	source := "<?php\n" + strings.Repeat("$existing = 1;\n", 1000) + "return build();"
	tree := parser.Parse(source)
	start := uint32(strings.LastIndex(source, "build"))
	b.ReportAllocs()
	for b.Loop() {
		_, err := ExtractVariable(context.Background(), tree.Tree.Root, source, cst.TextRange{Start: start, End: start})
		if err != nil {
			b.Fatal(err)
		}
	}
}
