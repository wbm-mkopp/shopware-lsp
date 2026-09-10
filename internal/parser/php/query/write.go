package query

import (
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
	"strings"
)

// VariableIsWrite identifies syntactic local-variable write targets, including
// array/destructuring assignments, increments, and foreach targets. It does
// not infer writes performed by a function through a reference parameter.
func VariableIsWrite(node *syntax.Node) bool {
	if node == nil || node.Kind() != syntax.PhpVariable {
		return false
	}
	for current := node; current.Parent() != nil; current = current.Parent() {
		parent := current.Parent()
		switch parent.Kind() {
		case syntax.PhpAssignmentExpression:
			for child := range parent.ChildNodes() {
				return child == current
			}
			return false
		case syntax.PhpUnaryExpression:
			for token := range parent.ChildTokens() {
				if token.Text() == "++" || token.Text() == "--" {
					return true
				}
			}
			return false
		case syntax.PhpForeachStatement:
			afterAs := false
			for i := 0; i < parent.ChildCount(); i++ {
				element := parent.Child(i)
				if token, ok := element.(*syntax.Token); ok && strings.EqualFold(token.Text(), "as") {
					afterAs = true
				}
				if element == current {
					return afterAs
				}
			}
			return false
		case syntax.PhpArrayAccess:
			// Writing $array[$index] changes the array, but only reads the index.
			for child := range parent.ChildNodes() {
				if child != current {
					return false
				}
				break
			}
		case syntax.PhpArrayItem:
			if ArrayItemValue(parent) != current {
				return false
			}
		case syntax.PhpArray, syntax.PhpParenthesized:
		default:
			return false
		}
	}
	return false
}
