package query

import "github.com/shopware/shopware-lsp/internal/parser/php/syntax"

// IsConstantReference distinguishes expression names from declaration names,
// member names, named-argument labels, and class/function references.
func IsConstantReference(node *syntax.Node) bool {
	if node == nil || node.Kind() != syntax.PhpName || node.Parent() == nil {
		return false
	}
	parent := node.Parent()
	switch parent.Kind() {
	case syntax.PhpNamedArgument:
		return DirectChild(parent, syntax.PhpName) != node
	case syntax.PhpConstDeclaration, syntax.PhpClassConstDeclaration:
		for _, name := range ConstantNames(parent) {
			if name == node {
				return false
			}
		}
		return true
	case syntax.PhpEnumCaseDeclaration:
		return DirectChild(parent, syntax.PhpName) != node
	case syntax.PhpMemberAccess, syntax.PhpMemberCall, syntax.PhpScopedAccess, syntax.PhpScopedCall:
		// A braced member expression can contain a constant; a literal member
		// name does not consume a namespace import.
		var previous string
		for token := range parent.ChildTokens() {
			if token.Range().Start >= node.RangeTrimmedTrivia().Start {
				break
			}
			if !token.Kind().IsTrivia() {
				previous = token.Text()
			}
		}
		return previous == "{"
	case syntax.PhpForStatement, syntax.PhpForeachStatement, syntax.PhpArrowFunction,
		syntax.PhpThrowStatement, syntax.PhpBreakStatement, syntax.PhpContinueStatement,
		syntax.PhpStaticStatement, syntax.PhpArgument, syntax.PhpEchoStatement, syntax.PhpReturnStatement,
		syntax.PhpExpressionStatement, syntax.PhpArrayItem, syntax.PhpArrayAccess,
		syntax.PhpParenthesized, syntax.PhpAssignmentExpression, syntax.PhpBinaryExpression,
		syntax.PhpUnaryExpression, syntax.PhpTernaryExpression, syntax.PhpMatchArm,
		syntax.PhpCaseClause, syntax.PhpThrowExpression, syntax.PhpYieldExpression,
		syntax.PhpCloneExpression, syntax.PhpCastExpression, syntax.PhpParameter,
		syntax.PhpPropertyDeclaration:
		return true
	default:
		return false
	}
}
