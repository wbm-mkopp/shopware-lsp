package refactoring

import (
	"context"
	"maps"
	"strconv"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	"github.com/shopware/shopware-lsp/internal/parser/php/query"
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
)

type extractionFlow struct {
	retained         []string
	retainedSet      map[string]bool
	returns          []*cst.Node
	ctx              context.Context
	base, references map[string]bool
	inputs           []string
	inputSet, writes map[string]bool
	static           bool
}

func (f *extractionFlow) read(name string, env map[string]bool) bool {
	if name == "$this" {
		return !f.static
	}
	if env[name] {
		return true
	}
	if !f.base[name] {
		return false
	}
	if !f.inputSet[name] {
		f.inputSet[name] = true
		f.inputs = append(f.inputs, name)
	}
	env[name] = true
	return true
}
func (f *extractionFlow) write(name string, env map[string]bool) bool {
	if name == "$this" {
		return false
	}
	if f.base[name] {
		f.references[name] = true
	}
	if f.references[name] && !f.read(name, env) {
		return false
	}
	f.writes[name], env[name] = true, true
	return true
}
func (f *extractionFlow) expression(node *cst.Node, env map[string]bool) bool {
	if f.ctx.Err() != nil {
		return false
	}
	children := query.ExpressionChildren(node)
	switch node.Kind() {
	case syntax.PhpAssignmentExpression:
		if len(children) != 2 {
			return false
		}
		lhs, rhs := children[0], children[1]
		if query.ExpressionOperator(node) != "=" && !f.expression(lhs, env) {
			return false
		}
		if !f.expression(rhs, env) {
			return false
		}
		if lhs.Kind() == syntax.PhpVariable && extractionMayRetainObject(rhs) {
			name := query.VariableKey(lhs)
			if f.retainedSet == nil {
				f.retainedSet = map[string]bool{}
			}
			if !f.retainedSet[name] {
				f.retainedSet[name] = true
				f.retained = append(f.retained, name)
			}
		}
		return f.target(lhs, env)
	case syntax.PhpUnaryExpression:
		op := query.ExpressionOperator(node)
		if (op == "++" || op == "--") && len(children) == 1 {
			return f.expression(children[0], env) && f.target(children[0], env)
		}
	case syntax.PhpVariable:
		return f.read(query.VariableKey(node), env)
	case syntax.PhpClosure, syntax.PhpArrowFunction, syntax.PhpAnonymousClass, syntax.PhpYieldExpression:
		return false
	case syntax.PhpBinaryExpression, syntax.PhpTernaryExpression, syntax.PhpMatchExpression:
		// Branch-local writes require expression-level CFG handling. Reads and
		// pure branch expressions are supported without treating writes as definite.
		for element := range node.Descendants() {
			if child, ok := element.(*cst.Node); ok && child.Kind() == syntax.PhpAssignmentExpression {
				return false
			}
		}
	}
	for child := range node.ChildNodes() {
		if !f.expression(child, env) {
			return false
		}
	}
	for token := range node.ChildTokens() {
		if token.Kind() == syntax.TkVariable && !f.read(token.Text(), env) {
			return false
		}
		if token.Kind() == syntax.TkString {
			names, ok := query.StringVariables(token)
			if !ok {
				return false
			}
			for _, name := range names {
				if !f.read(name, env) {
					return false
				}
			}
		}
	}
	if node.Kind() == syntax.PhpArgument || node.Kind() == syntax.PhpNamedArgument {
		for element := range node.Descendants() {
			if token, ok := element.(*cst.Token); ok && token.Kind() == syntax.TkVariable && token.Text() != "$this" {
				// A call may write through a reference argument, but cannot make an
				// otherwise undefined input safe to read.
				f.writes[token.Text()] = true
				if f.base[token.Text()] {
					f.references[token.Text()] = true
				}
			}
		}
	}
	return true
}
func (f *extractionFlow) target(node *cst.Node, env map[string]bool) bool {
	switch node.Kind() {
	case syntax.PhpVariable:
		return f.write(query.VariableKey(node), env)
	case syntax.PhpArrayAccess:
		if !f.expression(node, env) {
			return false
		}
		for child := range node.ChildNodes() {
			return f.target(child, env)
		}
	case syntax.PhpMemberAccess, syntax.PhpScopedAccess:
		return f.expression(node, env)
	case syntax.PhpArray:
		for _, item := range query.ArrayItems(node) {
			if key := query.ArrayItemKey(item); key != nil && !f.expression(key, env) {
				return false
			}
			value := query.ArrayItemValue(item)
			if value == nil || value.Kind() != syntax.PhpVariable || !f.write(query.VariableKey(value), env) {
				return false
			}
		}
		return true
	}
	return false
}

// statements returns whether all paths return from the enclosing callable.
func (f *extractionFlow) statements(nodes []*cst.Node, env map[string]bool, loops int) (bool, bool) {
	for i, node := range nodes {
		returns, ok := f.statement(node, env, loops)
		if !ok {
			return false, false
		}
		if returns {
			return true, i == len(nodes)-1
		}
		if node.Kind() == syntax.PhpBreakStatement || node.Kind() == syntax.PhpContinueStatement {
			return false, true
		}
	}
	return false, true
}
func (f *extractionFlow) statement(node *cst.Node, env map[string]bool, loops int) (bool, bool) {
	if f.ctx.Err() != nil {
		return false, false
	}
	switch node.Kind() {
	case syntax.PhpBlock:
		return f.statements(query.ExpressionChildren(node), env, loops)
	case syntax.PhpExpressionStatement, syntax.PhpEchoStatement, syntax.PhpThrowStatement:
		return false, f.expression(node, env)
	case syntax.PhpReturnStatement:
		f.returns = append(f.returns, node)
		return true, f.expression(node, env)
	case syntax.PhpIfStatement:
		return f.conditional(node, env, loops)
	case syntax.PhpSwitchStatement:
		return false, f.switchStatement(node, env, loops)
	case syntax.PhpForStatement, syntax.PhpForeachStatement, syntax.PhpWhileStatement, syntax.PhpDoWhileStatement:
		return false, f.loop(node, env, loops)
	case syntax.PhpBreakStatement, syntax.PhpContinueStatement:
		level := 1
		for _, child := range query.ExpressionChildren(node) {
			if child.Kind() != syntax.PhpNumber {
				return false, false
			}
			var err error
			level, err = strconv.Atoi(child.Text())
			if err != nil {
				return false, false
			}
		}
		return false, level > 0 && level <= loops
	}
	return false, false
}
func (f *extractionFlow) conditional(node *cst.Node, env map[string]bool, loops int) (bool, bool) {
	before := maps.Clone(env)
	var states []map[string]bool
	allReturn, hasElse := true, false
	for _, branch := range query.ConditionalBranches(node) {
		if branch.Condition != nil {
			if !f.expression(branch.Condition, before) {
				return false, false
			}
		} else {
			hasElse = true
		}
		local := maps.Clone(before)
		ret, ok := f.statement(branch.Body, local, loops)
		if !ok {
			return false, false
		}
		allReturn = allReturn && ret
		if !ret {
			states = append(states, local)
		}
	}
	if !hasElse {
		states = append(states, before)
		allReturn = false
	}

	intersectFlow(env, states)
	return allReturn, true
}
func (f *extractionFlow) loop(node *cst.Node, env map[string]bool, loops int) bool {
	parts := query.ExtractionLoopParts(node)
	for _, expression := range parts.Init {
		if !f.expression(expression, env) {
			return false
		}
	}
	if node.Kind() != syntax.PhpDoWhileStatement {
		for _, expression := range parts.Condition {
			if !f.expression(expression, env) {
				return false
			}
		}
	}
	local := maps.Clone(env)
	for _, target := range parts.Targets {
		if !f.target(target, local) {
			return false
		}
	}
	if parts.Body == nil {
		return false
	}
	if _, ok := f.statement(parts.Body, local, loops+1); !ok {
		return false
	}
	for _, expression := range parts.Update {
		if !f.expression(expression, local) {
			return false
		}
	}
	if node.Kind() == syntax.PhpDoWhileStatement {
		for _, expression := range parts.Condition {
			if !f.expression(expression, local) {
				return false
			}
		}
	}
	return true // The body may execute zero times or exit before a write.
}
func intersectFlow(target map[string]bool, states []map[string]bool) {
	clear(target)
	if len(states) == 0 {
		return
	}
	for name := range states[0] {
		present := true
		for _, state := range states[1:] {
			present = present && state[name]
		}
		if present {
			target[name] = true
		}
	}
}

func (f *extractionFlow) switchStatement(node *cst.Node, env map[string]bool, loops int) bool {
	var states []map[string]bool
	hasDefault := false
	for child := range node.ChildNodes() {
		if child.Kind() != syntax.PhpCaseClause {
			if !f.expression(child, env) {
				return false
			}
			continue
		}
		local := maps.Clone(env)
		isDefault := false
		for token := range child.ChildTokens() {
			if token.Text() == "default" {
				isDefault = true
				hasDefault = true
			}
		}
		nodes := query.ExpressionChildren(child)
		if !isDefault {
			if len(nodes) == 0 || !f.expression(nodes[0], local) {
				return false
			}
			nodes = nodes[1:]
		}
		if _, ok := f.statements(nodes, local, loops+1); !ok {
			return false
		}
		states = append(states, local)
	}
	if !hasDefault {
		states = append(states, maps.Clone(env))
	}
	intersectFlow(env, states)
	return true
}

// Locals holding an object (including one nested in an array) must remain in
// the caller until their original lifetime ends, even without a later read.
func extractionMayRetainObject(node *cst.Node) bool {
	switch node.Kind() {
	case syntax.PhpNumber, syntax.PhpBoolean, syntax.PhpNull, syntax.PhpString,
		syntax.PhpBinaryExpression, syntax.PhpUnaryExpression:
		return false
	case syntax.PhpParenthesized:
		children := query.ExpressionChildren(node)
		return len(children) != 1 || extractionMayRetainObject(children[0])
	}
	return true
}
