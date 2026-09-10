package imports

import (
	"context"
	"strings"
	"testing"

	"github.com/shopware/shopware-lsp/internal/parser/php/parser"
	"github.com/shopware/shopware-lsp/internal/php/binder"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func analyzeTest(t *testing.T, source string) Analysis {
	t.Helper()
	tree := parser.Parse(source)
	require.Equal(t, source, tree.Tree.Root.Text())
	document := binder.New().Bind("/live.php", 1, tree.Tree.Root)
	analysis, err := Analyze(context.Background(), tree.Tree.Root, document)
	require.NoError(t, err)
	return analysis
}
func unused(analysis Analysis) []string {
	var result []string
	for _, scope := range analysis.Scopes {
		for _, declaration := range scope.Declarations {
			for _, item := range declaration.Items {
				if !item.Used {
					result = append(result, item.Alias)
				}
			}
		}
	}
	return result
}
func TestImportUsage(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		unused       []string
	}{
		{"classes and aliases", `<?php use Vendor\Thing as Alias; use Vendor\Unused; new Alias();`, []string{"Unused"}},
		{"unresolved references", `<?php use Missing\Thing; function run(Thing $a): Thing { return new Thing; }`, nil},
		{"fully qualified", `<?php use Vendor\Thing; new \Vendor\Thing();`, []string{"Thing"}},
		{"namespace relative", `<?php namespace App; use App\Thing; new namespace\Thing();`, []string{"Thing"}},
		{"prefix aliases", `<?php use Vendor\Package as P; P\run(); echo P\VALUE;`, nil},
		{"function and const", `<?php use function Vendor\same; use const Vendor\same; same();`, []string{"same"}},
		{"constant usage", `<?php use function Vendor\same; use const Vendor\same; echo same;`, []string{"same"}},
		{"case sensitivity", `<?php use Vendor\THING; use function Vendor\RUN; use const Vendor\VALUE; new thing; run(); echo value;`, []string{"VALUE"}},
		{"grouped mixed", `<?php use Vendor\{Thing as T, Other, function run, const FLAG}; new T; run(); echo FLAG;`, []string{"Other"}},
		{"type attribute trait", `<?php use Vendor\A; use Vendor\B; use Vendor\C; #[A] class T extends B { use C; }`, nil},
		{"phpdoc", `<?php use Vendor\Type; use Vendor\Collection; /** @return Collection<Type> */ function get() {}`, nil},
		{"phpdoc annotation", `<?php use Doctrine\ORM\Mapping as ORM; /** @ORM\Entity */ class Entity {}`, nil},
		{"phpdoc only fully qualified", `<?php use Vendor\Type; /** @return \Vendor\Type */ function get() {}`, []string{"Type"}},
		{"annotation class string", `<?php use Vendor\Entity; use Doctrine\ORM\Mapping as ORM; /** @ORM\ManyToOne(targetEntity="Entity") */ class C {}`, nil},
		{"catch and instanceof", `<?php use Vendor\Problem; use Vendor\Thing; try {} catch (Problem $e) {} $is = $e instanceof Thing;`, nil},
		{"constant defaults", `<?php use const Vendor\VALUE; function f($x = VALUE) { return [VALUE]; }`, nil},
		{"constant conditions", `<?php use const Vendor\VALUE; if (VALUE) {} while (VALUE) {} switch (VALUE) {} $x = match(VALUE) { default => 1 }; for (;VALUE;) {}`, nil},
		{"constant label not use", `<?php use const Vendor\label; f(label: 1);`, []string{"label"}},
		{"plain string", `<?php use Vendor\Thing; echo 'Thing';`, []string{"Thing"}},
		{"plain comment", `<?php use Vendor\Thing; // Thing
`, []string{"Thing"}},
		{"separate namespaces", `<?php namespace A; use Vendor\Thing; new Thing; namespace B; use Vendor\Thing;`, []string{"Thing"}},
		{"braced namespaces", `<?php namespace A { use Vendor\Thing; } namespace B { use Vendor\Thing; new Thing; }`, []string{"Thing"}},
		{"closure captures and trait uses", `<?php class T { use MyTrait; } $x=1; $c=function() use ($x) {};`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) { assert.Equal(t, tc.unused, unused(analyzeTest(t, tc.source))) })
	}
}

func TestOrganizeImports(t *testing.T) {
	for _, tc := range []struct{ name, source, want string }{
		{"sort remove and kinds", "<?php\nuse Z\\Zed;\nuse A\\Unused;\nuse const A\\FLAG;\nuse A\\Alpha as A;\nuse function A\\run;\nnew Zed; new A; run(); echo FLAG;\n", "<?php\nuse A\\Alpha as A;\nuse Z\\Zed;\nuse function A\\run;\nuse const A\\FLAG;\nnew Zed; new A; run(); echo FLAG;\n"},
		{"expand mixed group", `<?php use Vendor\{Other, Thing as T, function run, const FLAG}; new T; run(); echo FLAG;`, "<?php use Vendor\\Thing as T;\nuse function Vendor\\run;\nuse const Vendor\\FLAG; new T; run(); echo FLAG;"},
		{"remove line CRLF", "<?php\r\nuse Vendor\\Unused;\r\nclass Live {}\r\n", "<?php\r\nclass Live {}\r\n"},
		{"comment barrier", "<?php\nuse Z\\Zed;\n// keep explanation\nuse A\\Alpha;\nnew Zed; new Alpha;", "<?php\nuse Z\\Zed;\n// keep explanation\nuse A\\Alpha;\nnew Zed; new Alpha;"},
		{"internal line comment", "<?php use Vendor\\{Unused, // keep\n Thing}; new Thing;", "<?php use Vendor\\{// keep\nThing}; new Thing;"},
		{"internal comments", `<?php use Vendor\{Unused /* keep */, Thing}; new Thing;`, `<?php use Vendor\{/* keep */Thing}; new Thing;`},
		{"all unused preserve comments", `<?php use /* keep */ Vendor\Unused; echo 1;`, `<?php /* keep */ echo 1;`},
		{"scoped ordering", "<?php namespace A {\n use Z\\Zed;\n use A\\Alpha;\n new Zed; new Alpha;\n}\nnamespace B {\n use Z\\Zed;\n}\n", "<?php namespace A {\n use A\\Alpha;\n use Z\\Zed;\n new Zed; new Alpha;\n}\nnamespace B {\n}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			analysis := analyzeTest(t, tc.source)
			edits, err := Organize(context.Background(), tc.source, analysis)
			require.NoError(t, err)
			result, err := rewrite.Apply(tc.source, edits)
			require.NoError(t, err)
			assert.Equal(t, tc.want, result)
			require.Empty(t, parser.Parse(result).Errors)
			again, err := Organize(context.Background(), result, analyzeTest(t, result))
			require.NoError(t, err)
			assert.Empty(t, again)
		})
	}
}

func TestRemoveGroupedImport(t *testing.T) {
	source := `<?php use Vendor\{A, B as Alias, C};`
	for _, tc := range []struct{ alias, want string }{{"A", `<?php use Vendor\{B as Alias, C};`}, {"Alias", `<?php use Vendor\{A, C};`}, {"C", `<?php use Vendor\{A, B as Alias};`}} {
		t.Run(tc.alias, func(t *testing.T) {
			a := analyzeTest(t, source)
			edits, err := Remove(source, a.Scopes[0].Declarations[0], func(i Item) bool { return i.Alias == tc.alias })
			require.NoError(t, err)
			result, err := rewrite.Apply(source, edits)
			require.NoError(t, err)
			assert.Equal(t, tc.want, result)
		})
	}
}

func BenchmarkImports(b *testing.B) {
	var source strings.Builder
	source.WriteString("<?php\n")
	for i := 0; i < 200; i++ {
		source.WriteString("use Vendor\\Unused")
		source.WriteString(strings.Repeat("X", i))
		source.WriteString(";\n")
	}
	tree := parser.Parse(source.String())
	document := binder.New().Bind("/bench.php", 1, tree.Tree.Root)
	b.ReportAllocs()
	for b.Loop() {
		if _, err := Analyze(context.Background(), tree.Tree.Root, document); err != nil {
			b.Fatal(err)
		}
	}
}

func TestConstantImportExpressionContexts(t *testing.T) {
	for _, expression := range []string{
		"if (VALUE) {}", "while (VALUE) {}", "switch (VALUE) {}",
		"$x = match(VALUE) { default => 1 };", "for (;VALUE;) {}",
		"foreach (VALUE as $v) {}", "do {} while (VALUE);",
		"$x = VALUE;", "f(VALUE);", "class C { const X = VALUE; }",
		"enum C: int { case X = VALUE; }", "class C { public $x = VALUE; }",
		"$x = VALUE ? 1 : 0;", "$x = [VALUE => 1];", "yield VALUE;", "$f = fn() => VALUE;", "throw VALUE;", "$x->{VALUE};", "C::{VALUE}();",
	} {
		t.Run(expression, func(t *testing.T) {
			assert.Empty(t, unused(analyzeTest(t, "<?php use const Vendor\\VALUE; "+expression)))
		})
	}
}
