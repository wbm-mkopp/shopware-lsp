package refactoring

import (
	"fmt"
	"strings"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/parser/php/query"
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
	"github.com/shopware/shopware-lsp/internal/rewrite"
)

// A tagged result transports returns across the new callable boundary without
// confusing an ordinary fallthrough with "return null" or a void return.
func controlReturnCall(source, call string, flow methodFlow, newline, indent string) string {
	name := "$extractedResult"
	for i := 2; strings.Contains(source, name[1:]); i++ {
		name = fmt.Sprintf("$extractedResult%d", i)
	}
	lines := []string{name + " = " + call}
	if flow.valueReturn {
		lines = append(lines, "if ("+name+"[0] === 1) { return "+name+"[1]; }")
	}
	if flow.voidReturn {
		lines = append(lines, "if ("+name+"[0] === 2) { return; }")
	}
	if len(flow.outputs) > 0 {
		lines = append(lines, flowOutput(flow.outputs)+" = "+name+"[1];")
	}
	lines = append(lines, name+" = null;")
	return strings.Join(lines, newline+indent)
}

func reindentControlReturns(source string, rng cst.TextRange, from, to string, statements, returns []*cst.Node) (string, error) {
	type markers struct {
		start, end uint32
		bare       bool
	}
	var markersByReturn []markers
	var positions []uint32
	for _, ret := range returns {
		marker := markers{bare: len(query.ExpressionChildren(ret)) == 0}
		for token := range ret.ChildTokens() {
			if token.Text() == "return" {
				marker.start = token.Range().End
			}
			if token.Kind() == syntax.TkSemicolon {
				marker.end = token.Range().Start
			}
		}
		if marker.start == 0 || marker.end == 0 {
			return "", fmt.Errorf("return statement has no stable boundaries")
		}
		positions = append(positions, marker.start, marker.end)
		markersByReturn = append(markersByReturn, marker)
	}
	body, offsets := reindentExtractionOffsets(source, rng, from, to, statements, positions)
	builder := rewrite.NewBuilder(body)
	for _, marker := range markersByReturn {
		prefix, suffix := " [1,", "]"
		if marker.bare {
			prefix, suffix = " [2,", "null]"
		}
		if err := builder.Insert(offsets[marker.start], prefix); err != nil {
			return "", err
		}
		if err := builder.Insert(offsets[marker.end], suffix); err != nil {
			return "", err
		}
	}
	edits, err := builder.Finish()
	if err != nil {
		return "", err
	}
	return rewrite.Apply(body, edits)
}
