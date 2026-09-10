package query

import (
	"context"
	"strings"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
)

type MethodSelection struct {
	Method, Class *syntax.Node
	Block         *syntax.Node
	Statements    []*syntax.Node
	Range         cst.TextRange
}

// SelectMethodStatements accepts complete consecutive top-level method
// statements. A caret selects the containing statement.
func SelectMethodStatements(ctx context.Context, root *syntax.Node, source string, selected cst.TextRange) *MethodSelection {
	if root == nil || selected.Start > selected.End || selected.End > uint32(len(source)) {
		return nil
	}
	caret := selected.Start == selected.End
	if !caret {
		for selected.Start < selected.End && strings.ContainsRune(" \t\r\n", rune(source[selected.Start])) {
			selected.Start++
		}
		for selected.End > selected.Start && strings.ContainsRune(" \t\r\n", rune(source[selected.End-1])) {
			selected.End--
		}
	}

	var best *MethodSelection
	for _, body := range Nodes(root, syntax.PhpBlock) {
		if ctx.Err() != nil {
			return nil
		}
		owner := FunctionLikeAt(body)
		if owner == nil || (owner.Kind() != syntax.PhpMethodDeclaration && owner.Kind() != syntax.PhpFunctionDeclaration) {
			continue
		}
		class := ClassAt(owner)
		if owner.Kind() == syntax.PhpMethodDeclaration && (class == nil || class.Kind() != syntax.PhpClassDeclaration) {
			continue
		}
		if owner.Kind() == syntax.PhpFunctionDeclaration {
			if FunctionLikeAt(owner.Parent()) != nil {
				continue
			}
			class = nil
		} else if owner.Parent() != ClassBody(class) {
			continue
		}
		result := &MethodSelection{Method: owner, Class: class, Block: body}
		for statement := range body.ChildNodes() {
			rng := statement.RangeTrimmedTrivia()
			if caret && rng.Start <= selected.Start && selected.Start < rng.End || !caret && rng.Start >= selected.Start && rng.End <= selected.End {
				result.Statements = append(result.Statements, statement)
			}
		}
		if len(result.Statements) == 0 {
			continue
		}
		result.Range = cst.TextRange{Start: result.Statements[0].RangeTrimmedTrivia().Start, End: result.Statements[len(result.Statements)-1].RangeTrimmedTrivia().End}
		if (caret || result.Range == selected) && (best == nil || result.Range.End-result.Range.Start < best.Range.End-best.Range.Start) {
			best = result
		}
	}
	return best

}

// SimpleAssignment identifies an unconditional assignment to one local.
func SimpleAssignment(statement *syntax.Node) (string, *syntax.Node) {
	if statement == nil || statement.Kind() != syntax.PhpExpressionStatement {
		return "", nil
	}
	assignment := DirectChild(statement, syntax.PhpAssignmentExpression)
	if assignment == nil || extractionOperator(assignment) != "=" {
		return "", nil
	}
	children := extractionChildren(assignment)
	if len(children) != 2 || children[0].Kind() != syntax.PhpVariable {
		return "", nil
	}
	return "$" + VariableName(children[0]), children[1]
}

func DeclarationIsStatic(node *syntax.Node) bool {
	for token := range node.ChildTokens() {
		if strings.EqualFold(token.Text(), "static") {
			return true
		}
	}
	return false
}
