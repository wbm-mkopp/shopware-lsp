// Package imports analyzes namespace imports against the current PHP snapshot.
package imports

import (
	"context"
	"strings"

	"github.com/shopware/shopware-lsp/internal/parser/cst"
	phpquery "github.com/shopware/shopware-lsp/internal/parser/php/query"
	phpsyntax "github.com/shopware/shopware-lsp/internal/parser/php/syntax"
	"github.com/shopware/shopware-lsp/internal/php/phpdoc"
	"github.com/shopware/shopware-lsp/internal/php/resolver"
	"github.com/shopware/shopware-lsp/internal/php/semantic"
)

type Item struct {
	resolver.Import
	Range     cst.TextRange
	NameRange cst.TextRange
	Used      bool
}
type Declaration struct {
	Node   *cst.Node
	Items  []Item
	Commas []cst.TextRange
}
type Scope struct {
	Unsafe       bool
	Range        cst.TextRange
	Declarations []Declaration
}
type Analysis struct{ Scopes []Scope }

// Analyze uses lexical reference names, not resolved targets: an unresolved
// class still uses its import, while a fully qualified name does not.
func Analyze(ctx context.Context, root *cst.Node, document *semantic.Document) (Analysis, error) {
	result := Analysis{Scopes: scopes(root)}
	for si := range result.Scopes {
		scope := &result.Scopes[si]
		if scope.Unsafe || len(scope.Declarations) == 0 {
			continue
		}
		used := map[resolver.ImportKind]map[string]bool{resolver.ClassImport: {}, resolver.FunctionImport: {}, resolver.ConstantImport: {}}
		mark := func(name string, kind resolver.ImportKind) {
			if strings.HasPrefix(name, "\\") || strings.HasPrefix(strings.ToLower(name), "namespace\\") {
				return
			}
			if i := strings.IndexByte(name, '\\'); i >= 0 {
				name = name[:i]
				kind = resolver.ClassImport
			}
			if kind != resolver.ConstantImport {
				name = strings.ToLower(name)
			}
			used[kind][name] = true
		}
		for _, ref := range document.References {
			if err := ctx.Err(); err != nil {
				return Analysis{}, err
			}
			if ref.Range.Start < scope.Range.Start || ref.Range.Start >= scope.Range.End {
				continue
			}
			switch ref.Kind {
			case semantic.ClassName:
				mark(ref.Name, resolver.ClassImport)
			case semantic.FunctionName:
				mark(ref.Name, resolver.FunctionImport)
			case semantic.ConstantName:
				mark(ref.Name, resolver.ConstantImport)
			}
		}
		for element := range root.Descendants() {
			if err := ctx.Err(); err != nil {
				return Analysis{}, err
			}
			if node, ok := element.(*cst.Node); ok && node.RangeTrimmedTrivia().Start >= scope.Range.Start && node.RangeTrimmedTrivia().Start < scope.Range.End && phpquery.IsConstantReference(node) {
				mark(phpquery.NameValue(node), resolver.ConstantImport)
			}
			token, ok := element.(*cst.Token)
			if !ok || token.Kind() != phpsyntax.TkBlockComment || token.Range().Start < scope.Range.Start || token.Range().Start >= scope.Range.End || !strings.HasPrefix(token.Text(), "/**") {
				continue
			}
			phpdoc.VisitReferenceNames(token.Text(), func(name string) { mark(name, resolver.ClassImport) })
		}
		for di := range scope.Declarations {
			for ii := range scope.Declarations[di].Items {
				item := &scope.Declarations[di].Items[ii]
				alias := item.Alias
				if item.Kind != resolver.ConstantImport {
					alias = strings.ToLower(alias)
				}
				item.Used = used[item.Kind][alias]
			}
		}
	}
	return result, ctx.Err()
}

func scopes(root *cst.Node) []Scope {
	if root == nil {
		return nil
	}
	result := []Scope{{Range: root.Range()}}
	current := 0
	for child := range root.ChildNodes() {
		if child.Kind() == phpsyntax.PhpNamespace {
			result[current].Range.End = child.Range().Start
			if block := phpquery.DirectChild(child, phpsyntax.PhpBlock); block != nil {
				scope := Scope{Range: block.Range()}
				for node := range block.ChildNodes() {
					appendDeclaration(&scope, node)
				}
				result = append(result, scope)
				result = append(result, Scope{Range: cst.TextRange{Start: child.Range().End, End: root.Range().End}})
				current = len(result) - 1
			} else {
				result = append(result, Scope{Range: cst.TextRange{Start: child.Range().End, End: root.Range().End}})
				current = len(result) - 1
			}
			continue
		}
		appendDeclaration(&result[current], child)
	}
	return result
}
func appendDeclaration(scope *Scope, node *cst.Node) {
	parsed, ok := phpquery.UseItems(node)
	if !ok {
		if node.Kind() == phpsyntax.PhpUseDeclaration {
			scope.Unsafe = true
		}
		return
	}
	declaration := Declaration{Node: node, Commas: parsed.Commas}
	for _, item := range parsed.Items {
		values := resolver.ParseUseDeclaration(item.Text)
		if len(values) != 1 {
			return
		}
		declaration.Items = append(declaration.Items, Item{Import: values[0], Range: item.Range, NameRange: item.NameRange})
	}
	scope.Declarations = append(scope.Declarations, declaration)
}
