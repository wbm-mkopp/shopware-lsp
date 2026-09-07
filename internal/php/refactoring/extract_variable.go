// Package refactoring builds PHP refactorings against immutable source snapshots.
package refactoring

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/parser/php/query"
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
	"github.com/shopware/shopware-lsp/internal/rewrite"
)

type VariableOptions struct {
	CallReturnsValue func(*cst.Node) bool
	AllOccurrences   bool
}

type Extraction struct {
	Occurrences int
	Name        string
	Edits       []rewrite.Edit
}

// ExtractVariable replaces a selected expression (or the smallest supported
// expression at a caret) with a fresh local variable. No workspace scan is used.
func ExtractVariable(ctx context.Context, root *cst.Node, source string, selection cst.TextRange, options ...VariableOptions) (*Extraction, error) {
	if root == nil || selection.End > uint32(len(source)) || selection.Start > selection.End {
		return nil, nil
	}
	// Explicit selections may include whitespace, but must cover one whole expression.
	if selection.Start != selection.End {
		for selection.Start < selection.End && strings.ContainsRune(" \t\r\n", rune(source[selection.Start])) {
			selection.Start++
		}
		for selection.End > selection.Start && strings.ContainsRune(" \t\r\n", rune(source[selection.End-1])) {
			selection.End--
		}
		if selection.Start == selection.End {
			return nil, nil
		}
	}
	var chosen, statement *cst.Node
	inline := false
	option := VariableOptions{}
	if len(options) > 0 {
		option = options[0]
	}
	names := map[string]bool{}
	for element := range root.Descendants() {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if token, ok := element.(*cst.Token); ok {
			if token.Kind() == syntax.TkVariable {
				names[token.Text()] = true
			}
			// Dynamic symbol tables and line-number literals make introducing a
			// local or moving source unsafe without whole-program information.
			switch strings.ToLower(token.Text()) {
			case "$", "eval", "extract", "get_defined_vars", "__line__":
				return nil, nil
			}
		}
		node, ok := element.(*cst.Node)
		if ok && node.Kind() == syntax.Error {
			return nil, nil
		}
		if !ok || !query.ExtractableExpression(node) {
			continue
		}
		rng := node.RangeTrimmedTrivia()
		matches := rng == selection
		if selection.Start == selection.End {
			matches = rng.Start <= selection.Start && selection.Start < rng.End
		}
		if !matches || chosen != nil && rng.End-rng.Start >= chosen.RangeTrimmedTrivia().End-chosen.RangeTrimmedTrivia().Start {
			continue
		}
		owner := query.ExtractionStatement(node)
		inPlace := false
		if owner == nil {
			owner = query.InlineExtractionOwner(node, option.CallReturnsValue)
			inPlace = true
		}
		if owner != nil && safeExtraction(node, owner) {
			chosen, statement, inline = node, owner, inPlace
		}
	}
	if chosen == nil {
		return nil, nil
	}
	name := "$extracted"
	for suffix := 2; names[name] || strings.Contains(source, name[1:]); suffix++ {
		name = fmt.Sprintf("$extracted%d", suffix)
	}
	if option.AllOccurrences {
		if inline {
			return extractArgumentOccurrences(ctx, source, chosen, name)
		}
		return extractAllOccurrences(ctx, root, source, chosen, statement, name)
	}
	return buildVariableExtraction(source, chosen, statement, name, inline)
}

func buildVariableExtraction(source string, chosen, statement *cst.Node, name string, inline bool) (*Extraction, error) {
	rng := chosen.RangeTrimmedTrivia()
	builder := rewrite.NewBuilder(source)
	if inline {
		if err := builder.ReplaceRange(rng, "("+name+" = "+source[rng.Start:rng.End]+")"); err != nil {
			return nil, err
		}
	} else {
		start := statement.RangeTrimmedTrivia().Start
		indent := sourceIndent(source, start)
		newline := "\n"
		if strings.Contains(source, "\r\n") {
			newline = "\r\n"
		}
		if err := builder.Insert(start, name+" = "+source[rng.Start:rng.End]+";"+newline+indent); err != nil {
			return nil, err
		}
		if err := builder.ReplaceRange(rng, name); err != nil {
			return nil, err
		}
	}
	edits, err := builder.Finish()
	return &Extraction{Name: name, Edits: edits, Occurrences: 1}, err
}

func safeExtraction(expression, statement *cst.Node) bool {
	for element := range expression.Descendants() {
		if node, ok := element.(*cst.Node); ok {
			switch node.Kind() {
			case syntax.PhpYieldExpression, syntax.PhpClosure, syntax.PhpArrowFunction:
				return false
			}
		}
	}
	// Returning through a new temporary would change by-reference returns.
	for parent := statement; parent != nil; parent = parent.Parent() {
		switch parent.Kind() {
		case syntax.PhpPropertyHook:
			return false
		case syntax.PhpFunctionDeclaration, syntax.PhpMethodDeclaration, syntax.PhpClosure, syntax.PhpArrowFunction:
			for token := range parent.ChildTokens() {
				if token.Kind() == syntax.TkAmpersand {
					return false
				}
			}
			return true
		}
	}
	return true
}
