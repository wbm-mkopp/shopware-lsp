package query

import (
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
	"strings"
)

// ExtractableExpression excludes write targets, bare names and variables.
func ExtractableExpression(node *syntax.Node) bool {
	switch node.Kind() {
	case syntax.PhpFunctionCall, syntax.PhpMemberCall, syntax.PhpScopedCall,
		syntax.PhpObjectCreation, syntax.PhpBinaryExpression, syntax.PhpUnaryExpression,
		syntax.PhpParenthesized, syntax.PhpTernaryExpression, syntax.PhpMatchExpression, syntax.PhpArray, syntax.PhpArrayAccess,
		syntax.PhpMemberAccess, syntax.PhpScopedAccess, syntax.PhpString, syntax.PhpNumber:
		return true
	}
	return false
}

// ExtractionStatement accepts only a path evaluated first and unconditionally.
// It deliberately excludes argument passing, reference contexts, loop headers,
// conditional branches and unbraced bodies.
func ExtractionStatement(expression *syntax.Node) *syntax.Node {
	for node := expression; node.Parent() != nil; node = node.Parent() {
		parent := node.Parent()
		children := extractionChildren(parent)
		switch parent.Kind() {
		case syntax.PhpParenthesized:
		case syntax.PhpBinaryExpression:
			if len(children) != 2 || children[0] != node || extractionOperator(parent) == "??" {
				return nil
			}
		case syntax.PhpUnaryExpression:
			switch extractionOperator(parent) {
			case "!", "~", "+", "-":
			default:
				return nil
			}
		case syntax.PhpAssignmentExpression:
			if len(children) != 2 || children[1] != node || children[0].Kind() != syntax.PhpVariable || extractionOperator(parent) != "=" {
				return nil
			}
		case syntax.PhpReturnStatement, syntax.PhpEchoStatement, syntax.PhpExpressionStatement:
			if len(children) == 0 || children[0] != node || parent.Parent() == nil {
				return nil
			}
			switch parent.Parent().Kind() {
			case syntax.PhpBlock, syntax.PhpProgram:
				return parent
			}
			return nil
		default:
			return nil
		}
	}
	return nil
}

func extractionChildren(node *syntax.Node) []*syntax.Node {
	var result []*syntax.Node
	for child := range node.ChildNodes() {
		result = append(result, child)
	}
	return result
}

func extractionOperator(node *syntax.Node) string {
	var result string
	for token := range node.ChildTokens() {
		if !token.Kind().IsTrivia() {
			result += token.Text()
		}
	}
	return result
}

// InlineExtractionOwner permits an assignment expression at the original
// evaluation point. This preserves short-circuiting, loop frequency and the
// order of other arguments without lifting work out of its control flow.
func InlineExtractionOwner(expression *syntax.Node, valueCall func(*syntax.Node) bool) *syntax.Node {
	for node := expression; node.Parent() != nil; node = node.Parent() {
		parent := node.Parent()
		children := extractionChildren(parent)
		switch parent.Kind() {
		case syntax.PhpParenthesized, syntax.PhpTernaryExpression, syntax.PhpMatchArm,
			syntax.PhpMatchExpression, syntax.PhpArrayAccess, syntax.PhpArray,
			syntax.PhpMemberAccess, syntax.PhpScopedAccess, syntax.PhpMemberCall,
			syntax.PhpScopedCall, syntax.PhpFunctionCall, syntax.PhpObjectCreation,
			syntax.PhpArgumentList, syntax.PhpCastExpression, syntax.PhpCloneExpression:
		case syntax.PhpArrayItem:
			if hasExtractionToken(parent, "&") {
				return nil
			}
		case syntax.PhpArgument, syntax.PhpNamedArgument:
			if !valueOnlyExpression(node, valueCall) {
				return nil
			}
		case syntax.PhpBinaryExpression:
			if extractionOperator(parent) == "??" && len(children) > 0 && children[0] == node {
				return nil
			}
		case syntax.PhpUnaryExpression:
			switch extractionOperator(parent) {
			case "!", "~", "+", "-", "@":
			default:
				return nil
			}
		case syntax.PhpAssignmentExpression:
			if len(children) != 2 || children[1] != node || hasExtractionToken(parent, "&") {
				return nil
			}
		case syntax.PhpForeachStatement:
			if len(children) == 0 || children[0] != node {
				return nil
			}
			return parent
		case syntax.PhpExpressionStatement, syntax.PhpReturnStatement, syntax.PhpEchoStatement,
			syntax.PhpThrowStatement, syntax.PhpIfStatement, syntax.PhpElseIfClause,
			syntax.PhpWhileStatement, syntax.PhpDoWhileStatement, syntax.PhpForStatement,
			syntax.PhpSwitchStatement, syntax.PhpCaseClause, syntax.PhpArrowFunction:
			return parent
		default:
			return nil
		}
	}
	return nil
}

func valueOnlyExpression(node *syntax.Node, valueCall func(*syntax.Node) bool) bool {
	switch node.Kind() {
	case syntax.PhpParenthesized:
		children := extractionChildren(node)
		return len(children) == 1 && valueOnlyExpression(children[0], valueCall)
	case syntax.PhpFunctionCall, syntax.PhpMemberCall, syntax.PhpScopedCall:
		return valueCall != nil && valueCall(node)
	case syntax.PhpBinaryExpression, syntax.PhpUnaryExpression, syntax.PhpTernaryExpression,
		syntax.PhpMatchExpression, syntax.PhpArray, syntax.PhpString, syntax.PhpNumber,
		syntax.PhpObjectCreation, syntax.PhpCastExpression, syntax.PhpCloneExpression:
		return true
	}
	return false
}

func hasExtractionToken(node *syntax.Node, text string) bool {
	for token := range node.ChildTokens() {
		if token.Text() == text {
			return true
		}
	}
	return false
}

// PureExtractionExpression limits occurrence sharing to expressions without
// calls, object/property access, interpolation or writes.
func PureExtractionExpression(node *syntax.Node) bool {
	switch node.Kind() {
	case syntax.PhpVariable, syntax.PhpNumber, syntax.PhpBoolean, syntax.PhpNull:
		return true
	case syntax.PhpString:
		for token := range node.ChildTokens() {
			if strings.ContainsAny(token.Text(), "$\r\n") || strings.HasPrefix(token.Text(), "<<<") {
				return false
			}
		}
		return true
	case syntax.PhpParenthesized, syntax.PhpBinaryExpression, syntax.PhpUnaryExpression:
		if hasExtractionToken(node, "++") || hasExtractionToken(node, "--") || hasExtractionToken(node, "@") {
			return false
		}
		for child := range node.ChildNodes() {
			if !PureExtractionExpression(child) {
				return false
			}
		}
		return true
	}
	return false
}

// EagerArgumentList finds an argument list in which this expression is always
// evaluated when its argument is evaluated.
func EagerArgumentList(expression *syntax.Node) *syntax.Node {
	for node := expression; node.Parent() != nil; node = node.Parent() {
		parent := node.Parent()
		switch parent.Kind() {
		case syntax.PhpArgument, syntax.PhpNamedArgument:
			return parent.Parent()
		case syntax.PhpParenthesized:
		case syntax.PhpBinaryExpression:
			children := extractionChildren(parent)
			switch extractionOperator(parent) {
			case "&&", "||", "and", "or", "??":
				if len(children) == 0 || children[0] != node {
					return nil
				}
			}
		case syntax.PhpUnaryExpression:
		default:
			return nil
		}
	}
	return nil
}
