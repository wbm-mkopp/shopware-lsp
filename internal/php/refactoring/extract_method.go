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

// MethodNameAvailable checks inherited and trait methods in the shared snapshot.
type MethodNameAvailable func(class *cst.Node, name string) bool

type methodFlow struct {
	controlReturns          []*cst.Node
	valueReturn, voidReturn bool
	inputs                  []string
	outputs                 []string
	references              map[string]bool
	returns                 bool
	bareReturn              bool
}

// ExtractMethod supports straight-line statements with at most one live output.
// Unknown call arguments are conservatively treated as possible reference writes.
func ExtractMethod(ctx context.Context, root *cst.Node, source string, selection cst.TextRange, available MethodNameAvailable) (*Extraction, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	selected := query.SelectMethodStatements(ctx, root, source, selection)
	if selected == nil || !methodScopeSafe(ctx, selected.Method) {
		return nil, nil
	}
	flow, ok := analyzeMethodFlow(ctx, selected)
	if !ok {
		return nil, ctx.Err()
	}
	baseName := "extractedMethod"
	if selected.Class == nil {
		baseName = "extractedFunction"
	}
	name := baseName
	lowerSource := strings.ToLower(source)
	for suffix := 2; strings.Contains(lowerSource, strings.ToLower(name)) || available != nil && !available(selected.Class, name); suffix++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		// An unavailable semantic hierarchy must not cause an unbounded name search.
		if suffix > 1000 {
			return nil, nil
		}
		name = fmt.Sprintf("%s%d", baseName, suffix)
	}
	return buildMethodExtraction(source, selected, flow, name)
}

func analyzeMethodFlow(ctx context.Context, selected *query.MethodSelection) (methodFlow, bool) {
	base, references := extractionAvailable(selected)
	f := &extractionFlow{ctx: ctx, base: base, references: references, inputSet: map[string]bool{}, writes: map[string]bool{}, static: query.DeclarationIsStatic(selected.Method)}
	env := map[string]bool{}
	returns, ok := f.statements(selected.Statements, env, 0)
	if !ok {
		return methodFlow{}, false
	}
	flow := methodFlow{returns: returns, references: references}
	for _, ret := range f.returns {
		if len(query.ExpressionChildren(ret)) == 0 {
			flow.voidReturn = true
		} else {
			flow.valueReturn = true
		}
	}
	flow.bareReturn = flow.voidReturn && !flow.valueReturn
	if len(f.returns) > 0 && (!returns || flow.voidReturn && flow.valueReturn) {
		flow.controlReturns = f.returns
		// Tagged early returns cannot retain a newly created local in the
		// caller on every path. Its destructor could otherwise run too soon.
		for _, name := range f.retained {
			if !base[name] {
				return methodFlow{}, false
			}
		}
	}
	if !returns {

		emitted := map[string]bool{}
		for element := range selected.Method.Descendants() {
			if ctx.Err() != nil {
				return methodFlow{}, false
			}
			token, ok := element.(*cst.Token)
			if !ok || token.Range().Start >= selected.Range.Start && token.Range().Start < selected.Range.End {
				continue
			}
			var names []string
			if token.Kind() == syntax.TkVariable {
				names = []string{token.Text()}
			}
			if token.Kind() == syntax.TkString {
				names, _ = query.StringVariables(token)
			}
			for _, name := range names {
				// Reads before the selection also matter when extracting a loop
				// body: the next iteration can consume a value written here.
				if !f.writes[name] || emitted[name] || name == "$this" {
					continue
				}
				if !env[name] && !f.read(name, env) {
					return methodFlow{}, false
				}
				if !base[name] && !env[name] {
					return methodFlow{}, false
				}
				emitted[name] = true
				flow.outputs = append(flow.outputs, name)
			}
		}
	}

	if !returns {
		for _, name := range f.retained {
			found := false
			for _, output := range flow.outputs {
				found = found || output == name
			}
			if found {
				continue
			}
			if !env[name] && !f.read(name, env) {
				return methodFlow{}, false
			}
			flow.outputs = append(flow.outputs, name)
		}
	}
	flow.inputs = f.inputs
	return flow, true
}

// methodScopeSafe excludes scope-dependent constructs and hidden variable reads.
// Simple string interpolation is analyzed through native token queries.
func methodScopeSafe(ctx context.Context, method *cst.Node) bool {
	for element := range method.Descendants() {
		if ctx.Err() != nil {
			return false
		}
		if node, ok := element.(*cst.Node); ok {
			switch node.Kind() {
			case syntax.Error, syntax.PhpGlobalStatement, syntax.PhpStaticStatement, syntax.PhpTryStatement,
				syntax.PhpYieldExpression:
				return false
			}
		}
		if token, ok := element.(*cst.Token); ok {
			if token.Kind() == syntax.TkAmpersand && token.Parent().Kind() != syntax.PhpParameter {
				return false
			}
			if token.Kind() == syntax.TkString {
				if _, ok := query.StringVariables(token); !ok {
					return false
				}
			}
			switch strings.ToLower(token.Text()) {
			case "$", "eval", "extract", "compact", "get_defined_vars", "unset",
				"func_get_args", "func_get_arg", "func_num_args", "debug_backtrace",
				"__method__", "__function__", "__line__", "include", "include_once", "require", "require_once", "goto":
				return false
			}
		}
	}
	return true
}

func buildMethodExtraction(source string, selected *query.MethodSelection, flow methodFlow, name string) (*Extraction, error) {
	newline := "\n"
	if strings.Contains(source, "\r\n") {
		newline = "\r\n"
	}
	methodIndent := sourceIndent(source, selected.Method.RangeTrimmedTrivia().Start)
	statementIndent := sourceIndent(source, selected.Range.Start)
	unit := strings.TrimPrefix(statementIndent, methodIndent)
	if unit == "" || unit == statementIndent && methodIndent != "" {
		unit = "    "
	}
	if methodIndent == "" && selected.Class != nil {
		methodIndent = sourceIndent(source, selected.Class.RangeTrimmedTrivia().Start) + unit
	}
	static := query.DeclarationIsStatic(selected.Method)
	call := "$this->" + name
	modifier := ""
	visibility := "private "
	if selected.Class == nil {
		call = name
		visibility = ""
	}
	if static && selected.Class != nil {
		call = "self::" + name
		modifier = "static "
	}
	call += "(" + strings.Join(flow.inputs, ", ") + ");"
	switch {
	case len(flow.controlReturns) > 0:
		call = controlReturnCall(source, call, flow, newline, statementIndent)
	case flow.returns && flow.bareReturn:
		call += newline + statementIndent + "return;"
	case flow.returns:
		call = "return " + call
	case len(flow.outputs) > 0:
		call = flowOutput(flow.outputs) + " = " + call
	}
	body := reindentExtraction(source, selected.Range, statementIndent, methodIndent+unit, selected.Statements)
	if len(flow.controlReturns) > 0 {
		var err error
		body, err = reindentControlReturns(source, selected.Range, statementIndent, methodIndent+unit, selected.Statements, flow.controlReturns)
		if err != nil {
			return nil, err
		}
	}
	if len(flow.controlReturns) > 0 {
		output := "null"
		if len(flow.outputs) > 0 {
			output = flowOutput(flow.outputs)
		}
		body += newline + methodIndent + unit + "return [0, " + output + "];"
	} else if len(flow.outputs) > 0 {
		body += newline + methodIndent + unit + "return " + flowOutput(flow.outputs) + ";"
	}
	parameters := append([]string(nil), flow.inputs...)
	for i, parameter := range parameters {
		if flow.references[parameter] {
			parameters[i] = "&" + parameter
		}
	}
	method := newline + newline + methodIndent + visibility + modifier + "function " + name + "(" + strings.Join(parameters, ", ") + ")" + newline + methodIndent + "{" + newline + body + newline + methodIndent + "}"
	// Insert immediately after the original method, preserving its surrounding
	// class comments and the existing closing-brace indentation.
	offset := selected.Method.RangeTrimmedTrivia().End
	builder := rewrite.NewBuilder(source)
	if err := builder.ReplaceRange(selected.Range, call); err != nil {
		return nil, err
	}
	if err := builder.Insert(offset, method); err != nil {
		return nil, err
	}
	edits, err := builder.Finish()
	return &Extraction{Name: name, Edits: edits}, err
}

func sourceIndent(source string, offset uint32) string {
	start := strings.LastIndexByte(source[:offset], '\n') + 1
	indent := source[start:offset]
	if strings.TrimSpace(indent) != "" {
		return ""
	}
	return indent
}

func flowOutput(names []string) string {
	if len(names) == 1 {
		return names[0]
	}
	return "[" + strings.Join(names, ", ") + "]"
}
