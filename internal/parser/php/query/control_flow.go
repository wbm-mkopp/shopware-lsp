package query

import "github.com/shopware/shopware-lsp/internal/parser/php/syntax"

type ConditionalBranch struct{ Condition, Body *syntax.Node }

func ConditionalBranches(node *syntax.Node) []ConditionalBranch {
	var result []ConditionalBranch
	var branch ConditionalBranch
	for child := range node.ChildNodes() {
		switch child.Kind() {
		case syntax.PhpElseIfClause, syntax.PhpElseClause:
			result = append(result, ConditionalBranches(child)...)
		default:
			if branch.Condition == nil && child.Kind() == syntax.PhpParenthesized {
				branch.Condition = child
			} else if branch.Body == nil {
				branch.Body = child
				result = append(result, branch)
			}
		}
	}
	return result
}

type LoopParts struct {
	Init, Condition, Update []*syntax.Node
	Targets                 []*syntax.Node
	Body                    *syntax.Node
}

func ExtractionLoopParts(node *syntax.Node) LoopParts {
	var parts LoopParts
	phase := 0
	inHeader := node.Kind() == syntax.PhpDoWhileStatement
	for i := 0; i < node.ChildCount(); i++ {
		switch child := node.Child(i).(type) {
		case *syntax.Token:
			if child.Kind() == syntax.TkOpenParen {
				inHeader = true
			}
			if child.Kind() == syntax.TkCloseParen {
				inHeader = false
			}
			if child.Kind() == syntax.TkSemicolon && inHeader {
				phase++
			}
			if child.Text() == "as" {
				phase = 1
			}
		case *syntax.Node:
			if node.Kind() == syntax.PhpForStatement && inHeader {
				switch phase {
				case 0:
					parts.Init = append(parts.Init, child)
				case 1:
					parts.Condition = append(parts.Condition, child)
				default:
					parts.Update = append(parts.Update, child)
				}
			} else if node.Kind() == syntax.PhpForeachStatement && inHeader {
				if phase == 0 {
					parts.Condition = append(parts.Condition, child)
				} else {
					parts.Targets = append(parts.Targets, child)
				}
			} else if child.Kind() == syntax.PhpParenthesized {
				parts.Condition = append(parts.Condition, child)
			} else {
				parts.Body = child
			}
		}
	}
	return parts
}

func ExpressionOperator(node *syntax.Node) string         { return extractionOperator(node) }
func ExpressionChildren(node *syntax.Node) []*syntax.Node { return extractionChildren(node) }
