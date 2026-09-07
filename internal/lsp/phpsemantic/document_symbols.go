package phpsemantic

import (
	"context"
	"strings"

	"github.com/shopware/shopware-lsp/internal/language"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	"github.com/shopware/shopware-lsp/internal/parser/cst"
	phpquery "github.com/shopware/shopware-lsp/internal/parser/php/query"
	phpsyntax "github.com/shopware/shopware-lsp/internal/parser/php/syntax"
)

// GetDocumentSymbols builds the outline directly from the open CST, without
// requiring indexing or type inference. Recovery nodes retain usable children.
func (p *Provider) GetDocumentSymbols(ctx context.Context, request *lsp.DocumentSymbolRequest) ([]protocol.DocumentSymbol, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request == nil || !phpEditorDocument(request.Document) {
		return nil, nil
	}
	builder := phpOutline{ctx: ctx, lines: request.Document.LineIndex}
	result := builder.children(request.Document.SyntaxTree.Root)
	return result, ctx.Err()
}

func phpEditorDocument(document *lsp.TextDocument) bool {
	return document != nil && document.SyntaxLanguage == language.PHP && document.SyntaxTree != nil && document.SyntaxTree.Root != nil && document.LineIndex != nil
}

type phpOutline struct {
	ctx   context.Context
	lines *cst.LineIndex
}

func (b phpOutline) children(parent *phpsyntax.Node) []protocol.DocumentSymbol {
	var result []protocol.DocumentSymbol
	namespace := -1
	for i := 0; i < parent.ChildCount(); i++ {
		if b.ctx.Err() != nil {
			break
		}
		node, ok := parent.Child(i).(*phpsyntax.Node)
		if !ok {
			continue
		}
		if node.Kind() == phpsyntax.PhpNamespace {
			if namespace >= 0 {
				result[namespace].Range.End = rangeFromText(b.lines, node.RangeTrimmedTrivia()).Start
			}
			namespace = -1
			symbols := b.node(node)
			result = append(result, symbols...)
			if len(symbols) > 0 && phpquery.DirectChild(node, phpsyntax.PhpBlock) == nil {
				namespace = len(result) - 1
				result[namespace].Range.End = rangeFromText(b.lines, parent.Range()).End
			}
			continue
		}
		symbols := b.node(node)
		if namespace >= 0 {
			result[namespace].Children = append(result[namespace].Children, symbols...)
		} else {
			result = append(result, symbols...)
		}
	}
	return result
}

func (b phpOutline) node(node *phpsyntax.Node) []protocol.DocumentSymbol {
	kind, named := outlineKind(node.Kind())
	if named {
		name := phpquery.DirectChild(node, phpsyntax.PhpName)
		if name == nil {
			if node.Kind() == phpsyntax.PhpNamespace {
				if keyword := node.ChildTokenOfKind(phpsyntax.TkKeyword); keyword != nil {
					symbol := b.symbol(node, keyword, "(global)", protocol.SymbolNamespace)
					symbol.Children = b.children(node)
					return []protocol.DocumentSymbol{symbol}
				}
			}
			return b.children(node)
		}
		symbol := b.symbol(node, name, phpquery.NameValue(name), kind)
		symbol.Children = b.children(node)
		if node.Kind() == phpsyntax.PhpMethodDeclaration && strings.EqualFold(symbol.Name, "__construct") {
			symbol.Kind = protocol.SymbolConstructor
			// Promoted parameters are properties of the class, not of the constructor.
			result := []protocol.DocumentSymbol{symbol}
			for _, parameter := range phpquery.Parameters(node) {
				if phpquery.DeclarationVisibility(parameter) == "" {
					continue
				}
				variable := phpquery.DirectChild(parameter, phpsyntax.PhpVariable)
				if variable != nil {
					result = append(result, b.symbol(parameter, variable, phpquery.VariableName(variable), protocol.SymbolProperty))
				}
			}
			return result
		}
		return []protocol.DocumentSymbol{symbol}
	}
	switch node.Kind() {
	case phpsyntax.PhpAnonymousClass:
		keyword := node.ChildTokenOfKind(phpsyntax.TkKeyword)
		if keyword == nil {
			return nil
		}
		symbol := b.symbol(node, keyword, "(anonymous class)", protocol.SymbolClass)
		symbol.Children = b.children(node)
		return []protocol.DocumentSymbol{symbol}
	case phpsyntax.PhpPropertyDeclaration:
		var result []protocol.DocumentSymbol
		for _, variable := range phpquery.PropertyVariables(node) {
			if phpquery.VariableName(variable) == "" {
				continue
			}
			symbol := b.symbol(node, variable, phpquery.VariableName(variable), protocol.SymbolProperty)
			symbol.Detail = phpquery.PropertyType(node)
			result = append(result, symbol)
		}
		return result
	case phpsyntax.PhpConstDeclaration, phpsyntax.PhpClassConstDeclaration:
		var result []protocol.DocumentSymbol
		for _, name := range phpquery.ConstantNames(node) {
			result = append(result, b.symbol(node, name, phpquery.NameValue(name), protocol.SymbolConstant))
		}
		return result
	case phpsyntax.PhpParameterList, phpsyntax.PhpAttributeGroup, phpsyntax.PhpUseDeclaration, phpsyntax.PhpTraitUseDeclaration:
		return nil
	default:
		return b.children(node)
	}
}

func (b phpOutline) symbol(node *phpsyntax.Node, name cst.Element, label string, kind protocol.SymbolKind) protocol.DocumentSymbol {
	selection := name.Range()
	if nameNode, ok := name.(*phpsyntax.Node); ok {
		selection = nameNode.RangeTrimmedTrivia()
	}
	return protocol.DocumentSymbol{Name: label, Kind: kind, Range: *rangeFromText(b.lines, node.RangeTrimmedTrivia()), SelectionRange: *rangeFromText(b.lines, selection)}
}

func outlineKind(kind phpsyntax.Kind) (protocol.SymbolKind, bool) {
	switch kind {
	case phpsyntax.PhpNamespace:
		return protocol.SymbolNamespace, true
	case phpsyntax.PhpClassDeclaration:
		return protocol.SymbolClass, true
	case phpsyntax.PhpInterfaceDeclaration:
		return protocol.SymbolInterface, true
	case phpsyntax.PhpTraitDeclaration:
		return protocol.SymbolClass, true
	case phpsyntax.PhpEnumDeclaration:
		return protocol.SymbolEnum, true
	case phpsyntax.PhpEnumCaseDeclaration:
		return protocol.SymbolEnumMember, true
	case phpsyntax.PhpMethodDeclaration:
		return protocol.SymbolMethod, true
	case phpsyntax.PhpFunctionDeclaration:
		return protocol.SymbolFunction, true
	default:
		return 0, false
	}
}

var _ lsp.DocumentSymbolProvider = (*Provider)(nil)
