package refactoring

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/parser/php/parser"
	"github.com/shopware/shopware-lsp/internal/rewrite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Optional execution checks supplement native-CST tests without making PHP a
// build prerequisite. Fixtures are generated locally and need no extensions.
func TestExtractionPreservesPHPExecution(t *testing.T) {
	php, err := exec.LookPath("php")
	if err != nil {
		t.Skip("PHP executable unavailable")
	}
	run := func(t *testing.T, source string) string {
		t.Helper()
		command := exec.Command(php, "-n", "-d", "display_errors=1", "-d", "error_reporting=-1")
		command.Stdin = strings.NewReader(source)
		output, err := command.CombinedOutput()
		require.NoError(t, err, string(output))
		return string(output)
	}
	for _, tc := range []struct {
		name, source, selected string
		variable               bool
	}{
		{"argument order", "<?php $log=[]; function step($n){global $log;$log[]=$n;return $n;} function useValues($a,$b,$c){global $log;echo json_encode($log);} useValues(step(1), step(2), step(3));", "step(2)", true},
		{"short circuit", "<?php $n=0; function step(){global $n;$n++;return true;} $x=false && step(); echo $n;", "step()", true},
		{"loop frequency", "<?php $n=0; function step(){global $n;return $n++ < 3;} while(step()) {} echo $n;", "step()", true},
		{"retained object", "<?php class Trace { function __destruct(){echo 'd';} } function run(){ $x=new Trace; echo 'a'; echo 'b'; } run();", "$x=new Trace; echo 'a';", false},
		{"reassigned input", "<?php class Trace { function __destruct(){echo 'd';} } function run($a){ $a=1; echo 'a'; echo 'b'; } run(new Trace);", "$a=1; echo 'a';", false},
		{"multiple outputs", "<?php function run($a,$b){ $x=$a+1; $y=$b+2; return [$x,$y]; } echo json_encode(run(2,3));", "$x=$a+1; $y=$b+2;", false},
		{"condition assignment", "<?php function run($a){ if($x=$a){echo $x;}else{echo $x;} return $x; } echo run(0),run(2);", "if($x=$a){echo $x;}else{echo $x;}", false},
		{"nested branch", "<?php function run($a){ if($a){$x=1;}else{$x=2;} return $x; } echo run(true), run(false);", "if($a){$x=1;}else{$x=2;}", false},
		{"foreach", "<?php function run($items){ $sum=0; foreach($items as $item){$sum += $item;} return $sum; } echo run([]), run([1,2,3]);", "foreach($items as $item){$sum += $item;}", false},
		{"for", "<?php function run($n){ $sum=0; for($i=0;$i<$n;$i++){$sum += $i;} return $sum; } echo run(0), run(4);", "for($i=0;$i<$n;$i++){$sum += $i;}", false},
		{"references", "<?php function run(&$a,&$b){ $a=7; echo $b; } $v=1; run($v,$v); echo $v;", "$a=7; echo $b;", false},
		{"literal", "<?php function run($a){ echo \"first\n  second $a\"; } run(3);", "echo \"first\n  second $a\";", false},
		{"early return value", "<?php function run($a){ if($a) return null; $x=3; return $x; } echo json_encode([run(true),run(false)]);", "if($a) return null; $x=3;", false},
		{"early return void", "<?php function run($a):void { if($a) return; echo 'a'; echo 'b'; } run(true);run(false);", "if($a) return; echo 'a';", false},
		{"early loop return", "<?php function run($a){ foreach($a as $v){if($v>1)return $v;} return 0; } echo run([]),run([0,2]);", "foreach($a as $v){if($v>1)return $v;}", false},
		{"switch", "<?php function run($a){ switch($a){case 1:$x=1;break;default:$x=2;} return $x; } echo run(1),run(2);", "switch($a){case 1:$x=1;break;default:$x=2;}", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tree := parser.Parse(tc.source)
			start := uint32(strings.LastIndex(tc.source, tc.selected))
			selection := cst.TextRange{Start: start, End: start + uint32(len(tc.selected))}
			var result *Extraction
			if tc.variable {
				result, err = ExtractVariable(context.Background(), tree.Tree.Root, tc.source, selection, VariableOptions{CallReturnsValue: func(*cst.Node) bool { return true }})
			} else {
				result, err = ExtractMethod(context.Background(), tree.Tree.Root, tc.source, selection, nil)
			}
			require.NoError(t, err)
			require.NotNil(t, result)
			changed, err := rewrite.Apply(tc.source, result.Edits)
			require.NoError(t, err)
			assert.Equal(t, run(t, tc.source), run(t, changed), changed)
		})
	}
}
