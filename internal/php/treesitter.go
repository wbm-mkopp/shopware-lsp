package php

import (
	"context"
	"slices"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func (s *PHPIndex) IsMethodCalledName(ctx context.Context, node *tree_sitter.Node, content []byte, methodNames ...string) bool {
	current := node
	for current != nil && current.Kind() != "member_call_expression" {
		current = current.Parent()
	}

	if current == nil {
		return false
	}

	methodNameNode := treesitterhelper.GetFirstNodeOfKind(current, "name")
	if methodNameNode == nil {
		return false
	}

	return slices.Contains(methodNames, string(methodNameNode.Utf8Text(content)))
}

func (s *PHPIndex) IsMethodCalledOnClass(ctx context.Context, node *tree_sitter.Node, content []byte, className string) bool {
	current := node
	for current != nil && current.Kind() != "member_call_expression" {
		current = current.Parent()
	}

	if current == nil {
		return false
	}

	return s.GetTypeOfNode(ctx, current, content).Matches(NewPHPType(className))
}
