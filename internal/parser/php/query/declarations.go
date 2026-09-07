package query

import (
	"github.com/shopware/shopware-lsp/internal/parser/php/syntax"
	"strings"
)

// ConstantNames returns only declared names, excluding names in initializers
// and native types. A declaration can introduce several constants.
func ConstantNames(node *syntax.Node) []*syntax.Node {
	if node == nil || (node.Kind() != syntax.PhpConstDeclaration && node.Kind() != syntax.PhpClassConstDeclaration) {
		return nil
	}
	var names []*syntax.Node
	expectName := false
	for i := 0; i < node.ChildCount(); i++ {
		switch child := node.Child(i).(type) {
		case *syntax.Token:
			if strings.EqualFold(child.Text(), "const") || child.Kind() == syntax.TkComma {
				expectName = true
			}
			if child.Kind() == syntax.TkEquals {
				expectName = false
			}
		case *syntax.Node:
			if expectName && child.Kind() == syntax.PhpName {
				names = append(names, child)
				expectName = false
			}
		}
	}
	return names
}
