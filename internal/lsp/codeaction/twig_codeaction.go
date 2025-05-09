package codeaction

import (
	"context"
	"strings"

	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
)

type TwigCodeActionProvider struct {
}

func NewTwigCodeActionProvider(server *lsp.Server) *TwigCodeActionProvider {
	return &TwigCodeActionProvider{}
}

func (p *TwigCodeActionProvider) GetCodeActionKinds() []protocol.CodeActionKind {
	return []protocol.CodeActionKind{
		protocol.CodeActionRefactorExtract,
	}
}

func (p *TwigCodeActionProvider) GetCodeActions(ctx context.Context, params *protocol.CodeActionParams) []protocol.CodeAction {
	if params.Node == nil {
		return nil
	}

	if !strings.Contains(params.TextDocument.URI, "Resources/views/storefront") {
		return nil
	}

	var codeActions []protocol.CodeAction

	if IsBlock().Matches(params.Node, params.DocumentContent) {
		textValue := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)

		codeActions = append(codeActions, protocol.CodeAction{
			Title: "Overwrite this block in Extension",
			Kind:  protocol.CodeActionRefactorExtract,
			Command: &protocol.CommandAction{
				Title:     "Overwrite Block",
				Command:   "shopware.twig.extendBlock",
				Arguments: []any{params.TextDocument.URI, textValue},
			},
		})
	}

	return codeActions
}

func IsBlock() treesitterhelper.Pattern {
	return treesitterhelper.And(
		treesitterhelper.NodeKind("identifier"),
		treesitterhelper.Ancestor(
			treesitterhelper.NodeKind("block"),
			1,
		),
	)
}
