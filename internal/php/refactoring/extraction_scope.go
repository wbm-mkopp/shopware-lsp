package refactoring

import (
	"maps"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/parser/php/query"
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
)

func extractionAvailable(selected *query.MethodSelection) (map[string]bool, map[string]bool) {
	defined, references := map[string]bool{}, map[string]bool{}
	for _, parameter := range query.Parameters(selected.Method) {
		name := query.ParameterName(parameter)
		defined[name] = true
		for token := range parameter.ChildTokens() {
			if token.Kind() == syntax.TkAmpersand {
				references[name] = true
			}
		}
	}
	var path []*cst.Node
	for node := selected.Block; node != nil && node != selected.Method; node = node.Parent() {
		path = append(path, node)
	}
	for i := len(path) - 1; i >= 0; i-- {
		node := path[i]
		limit := selected.Range.Start
		if i > 0 {
			limit = path[i-1].RangeTrimmedTrivia().Start
		}
		if node.Kind() == syntax.PhpBlock {
			for child := range node.ChildNodes() {
				if child.RangeTrimmedTrivia().Start >= limit {
					break
				}
				addDefiniteWrites(child, defined)
			}
		} else {
			switch node.Kind() {
			case syntax.PhpIfStatement, syntax.PhpElseIfClause:
				if condition := query.DirectChild(node, syntax.PhpParenthesized); condition != nil {
					addDefiniteWrites(condition, defined)
				}
			case syntax.PhpForStatement, syntax.PhpForeachStatement, syntax.PhpWhileStatement:
				parts := query.ExtractionLoopParts(node)
				for _, init := range parts.Init {
					addDefiniteWrites(init, defined)
				}
				for _, condition := range parts.Condition {
					addDefiniteWrites(condition, defined)
				}
				for _, target := range parts.Targets {
					if target.Kind() == syntax.PhpVariable {
						defined[query.VariableKey(target)] = true
					}
				}
			}
		}
	}
	return defined, references
}
func addDefiniteWrites(node *cst.Node, defined map[string]bool) {
	switch node.Kind() {
	case syntax.PhpExpressionStatement, syntax.PhpBlock, syntax.PhpParenthesized:
		for child := range node.ChildNodes() {
			addDefiniteWrites(child, defined)
		}
	case syntax.PhpBinaryExpression:
		for child := range node.ChildNodes() {
			addDefiniteWrites(child, defined)
			break
		}
	case syntax.PhpAssignmentExpression:
		children := query.ExpressionChildren(node)
		if len(children) == 2 && children[0].Kind() == syntax.PhpVariable && query.ExpressionOperator(node) == "=" {
			defined[query.VariableKey(children[0])] = true
		}
	case syntax.PhpIfStatement:
		var branches []map[string]bool
		hasElse := false
		for _, branch := range query.ConditionalBranches(node) {
			local := maps.Clone(defined)
			addDefiniteWrites(branch.Body, local)
			branches = append(branches, local)
			hasElse = hasElse || branch.Condition == nil
		}
		if !hasElse {
			branches = append(branches, maps.Clone(defined))
		}
		intersectFlow(defined, branches)
	case syntax.PhpForStatement:
		for _, init := range query.ExtractionLoopParts(node).Init {
			addDefiniteWrites(init, defined)
		}
	}
}
