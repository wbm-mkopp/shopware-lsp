package imports

import (
	"context"
	"sort"
	"strings"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	phpsyntax "github.com/shopware/shopware-lsp/internal/parser/php/syntax"
	"github.com/shopware/shopware-lsp/internal/php/resolver"
	"github.com/shopware/shopware-lsp/internal/rewrite"
)

// Remove removes selected clauses while preserving every source comment.
func Remove(source string, declaration Declaration, selected func(Item) bool) ([]rewrite.Edit, error) {
	builder := rewrite.NewBuilder(source)
	kept := []int{}
	for i, item := range declaration.Items {
		if !selected(item) {
			kept = append(kept, i)
		}
	}
	if len(kept) == len(declaration.Items) {
		return nil, nil
	}
	var ranges []cst.TextRange
	if len(kept) == 0 {
		ranges = append(ranges, declaration.Node.RangeTrimmedTrivia())
	} else {
		for i := 0; i < len(declaration.Items); {
			if !selected(declaration.Items[i]) {
				i++
				continue
			}
			end := i + 1
			for end < len(declaration.Items) && selected(declaration.Items[end]) {
				end++
			}
			rng := cst.TextRange{Start: declaration.Items[i].Range.Start, End: declaration.Items[end-1].Range.End}
			if end < len(declaration.Items) {
				rng.End = declaration.Items[end].Range.Start
			} else if i > 0 {
				rng.Start = declaration.Commas[i-1].Start
			}
			ranges = append(ranges, rng)
			i = end
		}
	}
	comments := commentRanges(declaration)
	// Keep the line ending with line comments so retained PHP is never
	// swallowed by a comment when adjacent clauses are removed.
	for i := range comments {
		text := source[comments[i].Start:comments[i].End]
		if strings.HasPrefix(text, "//") || strings.HasPrefix(text, "#") {
			if int(comments[i].End) < len(source) && source[comments[i].End] == '\r' {
				comments[i].End++
			}
			if int(comments[i].End) < len(source) && source[comments[i].End] == '\n' {
				comments[i].End++
			}
		}
	}
	for _, rng := range ranges {
		if len(kept) == 0 && len(comments) == 0 {
			rng = wholeLine(source, rng)
		}
		start := rng.Start
		for _, comment := range comments {
			if comment.End <= start || comment.Start >= rng.End {
				continue
			}
			if err := builder.ReplaceRange(cst.TextRange{Start: start, End: comment.Start}, ""); err != nil {
				return nil, err
			}
			start = comment.End
		}
		if err := builder.ReplaceRange(cst.TextRange{Start: start, End: rng.End}, ""); err != nil {
			return nil, err
		}
	}
	return builder.Finish()
}

// Organize removes unused imports and orders independent, comment-free runs.
// Comments form ordering boundaries. Grouped imports without comments expand
// to one import per line, with classes, functions, and constants ordered apart.
func Organize(ctx context.Context, source string, analysis Analysis) ([]rewrite.Edit, error) {
	builder := rewrite.NewBuilder(source)
	for _, scope := range analysis.Scopes {
		if scope.Unsafe {
			continue
		}
		for i := 0; i < len(scope.Declarations); {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			first := scope.Declarations[i]
			if len(commentRanges(first)) > 0 {
				edits, err := Remove(source, first, func(item Item) bool { return !item.Used })
				if err != nil {
					return nil, err
				}
				for _, edit := range edits {
					if err := builder.ReplaceRange(edit.Range, edit.NewText); err != nil {
						return nil, err
					}
				}
				i++
				continue
			}
			end := i + 1
			for end < len(scope.Declarations) {
				next := scope.Declarations[end]
				gap := source[scope.Declarations[end-1].Node.RangeTrimmedTrivia().End:next.Node.RangeTrimmedTrivia().Start]
				if strings.TrimSpace(gap) != "" || len(commentRanges(next)) > 0 {
					break
				}
				end++
			}
			rng := cst.TextRange{Start: first.Node.RangeTrimmedTrivia().Start, End: scope.Declarations[end-1].Node.RangeTrimmedTrivia().End}
			replacement := organizedText(source, rng, scope.Declarations[i:end])
			if replacement == "" {
				rng = wholeLine(source, rng)
			}
			if source[rng.Start:rng.End] != replacement {
				if err := builder.ReplaceRange(rng, replacement); err != nil {
					return nil, err
				}
			}
			i = end
		}
	}
	return builder.Finish()
}

func importText(item resolver.Import) string {
	text := "use "
	switch item.Kind {
	case resolver.FunctionImport:
		text += "function "
	case resolver.ConstantImport:
		text += "const "
	}
	text += item.Target
	short := item.Target
	if i := strings.LastIndexByte(short, '\\'); i >= 0 {
		short = short[i+1:]
	}
	if short != item.Alias {
		text += " as " + item.Alias
	}
	return text + ";"
}
func commentRanges(declaration Declaration) []cst.TextRange {
	var result []cst.TextRange
	rng := declaration.Node.RangeTrimmedTrivia()
	for token := range declaration.Node.ChildTokens() {
		if token.Range().Start < rng.Start {
			continue
		}
		if token.Kind() == phpsyntax.TkLineComment || token.Kind() == phpsyntax.TkBlockComment {
			result = append(result, token.Range())
		}
	}
	return result
}
func wholeLine(source string, rng cst.TextRange) cst.TextRange {
	start := strings.LastIndexByte(source[:rng.Start], '\n') + 1
	end := int(rng.End)
	for end < len(source) && source[end] != '\n' {
		end++
	}
	if strings.TrimSpace(source[start:rng.Start]) != "" || strings.TrimSpace(source[rng.End:end]) != "" {
		return rng
	}
	if end < len(source) {
		end++
	}
	return cst.TextRange{Start: uint32(start), End: uint32(end)}
}

func organizedText(source string, rng cst.TextRange, declarations []Declaration) string {
	var items []Item
	for _, declaration := range declarations {
		for _, item := range declaration.Items {
			if item.Used {
				items = append(items, item)
			}
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Kind != items[j].Kind {
			return items[i].Kind < items[j].Kind
		}
		left, right := strings.ToLower(items[i].Target), strings.ToLower(items[j].Target)
		if left != right {
			return left < right
		}
		return items[i].Alias < items[j].Alias
	})
	newline := "\n"
	if strings.Contains(source, "\r\n") {
		newline = "\r\n"
	}
	lineStart := strings.LastIndexByte(source[:rng.Start], '\n') + 1
	indent := source[lineStart:rng.Start]
	if strings.TrimSpace(indent) != "" {
		indent = ""
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, importText(item.Import))
	}
	return strings.Join(lines, newline+indent)
}
