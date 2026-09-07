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

func TestExtractVariablePreservesEvaluationPoint(t *testing.T) {
	for _, source := range []string{
		"<?php return first() + SELECTED();",
		"<?php return $a && SELECTED();",
		"<?php return $a ? SELECTED() : 1;",
		"<?php return $a ?? SELECTED();",
		"<?php while (SELECTED()) {}",
		"<?php for (;SELECTED();) {}",
		"<?php if ($a) return SELECTED();",
		"<?php $a[other()] = SELECTED();",
		"<?php $f = fn() => SELECTED();",
		"<?php consume(first(), SELECTED(), last());",
	} {
		t.Run(source, func(t *testing.T) {
			tree := parser.Parse(source)
			start := uint32(strings.Index(source, "SELECTED()"))
			result, err := ExtractVariable(context.Background(), tree.Tree.Root, source, cst.TextRange{Start: start, End: start + 10}, VariableOptions{CallReturnsValue: func(*cst.Node) bool { return true }})
			require.NoError(t, err)
			require.NotNil(t, result)
			got, err := rewrite.Apply(source, result.Edits)
			require.NoError(t, err)
			assert.Equal(t, strings.Replace(source, "SELECTED()", "($extracted = SELECTED())", 1), got)
			assert.Empty(t, parser.Parse(got).Errors)
		})
	}
}

func TestExtractVariableAllOccurrencesAndMutationBarriers(t *testing.T) {
	for _, tc := range []struct {
		source string
		count  int
	}{
		{"<?php\n$x = $a + 1;\n$y = $a+1;\nreturn $a + 1;", 3},
		{"<?php\n$x = $a + 1;\n$a = 2;\n$y = $a + 1;", 0},
		{"<?php\n$x = $a + 1;\nmutate($a);\n$y = $a + 1;", 0},
		{"<?php\n$x = $a + 1;\nif ($a) { $y = $a + 1; }", 0},
		{"<?php\n$x = ($a + 1) + ($a + 1);", 2},
		{"<?php consume($a + 1, $a+1);", 2},
		{"<?php consume($a && ($a + 1), $a+1);", 0},
		{"<?php $x = $a + 1; $y = $a /* keep */ + 1;", 0},
	} {
		t.Run(tc.source, func(t *testing.T) {
			tree := parser.Parse(tc.source)
			start := uint32(strings.Index(tc.source, "$a + 1"))
			result, err := ExtractVariable(context.Background(), tree.Tree.Root, tc.source, cst.TextRange{Start: start, End: start + 6}, VariableOptions{AllOccurrences: true})
			require.NoError(t, err)
			if tc.count == 0 {
				assert.Nil(t, result)
				return
			}
			require.NotNil(t, result)
			assert.Equal(t, tc.count, result.Occurrences)
			got, err := rewrite.Apply(tc.source, result.Edits)
			require.NoError(t, err)
			expectedMentions := tc.count + 1
			if strings.Contains(tc.source, "consume") {
				expectedMentions--
			}
			assert.Equal(t, expectedMentions, strings.Count(got, "$extracted"))
			assert.Empty(t, parser.Parse(got).Errors)
		})
	}
}

func TestExtractMethodStructuredFlowAndMultipleOutputs(t *testing.T) {
	for _, tc := range []struct{ body, selected, call string }{
		{"if ($a) { echo $b; }", "if ($a) { echo $b; }", "$this->extractedMethod($a, $b);"},
		{"if ($a) { echo $b; }", "echo $b;", "$this->extractedMethod($b);"},
		{"$x = $a; $y = $b; echo $x; echo $y;", "$x = $a; $y = $b;", "[$x, $y] = $this->extractedMethod($a, $b);"},
		{"if ($a) { $x = 1; } else { $x = 2; } echo $x;", "if ($a) { $x = 1; } else { $x = 2; }", "$x = $this->extractedMethod($a);"},
		{"if ($a) { $b = 1; } echo $b;", "if ($a) { $b = 1; }", "$b = $this->extractedMethod($a, $b);"},
		{"$sum = 0; foreach ($a as $item) { $sum += $item; } echo $sum;", "foreach ($a as $item) { $sum += $item; }", "$sum = $this->extractedMethod($a, $sum);"},
		{"foreach ($a as $item) { echo $item; }", "echo $item;", "$this->extractedMethod($item);"},
		{"$sum = 0; for ($i = 0; $i < $a; $i++) { $sum += $i; } echo $sum;", "for ($i = 0; $i < $a; $i++) { $sum += $i; }", "$sum = $this->extractedMethod($a, $sum);"},
		{"while ($a) { $a--; if ($a == 1) break; } echo $a;", "while ($a) { $a--; if ($a == 1) break; }", "$a = $this->extractedMethod($a);"},
		{"if ($a) { return 1; } else { return 2; }", "if ($a) { return 1; } else { return 2; }", "return $this->extractedMethod($a);"},
		{"echo \"Hello $a\";", "echo \"Hello $a\";", "$this->extractedMethod($a);"},
		{"$a[0] = 1;", "$a[0] = 1;", "$a = $this->extractedMethod($a);"},
		{"$this->value = $a;", "$this->value = $a;", "$this->extractedMethod($a);"},
	} {
		t.Run(tc.selected, func(t *testing.T) {
			result, got := extractMethodTest(t, methodFixture("        "+tc.body), tc.selected)
			require.NotNil(t, result)
			assert.Contains(t, got, tc.call)
		})
	}
}

func TestExtractMethodKeepsUnsafeBoundaryUnavailable(t *testing.T) {
	for _, tc := range []struct{ body, selected string }{
		{"if ($a) { $x = 1; } echo $x;", "if ($a) { $x = 1; }"},
		{"while ($a) { break; }", "break;"},
		{"while ($a) { continue 2; }", "while ($a) { continue 2; }"},
		{"$fn = fn() => $a; echo $b;", "$fn = fn() => $a;"},
		{"$x = new Trace; if ($a) return; echo $b;", "$x = new Trace; if ($a) return;"},
	} {
		t.Run(tc.selected, func(t *testing.T) {
			result, _ := extractMethodTest(t, methodFixture("        "+tc.body), tc.selected)
			assert.Nil(t, result)
		})
	}
}

func TestExtractFunctionReferencesAndLiteralContents(t *testing.T) {
	source := "<?php namespace Demo;\nfunction run(&$a) {\n    $a++;\n    echo \"first\n  second $a\";\n}\n"
	selected := "$a++;\n    echo \"first\n  second $a\";"
	result, got := extractMethodTest(t, source, selected)
	require.NotNil(t, result)
	assert.Equal(t, "extractedFunction", result.Name)
	assert.Contains(t, got, "function extractedFunction(&$a)")
	assert.Contains(t, got, "\"first\n  second $a\"")
	assert.Contains(t, got, "$a = extractedFunction($a);")
	assert.NotContains(t, got, "private function")
}

func BenchmarkExtractMethodEarlyReturns(b *testing.B) {
	body := strings.Repeat("        if ($a) { return 1; }\n", 100) + "        echo $b;"
	source := methodFixture(body + "\n        return 0;")
	tree := parser.Parse(source)
	start := uint32(strings.Index(source, "if ($a)"))
	end := uint32(strings.Index(source, "echo $b;") + len("echo $b;"))
	b.ReportAllocs()
	for b.Loop() {
		result, err := ExtractMethod(context.Background(), tree.Tree.Root, source, cst.TextRange{Start: start, End: end}, nil)
		if err != nil || result == nil {
			b.Fatal(err)
		}
	}
}
