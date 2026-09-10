package refactoring

import (
	"context"
	"strings"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/parser/php/query"
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
	"github.com/shopware/shopware-lsp/internal/rewrite"
)

// Sharing is limited to consecutive pure statements in the same block.
// A mutation of an input, a call or control-flow boundary ends the region.
func extractAllOccurrences(ctx context.Context, root *cst.Node, source string, chosen, statement *cst.Node, name string) (*Extraction, error) {
	if !query.PureExtractionExpression(chosen) || statement.Parent() == nil {
		return nil, nil
	}
	dependencies := map[string]bool{}
	for element := range chosen.Descendants() {
		if token, ok := element.(*cst.Token); ok && token.Kind() == syntax.TkVariable {
			dependencies[token.Text()] = true
		}
	}
	key := extractionKey(chosen)
	var matches []cst.TextRange
	started := false
	for next := range statement.Parent().ChildNodes() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if next == statement {
			started = true
		}
		if !started {
			continue
		}
		target, expression := query.SimpleAssignment(next)
		if target != "" {
			if dependencies[target] {
				break
			}
		} else if next.Kind() == syntax.PhpEchoStatement || next.Kind() == syntax.PhpReturnStatement {
			for child := range next.ChildNodes() {
				expression = child
				break
			}
		} else {
			break
		}
		if expression == nil || !query.PureExtractionExpression(expression) {
			break
		}
		add := func(node *cst.Node) {
			rng := node.RangeTrimmedTrivia()
			if rng.Start < chosen.RangeTrimmedTrivia().Start || extractionKey(node) != key || node != chosen && extractionHasComments(node) {
				return
			}
			if len(matches) == 0 || matches[len(matches)-1].End <= rng.Start {
				matches = append(matches, rng)
			}
		}
		add(expression)
		for element := range expression.Descendants() {
			if node, ok := element.(*cst.Node); ok {
				add(node)
			}
		}
		if next.Kind() == syntax.PhpReturnStatement {
			break
		}
	}
	if len(matches) < 2 {
		return nil, nil
	}
	builder := rewrite.NewBuilder(source)
	newline := "\n"
	if strings.Contains(source, "\r\n") {
		newline = "\r\n"
	}
	start := statement.RangeTrimmedTrivia().Start
	rng := chosen.RangeTrimmedTrivia()
	if err := builder.Insert(start, name+" = "+source[rng.Start:rng.End]+";"+newline+sourceIndent(source, start)); err != nil {
		return nil, err
	}
	for _, match := range matches {
		if err := builder.ReplaceRange(match, name); err != nil {
			return nil, err
		}
	}
	edits, err := builder.Finish()
	return &Extraction{Name: name, Edits: edits, Occurrences: len(matches)}, err
}

func extractionKey(node *cst.Node) string {
	var text strings.Builder
	for element := range node.Descendants() {
		if token, ok := element.(*cst.Token); ok && !token.Kind().IsTrivia() {
			text.WriteString(token.Text())
			text.WriteByte(0)
		}
	}
	return text.String()
}

func extractionHasComments(node *cst.Node) bool {
	for element := range node.Descendants() {
		if token, ok := element.(*cst.Token); ok && (token.Kind() == syntax.TkLineComment || token.Kind() == syntax.TkBlockComment) {
			return true
		}
	}
	return false
}

func extractArgumentOccurrences(ctx context.Context, source string, chosen *cst.Node, name string) (*Extraction, error) {
	list := query.EagerArgumentList(chosen)
	if list == nil || !query.PureExtractionExpression(chosen) {
		return nil, nil
	}
	key := extractionKey(chosen)
	var matches []cst.TextRange
	index := 0
	for range list.ChildNodes() {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		expression := query.ArgumentExpression(list.Parent(), index)
		index++
		if expression == nil || !query.PureExtractionExpression(expression) {
			return nil, nil
		}
		add := func(node *cst.Node) {
			rng := node.RangeTrimmedTrivia()
			if rng.Start < chosen.RangeTrimmedTrivia().Start || extractionKey(node) != key || node != chosen && extractionHasComments(node) {
				return
			}
			if len(matches) == 0 || matches[len(matches)-1].End <= rng.Start {
				matches = append(matches, rng)
			}
		}
		add(expression)
		for element := range expression.Descendants() {
			if node, ok := element.(*cst.Node); ok {
				add(node)
			}
		}
	}
	if len(matches) < 2 || matches[0] != chosen.RangeTrimmedTrivia() {
		return nil, nil
	}
	builder := rewrite.NewBuilder(source)
	for i, rng := range matches {
		text := name
		if i == 0 {
			text = "(" + name + " = " + source[rng.Start:rng.End] + ")"
		}
		if err := builder.ReplaceRange(rng, text); err != nil {
			return nil, err
		}
	}
	edits, err := builder.Finish()
	return &Extraction{Name: name, Edits: edits, Occurrences: len(matches)}, err
}
