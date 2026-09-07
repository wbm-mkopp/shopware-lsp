package folding

import (
	"context"
	"sort"
	"strings"

	"github.com/shopware/shopware-lsp/internal/language"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/parser/cst"
	phpsyntax "github.com/shopware/shopware-lsp/internal/parser/php/syntax"
)

// PHPFoldingProvider reads only the current CST and emits portable line folds.
type PHPFoldingProvider struct{}

func NewPHPFoldingProvider() *PHPFoldingProvider { return &PHPFoldingProvider{} }

func (p *PHPFoldingProvider) GetFoldingRanges(ctx context.Context, request *lsp.FoldingRangeRequest) ([]protocol.FoldingRange, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request == nil || request.Document == nil || request.Document.SyntaxLanguage != language.PHP || request.Document.SyntaxTree == nil || request.Document.SyntaxTree.Root == nil || request.Document.LineIndex == nil {
		return nil, nil
	}
	collector := phpFoldCollector{ctx: ctx, lines: request.Document.LineIndex, ranges: make(map[protocol.FoldingRange]struct{})}
	collector.visit(request.Document.SyntaxTree.Root)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	result := make([]protocol.FoldingRange, 0, len(collector.ranges))
	for rng := range collector.ranges {
		result = append(result, rng)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].StartLine != result[j].StartLine {
			return result[i].StartLine < result[j].StartLine
		}
		if result[i].EndLine != result[j].EndLine {
			return result[i].EndLine < result[j].EndLine
		}
		return result[i].Kind < result[j].Kind
	})
	return result, nil
}

type phpFoldCollector struct {
	ctx    context.Context
	lines  *cst.LineIndex
	ranges map[protocol.FoldingRange]struct{}
}

func (c *phpFoldCollector) add(rng cst.TextRange, hideLast bool, kind string) {
	if rng.Len() == 0 {
		return
	}
	start, _ := c.lines.PositionUTF16(rng.Start)
	end, _ := c.lines.PositionUTF16(rng.End - 1)
	if hideLast && end > start {
		end--
	}
	if end > start {
		c.ranges[protocol.FoldingRange{StartLine: int(start), EndLine: int(end), Kind: kind}] = struct{}{}
	}
}

func (c *phpFoldCollector) visit(node *phpsyntax.Node) {
	if c.ctx.Err() != nil {
		return
	}
	switch node.Kind() {
	case phpsyntax.PhpClassBody, phpsyntax.PhpBlock, phpsyntax.PhpArray, phpsyntax.PhpMatchExpression, phpsyntax.PhpPropertyHookList, phpsyntax.PhpTraitUseDeclaration:
		rng := node.RangeTrimmedTrivia()
		last := node.TokenAtOffset(rng.End - 1)
		closed := last != nil && (last.Kind() == phpsyntax.TkCloseBrace || last.Kind() == phpsyntax.TkCloseBracket || last.Kind() == phpsyntax.TkCloseParen)
		c.add(rng, closed, "")
	case phpsyntax.PhpIfStatement, phpsyntax.PhpForStatement, phpsyntax.PhpForeachStatement, phpsyntax.PhpWhileStatement, phpsyntax.PhpSwitchStatement:
		if node.Kind() == phpsyntax.PhpSwitchStatement && node.ChildTokenOfKind(phpsyntax.TkOpenBrace) != nil {
			c.add(node.RangeTrimmedTrivia(), node.ChildTokenOfKind(phpsyntax.TkCloseBrace) != nil, "")
		}
		// Alternative control syntax keeps its end keyword directly on the statement.
		for token := range node.ChildTokens() {
			switch strings.ToLower(token.Text()) {
			case "endif", "endfor", "endforeach", "endwhile", "endswitch":
				c.add(node.RangeTrimmedTrivia(), true, "")
			}
		}
	}
	var imports, comments cst.TextRange
	flushImports := func() { c.add(imports, false, protocol.FoldingRangeKindImports); imports = cst.TextRange{} }
	flushComments := func() { c.add(comments, false, protocol.FoldingRangeKindComment); comments = cst.TextRange{} }
	for i := 0; i < node.ChildCount(); i++ {
		if c.ctx.Err() != nil {
			return
		}
		element := node.Child(i)
		if child, ok := element.(*phpsyntax.Node); ok {
			flushComments()
			if child.Kind() == phpsyntax.PhpUseDeclaration {
				if imports.Len() == 0 {
					imports.Start = child.RangeTrimmedTrivia().Start
				}
				imports.End = child.RangeTrimmedTrivia().End
			} else {
				flushImports()
			}
			c.visit(child)
			continue
		}
		token, ok := element.(*phpsyntax.Token)
		if !ok {
			continue
		}
		switch token.Kind() {
		case phpsyntax.TkLineComment:
			start, _ := c.lines.PositionUTF16(token.Range().Start)
			previous, _ := c.lines.PositionUTF16(comments.End)
			if comments.Len() > 0 && start > previous+1 {
				flushComments()
			}
			if comments.Len() == 0 {
				comments.Start = token.Range().Start
			}
			comments.End = token.Range().End
		case phpsyntax.TkBlockComment:
			flushComments()
			c.add(token.Range(), false, protocol.FoldingRangeKindComment)
		case phpsyntax.TkString:
			flushComments()
			flushImports()
			c.add(token.Range(), false, "")
		default:
			if !token.Kind().IsTrivia() {
				flushComments()
				flushImports()
			}
		}
	}
	flushImports()
	flushComments()
}

var _ lsp.FoldingRangeProvider = (*PHPFoldingProvider)(nil)
