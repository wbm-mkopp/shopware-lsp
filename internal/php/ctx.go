package php

import (
	"context"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// phpContextKey is a custom type for the context key to avoid collisions
type phpContextKey string

// PHPContextKey is the key used to store PHP context in the context.Context
const PHPContextKey phpContextKey = "php.context"

type PHPContext struct {
	InsideClass *PHPClass
	Node        *tree_sitter.Node
}

func GetPHPContext(ctx context.Context) *PHPContext {
	return ctx.Value(PHPContextKey).(*PHPContext)
}

func (p *PHPIndex) AddContext(ctx context.Context, node *tree_sitter.Node, documentContent []byte) context.Context {
	className := treesitterhelper.GetClassName(node, documentContent)
	class := p.GetClass(className)

	return context.WithValue(ctx, PHPContextKey, &PHPContext{
		InsideClass: class,
		Node:        node,
	})
}
