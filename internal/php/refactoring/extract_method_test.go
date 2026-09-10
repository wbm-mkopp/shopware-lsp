package refactoring

import (
	"context"
	"strings"
	"testing"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/parser/php/parser"
	"github.com/shopware/shopware-lsp/internal/parser/php/query"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func methodFixture(body string) string {
	return "<?php\nclass Demo\n{\n    public function run($a, $b)\n    {\n" + body + "\n    }\n}\n"
}
func extractMethodTest(t *testing.T, source, selected string) (*Extraction, string) {
	t.Helper()
	parsed := parser.Parse(source)
	require.Equal(t, source, parsed.Tree.Root.Text())
	require.Empty(t, parsed.Errors)
	start := strings.Index(source, selected)
	require.GreaterOrEqual(t, start, 0)
	result, err := ExtractMethod(context.Background(), parsed.Tree.Root, source, cst.TextRange{Start: uint32(start), End: uint32(start + len(selected))}, nil)
	require.NoError(t, err)
	if result == nil {
		return nil, ""
	}
	got, err := rewrite.Apply(source, result.Edits)
	require.NoError(t, err)
	require.Empty(t, parser.Parse(got).Errors, got)
	return result, got
}

func TestExtractMethodInputsAndLiveOutput(t *testing.T) {
	selected := "$sum = $a + $b;\n        $result = $sum * 2;"
	source := methodFixture("        " + selected + "\n        return $result;")
	result, got := extractMethodTest(t, source, selected)
	require.NotNil(t, result)
	assert.Equal(t, methodFixture("        $result = $this->extractedMethod($a, $b);\n        return $result;")[:strings.LastIndex(methodFixture("        $result = $this->extractedMethod($a, $b);\n        return $result;"), "\n}")]+"\n\n    private function extractedMethod($a, $b)\n    {\n        $sum = $a + $b;\n        $result = $sum * 2;\n        return $result;\n    }\n}\n", got)
}

func TestExtractMethodReturnsStaticAndNames(t *testing.T) {
	for _, tc := range []struct{ name, source, selected, call, signature string }{
		{"return", methodFixture("        return $a + $b;"), "return $a + $b;", "return $this->extractedMethod($a, $b);", "private function extractedMethod($a, $b)"},
		{"static", strings.Replace(methodFixture("        return $a + 1;"), "public function", "public static function", 1), "return $a + 1;", "return self::extractedMethod($a);", "private static function extractedMethod($a)"},
		{"dynamic call input", methodFixture("        return $this->$a();"), "return $this->$a();", "return $this->extractedMethod($a);", "private function extractedMethod($a)"},
		{"this", methodFixture("        return $this->calculate();"), "return $this->calculate();", "return $this->extractedMethod();", "private function extractedMethod()"},
		{"bare return", methodFixture("        echo $a;\n        return;"), "echo $a;\n        return;", "$this->extractedMethod($a);\n        return;", "private function extractedMethod($a)"},
		{"existing name", strings.Replace(methodFixture("        echo $a;"), "class Demo", "/* extractedMETHOD */ class Demo", 1), "echo $a;", "$this->extractedMethod2($a);", "private function extractedMethod2($a)"},
		{"prefix local", methodFixture("        $local = 1;\n        echo $local;"), "echo $local;", "$this->extractedMethod($local);", "private function extractedMethod($local)"},
		{"reference-call output", methodFixture("        mutate($a);\n        return $a;"), "mutate($a);", "$a = $this->extractedMethod($a);", "private function extractedMethod(&$a)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, got := extractMethodTest(t, tc.source, tc.selected)
			require.NotNil(t, result)
			assert.Contains(t, got, tc.call)
			assert.Contains(t, got, tc.signature)
		})
	}
}

func TestExtractMethodRejectsUnsafeSelections(t *testing.T) {
	for _, tc := range []struct{ body, selected string }{
		{"return $a; echo $b;", "return $a; echo $b;"},
		{"echo $unknown;", "echo $unknown;"},
		{"$x = $a + $b;", "$a + $b"},
		{"echo __METHOD__;", "echo __METHOD__;"},
		{"global $a; echo $a;", "echo $a;"},
		{"echo $a; eval($b);", "echo $a;"},
	} {
		t.Run(tc.body, func(t *testing.T) {
			result, _ := extractMethodTest(t, methodFixture("        "+tc.body), tc.selected)
			assert.Nil(t, result)
		})
	}
}

func TestExtractMethodCaretCommentsCRLFAndCancellation(t *testing.T) {
	source := strings.ReplaceAll(methodFixture("        $x = /* keep */ $a + $b;\n        return $x;"), "\n", "\r\n")
	parsed := parser.Parse(source)
	start := uint32(strings.Index(source, "$x ="))
	result, err := ExtractMethod(context.Background(), parsed.Tree.Root, source, cst.TextRange{Start: start + 3, End: start + 3}, func(_ *cst.Node, name string) bool { return name != "extractedMethod" })
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "extractedMethod2", result.Name)
	got, err := rewrite.Apply(source, result.Edits)
	require.NoError(t, err)
	assert.Contains(t, got, "/* keep */")
	assert.NotContains(t, strings.ReplaceAll(got, "\r\n", ""), "\n")
	assert.Len(t, query.Methods(query.Classes(parser.Parse(got).Tree.Root)[0]), 2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = ExtractMethod(ctx, parsed.Tree.Root, source, cst.TextRange{}, nil)
	assert.ErrorIs(t, err, context.Canceled)
}

func BenchmarkExtractMethod(b *testing.B) {
	source := methodFixture(strings.Repeat("        $local = 1;\n", 1000) + "        return $a + $b;")
	parsed := parser.Parse(source)
	start := uint32(strings.LastIndex(source, "return"))
	b.ReportAllocs()
	for b.Loop() {
		result, err := ExtractMethod(context.Background(), parsed.Tree.Root, source, cst.TextRange{Start: start, End: start}, nil)
		if err != nil || result == nil {
			b.Fatal(err)
		}
	}
}
