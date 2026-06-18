package twig

import (
	"os"
	"path"
	"strings"

	"github.com/shopware/shopware-lsp/internal/extension"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	tree_sitter_helper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

const extendBlockBodyCursorLineOffset = 1

// ExtendBlockPlan describes how to extend a Twig block in a local extension.
type ExtendBlockPlan struct {
	URI         string
	Path        string
	OldContent  []byte
	NewContent  []byte
	FileExisted bool
	BlockLine   int // 1-based line where the cursor should be placed inside the new block body
}

func PlanExtendBlock(projectRoot string, twigIndexer *TwigIndexer, sourceURI, blockName string, ext extension.ShopwareExtension) (*ExtendBlockPlan, *protocol.ShopwareLspError) {
	originalPath := strings.TrimPrefix(sourceURI, "file://")

	if !IsOriginalTemplateSource(originalPath) {
		return nil, protocol.NewLspError("Not a storefront view file", "view.not_storefront")
	}

	storefrontRelativePath := StorefrontViewRelativePath(originalPath)
	if storefrontRelativePath == "" {
		return nil, protocol.NewLspError("Not a storefront view file", "view.not_storefront")
	}

	extendsRelPath := ConvertToRelativePath(originalPath)
	if extendsRelPath == "" {
		return nil, protocol.NewLspError("Failed to resolve template path", "view.path_failed")
	}

	extensionViewPath := path.Join(ext.GetStorefrontViewsPath(), storefrontRelativePath)
	if err := os.MkdirAll(path.Dir(extensionViewPath), 0755); err != nil {
		return nil, protocol.NewLspError("Failed to create directory", "directory.create_failed")
	}

	var currentContent []byte
	fileExists := false
	if data, err := os.ReadFile(extensionViewPath); err == nil {
		currentContent = data
		fileExists = true
	}

	if blockExistsInContent(currentContent, blockName) {
		return nil, protocol.NewLspError("Block already exists", "block.already_exists")
	}

	versionComment := resolveVersionCommentForExtendBlock(
		projectRoot,
		twigIndexer,
		originalPath,
		extendsRelPath,
		blockName,
	)

	blockSuffix := "\n\n" + versionComment + "{% block " + blockName + " %}\n\n{% endblock %}\n"

	var newContent []byte
	if !fileExists {
		extendsLine := "{% sw_extends \"" + extendsRelPath + "\" %}\n"
		newContent = []byte(extendsLine + blockSuffix)
	} else {
		newContent = append(currentContent, []byte(blockSuffix)...)
	}

	cursorLine, err := extendBlockCursorLineInContent(newContent, blockName)
	if err != nil {
		return nil, protocol.NewLspError("Block not found after planning", "block.not_found")
	}

	return &ExtendBlockPlan{
		URI:         "file://" + extensionViewPath,
		Path:        extensionViewPath,
		OldContent:  currentContent,
		NewContent:  newContent,
		FileExisted: fileExists,
		BlockLine:   cursorLine,
	}, nil
}

func (p *ExtendBlockPlan) WorkspaceEdit() *protocol.WorkspaceEdit {
	edit := p.workspaceTextEdit()
	return &protocol.WorkspaceEdit{
		Changes: map[string][]protocol.TextEdit{
			p.URI: {edit},
		},
		DocumentChanges: []protocol.DocumentChange{
			{
				TextDocument: protocol.OptionalVersionedTextDocumentIdentifier{
					URI:     p.URI,
					Version: nil,
				},
				Edits: []protocol.TextEdit{edit},
			},
		},
	}
}

func (p *ExtendBlockPlan) workspaceTextEdit() protocol.TextEdit {
	if !p.FileExisted {
		return protocol.TextEdit{
			Range: protocol.Range{
				Start: protocol.Position{Line: 0, Character: 0},
				End:   protocol.Position{Line: 0, Character: 0},
			},
			NewText: string(p.NewContent),
		}
	}

	end := endOfFileRange(p.OldContent)
	return protocol.TextEdit{
		Range:   end,
		NewText: string(p.NewContent[len(p.OldContent):]),
	}
}

func resolveVersionCommentForExtendBlock(
	projectRoot string,
	twigIndexer *TwigIndexer,
	sourcePath, extendsRelPath, blockName string,
) string {
	var originalHash *TwigBlockHash

	if twigIndexer != nil {
		if allBlockHashes, err := twigIndexer.GetTwigBlockHashes(blockName); err == nil {
			originalHash = FindOriginalStorefrontHashForExtends(allBlockHashes, extendsRelPath)
		}
	}

	if originalHash == nil {
		hash, err := FindBlockHashInTemplateFile(sourcePath, blockName)
		if err == nil && hash != nil {
			originalHash = hash
		}
	}

	if originalHash == nil {
		return ""
	}

	return FormatVersionComment(originalHash.Hash, DetectShopwareVersion(projectRoot))
}

func blockExistsInContent(content []byte, blockName string) bool {
	if len(content) == 0 {
		return false
	}

	if blockNameDeclaredInContent(content, blockName) {
		return true
	}

	parser := tree_sitter.NewParser()
	_ = parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language()))

	tree := parser.Parse(content, nil)
	blocks := tree_sitter_helper.FindAll(tree.RootNode(), tree_sitter_helper.TwigBlockWithNamePattern(blockName), content)

	return len(blocks) > 0
}

func blockOpeningLineInContent(content []byte, blockName string) (int, error) {
	parser := tree_sitter.NewParser()
	_ = parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language()))

	tree := parser.Parse(content, nil)
	blocks := tree_sitter_helper.FindAll(tree.RootNode(), tree_sitter_helper.TwigBlockWithNamePattern(blockName), content)
	if len(blocks) == 0 {
		return 0, os.ErrNotExist
	}

	return int(blocks[0].StartPosition().Row) + 1, nil
}

func extendBlockCursorLineInContent(content []byte, blockName string) (int, error) {
	blockLine, err := blockOpeningLineInContent(content, blockName)
	if err != nil {
		return 0, err
	}

	return blockLine + extendBlockBodyCursorLineOffset, nil
}

func endOfFileRange(content []byte) protocol.Range {
	if len(content) == 0 {
		return protocol.Range{
			Start: protocol.Position{Line: 0, Character: 0},
			End:   protocol.Position{Line: 0, Character: 0},
		}
	}

	line := 0
	lineStart := 0
	lastLineStart := 0

	for i, b := range content {
		if b == '\n' {
			line++
			lineStart = i + 1
			lastLineStart = lineStart
		}
	}

	character := len(content) - lastLineStart
	if line == 0 {
		character = len(content)
	}

	return protocol.Range{
		Start: protocol.Position{Line: line, Character: character},
		End:   protocol.Position{Line: line, Character: character},
	}
}
