package codeaction

import (
	"context"
	"fmt"
	"strings"

	"github.com/shopware/shopware-lsp/internal/extension"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	"github.com/shopware/shopware-lsp/internal/twig"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type TwigCodeActionProvider struct {
	twigIndexer      *twig.TwigIndexer
	extensionIndexer *extension.ExtensionIndexer
	projectRoot      string
}

func NewTwigCodeActionProvider(projectRoot string, server *lsp.Server) *TwigCodeActionProvider {
	provider := &TwigCodeActionProvider{projectRoot: projectRoot}

	if indexer, ok := server.GetIndexer("twig.indexer"); ok {
		if twigIndexer, ok := indexer.(*twig.TwigIndexer); ok {
			provider.twigIndexer = twigIndexer
		}
	}

	if indexer, ok := server.GetIndexer("extension.indexer"); ok {
		if extensionIndexer, ok := indexer.(*extension.ExtensionIndexer); ok {
			provider.extensionIndexer = extensionIndexer
		}
	}

	return provider
}

func (p *TwigCodeActionProvider) GetCodeActionKinds() []protocol.CodeActionKind {
	return []protocol.CodeActionKind{
		protocol.CodeActionRefactorExtract,
		protocol.CodeActionQuickFix,
	}
}

func (p *TwigCodeActionProvider) GetCodeActions(ctx context.Context, params *protocol.CodeActionParams) []protocol.CodeAction {
	var codeActions []protocol.CodeAction

	blockNames := twig.ExtendBlockCandidates(params.Node, params.DocumentContent, params.Range.Start.Line)
	if len(blockNames) > 0 {
		codeActions = append(codeActions, p.getExtendBlockActions(params, blockNames)...)
	}

	if params.Node == nil {
		return codeActions
	}

	if isTwigBlockName(params.Node, params.DocumentContent) {
		if action := p.getVersioningHashAction(params); action != nil {
			codeActions = append(codeActions, *action)
		}

		if action := p.getShowDiffAction(params); action != nil {
			codeActions = append(codeActions, *action)
		}
	}

	if action := p.getShowDiffActionFromComment(params); action != nil {
		codeActions = append(codeActions, *action)
	}

	return codeActions
}

func (p *TwigCodeActionProvider) getExtendBlockActions(params *protocol.CodeActionParams, blockNames []string) []protocol.CodeAction {
	if p.extensionIndexer == nil || !twig.IsOriginalTemplateSource(params.TextDocument.URI) {
		return nil
	}

	extensions, err := p.extensionIndexer.GetAll()
	if err != nil || len(extensions) == 0 {
		return nil
	}

	var codeActions []protocol.CodeAction

	for _, ext := range extensions {
		if !ext.IsLocal() {
			continue
		}

		plan, blockName := p.planFirstExtendableBlock(params, blockNames, ext)
		if plan == nil || blockName == "" {
			continue
		}

		// After the edit is applied, reveal the new block and place the cursor inside
		// its body via window/showDocument. Clients that execute code-action commands
		// (e.g. VSCode, Neovim) honor this; clients that ignore it still get the edit.
		codeActions = append(codeActions, protocol.CodeAction{
			Title: fmt.Sprintf("Extend block '%s' in %s", blockName, ext.Name),
			Kind:  protocol.CodeActionQuickFix,
			Edit:  plan.WorkspaceEdit(),
			Command: &protocol.CommandAction{
				Title:   "Focus extended block",
				Command: lsp.FocusExtendedBlockCommand,
				Arguments: []any{
					plan.URI,
					plan.BlockLine,
				},
			},
		})
	}

	return codeActions
}

func (p *TwigCodeActionProvider) planFirstExtendableBlock(
	params *protocol.CodeActionParams,
	blockNames []string,
	ext extension.ShopwareExtension,
) (*twig.ExtendBlockPlan, string) {
	for _, blockName := range blockNames {
		plan, planErr := twig.PlanExtendBlock(p.projectRoot, p.twigIndexer, params.TextDocument.URI, blockName, ext)
		if planErr == nil {
			return plan, blockName
		}
		if planErr.Code != "block.already_exists" {
			return nil, ""
		}
	}

	return nil, ""
}

func (p *TwigCodeActionProvider) getVersioningHashAction(params *protocol.CodeActionParams) *protocol.CodeAction {
	if p.twigIndexer == nil {
		return nil
	}

	if twig.IsStorefrontTemplate(params.TextDocument.URI) {
		return nil
	}

	if !isTwigBlockName(params.Node, params.DocumentContent) {
		return nil
	}

	blockName := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)

	rootNode := treesitterhelper.RootNode(params.Node)

	twigFile, err := twig.ParseTwig(params.TextDocument.URI, rootNode, params.DocumentContent)
	if err != nil {
		return nil
	}

	// Use the parsed block's version comment rather than the tree-sitter parent:
	// blocks whose body contains raw HTML are wrapped in an ERROR node and only
	// recovered by ParseTwig's regex fallback.
	if block, ok := twigFile.Blocks[blockName]; ok && block.VersionComment != nil {
		return nil
	}

	originalHash := twig.ResolveOriginalStorefrontHashForBlock(p.twigIndexer, blockName, twigFile.ExtendsFile)
	if originalHash == nil {
		return nil
	}

	blockLine := int(params.Node.Range().StartPoint.Row)
	blockCol := int(params.Node.Range().StartPoint.Column)
	indent := extractLineIndent(params.DocumentContent, blockLine, blockCol)
	versionComment := indent + twig.FormatVersionComment(originalHash.Hash, twig.ResolveBlockVersion(p.projectRoot, originalHash))

	edit := &protocol.WorkspaceEdit{
		Changes: map[string][]protocol.TextEdit{
			params.TextDocument.URI: {
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: blockLine, Character: 0},
						End:   protocol.Position{Line: blockLine, Character: 0},
					},
					NewText: versionComment,
				},
			},
		},
	}

	return &protocol.CodeAction{
		Title: "Add twig versioning hash",
		Kind:  protocol.CodeActionQuickFix,
		Edit:  edit,
	}
}

func (p *TwigCodeActionProvider) getShowDiffAction(params *protocol.CodeActionParams) *protocol.CodeAction {
	if p.twigIndexer == nil {
		return nil
	}

	if twig.IsStorefrontTemplate(params.TextDocument.URI) {
		return nil
	}

	if !isTwigBlockName(params.Node, params.DocumentContent) {
		return nil
	}

	blockName := treesitterhelper.GetNodeText(params.Node, params.DocumentContent)

	rootNode := treesitterhelper.RootNode(params.Node)

	twigFile, err := twig.ParseTwig(params.TextDocument.URI, rootNode, params.DocumentContent)
	if err != nil {
		return nil
	}

	block, exists := twigFile.Blocks[blockName]
	if !exists || block.VersionComment == nil {
		return nil
	}

	allBlockHashes, err := p.twigIndexer.GetTwigBlockHashes(blockName)
	if err != nil || len(allBlockHashes) == 0 {
		return nil
	}

	originalHash := twig.FindOriginalStorefrontHash(allBlockHashes)
	if originalHash == nil {
		return nil
	}

	if block.VersionComment.Hash == originalHash.Hash {
		return nil
	}

	return &protocol.CodeAction{
		Title: "Show block difference",
		Kind:  protocol.CodeActionQuickFix,
		Command: &protocol.CommandAction{
			Title:     "Show Block Difference",
			Command:   "shopware.twig.showBlockDiff",
			Arguments: []any{params.TextDocument.URI, blockName},
		},
	}
}

func (p *TwigCodeActionProvider) getShowDiffActionFromComment(params *protocol.CodeActionParams) *protocol.CodeAction {
	if p.twigIndexer == nil {
		return nil
	}

	if twig.IsStorefrontTemplate(params.TextDocument.URI) {
		return nil
	}

	if params.Node.Kind() != "comment" {
		return nil
	}

	commentText := string(params.Node.Utf8Text(params.DocumentContent))
	if !strings.Contains(commentText, twig.VersionCommentPrefix) {
		return nil
	}

	versionComment := twig.ParseVersionComment(commentText, int(params.Node.Range().StartPoint.Row)+1)
	if versionComment == nil {
		return nil
	}

	commentLine := int(params.Node.Range().StartPoint.Row) + 1

	rootNode := params.Node
	for rootNode.Parent() != nil {
		rootNode = rootNode.Parent()
	}

	twigFile, err := twig.ParseTwig(params.TextDocument.URI, rootNode, params.DocumentContent)
	if err != nil {
		return nil
	}

	var blockName string
	for _, block := range twigFile.Blocks {
		if block.VersionComment != nil && block.VersionComment.Line == commentLine {
			blockName = block.Name
			break
		}
	}

	if blockName == "" {
		return nil
	}

	allBlockHashes, err := p.twigIndexer.GetTwigBlockHashes(blockName)
	if err != nil || len(allBlockHashes) == 0 {
		return nil
	}

	originalHash := twig.FindOriginalStorefrontHash(allBlockHashes)
	if originalHash == nil {
		return nil
	}

	if versionComment.Hash == originalHash.Hash {
		return nil
	}

	return &protocol.CodeAction{
		Title: "Show block difference",
		Kind:  protocol.CodeActionQuickFix,
		Command: &protocol.CommandAction{
			Title:     "Show Block Difference",
			Command:   "shopware.twig.showBlockDiff",
			Arguments: []any{params.TextDocument.URI, blockName},
		},
	}
}

// isTwigBlockName reports whether node is the block-name identifier in a
// "{% block NAME %}" tag. It accepts both a proper "block" parent and the ERROR
// parent tree-sitter produces when the block body contains raw HTML.
func isTwigBlockName(node *tree_sitter.Node, content []byte) bool {
	if node == nil || node.Kind() != "identifier" {
		return false
	}
	if parent := node.Parent(); parent != nil && parent.Kind() == "block" {
		return true
	}
	return precededByBlockKeyword(content, int(node.StartByte()))
}

// precededByBlockKeyword reports whether the bytes before offset form a
// "{% block" opening tag, tolerating whitespace and the "-" whitespace-control
// modifier ("{%- block").
func precededByBlockKeyword(content []byte, offset int) bool {
	if offset > len(content) {
		return false
	}
	i := offset - 1
	skipSpace := func() {
		for i >= 0 && (content[i] == ' ' || content[i] == '\t') {
			i--
		}
	}

	skipSpace()
	end := i + 1
	for i >= 0 && isWordByte(content[i]) {
		i--
	}
	if string(content[i+1:end]) != "block" {
		return false
	}

	skipSpace()
	if i >= 0 && content[i] == '-' { // {%- block
		i--
		skipSpace()
	}
	return i >= 1 && content[i] == '%' && content[i-1] == '{'
}

func isWordByte(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// extractLineIndent returns the leading whitespace of the given line, capped at
// maxCol, so an inserted version comment keeps the block's indentation.
func extractLineIndent(content []byte, line, maxCol int) string {
	currentLine := 0
	lineStart := 0

	for i, b := range content {
		if currentLine == line {
			lineStart = i
			break
		}
		if b == '\n' {
			currentLine++
		}
	}

	end := lineStart + maxCol
	if end > len(content) {
		end = len(content)
	}

	indent := content[lineStart:end]

	// Only return actual whitespace characters
	for i, b := range indent {
		if b != ' ' && b != '\t' {
			return string(indent[:i])
		}
	}

	return string(indent)
}
