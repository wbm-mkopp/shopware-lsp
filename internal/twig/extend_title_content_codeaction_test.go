package twig_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shopware/shopware-lsp/internal/extension"
	"github.com/shopware/shopware-lsp/internal/indexer"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/codeaction"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

func TestExtendBlockCodeAction_titleContentAida(t *testing.T) {
	aida := "/Users/mkopp/Projects/Customers/aida"
	vendorPath := filepath.Join(aida, "vendor/store.shopware.com/swagcustomizedproducts/src/Resources/views/storefront/component/customized-products/_include/title.html.twig")
	if _, err := os.Stat(vendorPath); err != nil {
		t.Skip("aida not available")
	}

	blockName := "swag_customized_products_option_type_template_label_content"
	content, err := os.ReadFile(vendorPath)
	require.NoError(t, err)

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()
	tree := parser.Parse(content, nil)
	defer tree.Close()

	node := treesitterhelper.FindIdentifierNode(tree.RootNode(), content, blockName)
	require.NotNil(t, node)

	tmp := t.TempDir()
	extIdx, err := extension.NewExtensionIndexer(tmp)
	require.NoError(t, err)

	phpPath := filepath.Join(aida, "src/WbmAidaCore/WbmAidaCore.php")
	phpContent, err := os.ReadFile(phpPath)
	require.NoError(t, err)

	phpParser := tree_sitter.NewParser()
	require.NoError(t, phpParser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())))
	defer phpParser.Close()
	phpTree := phpParser.Parse(phpContent, nil)
	defer phpTree.Close()
	require.NoError(t, extIdx.Index(phpPath, phpTree.RootNode(), phpContent))

	fs, err := indexer.NewFileScanner(aida, filepath.Join(tmp, "scanner.db"))
	require.NoError(t, err)
	server := lsp.NewServer(fs, tmp, "test")
	server.RegisterIndexer(extIdx, nil)

	provider := codeaction.NewTwigCodeActionProvider(aida, server)
	params := &protocol.CodeActionParams{
		Node:            node,
		DocumentContent: content,
	}
	params.TextDocument.URI = "file://" + vendorPath

	actions := provider.GetCodeActions(t.Context(), params)
	t.Logf("actions count: %d", len(actions))
	for _, action := range actions {
		t.Logf("  - %s", action.Title)
	}

	var extendAction *protocol.CodeAction
	for _, action := range actions {
		if action.Title == "Extend block '"+blockName+"' in WbmAidaCore" {
			extendAction = &action
			break
		}
	}
	require.NotNil(t, extendAction, "expected extend action for %s", blockName)
}

func TestExtendBlockCodeAction_titleContentTextAlreadyExistsAida(t *testing.T) {
	aida := "/Users/mkopp/Projects/Customers/aida"
	vendorPath := filepath.Join(aida, "vendor/store.shopware.com/swagcustomizedproducts/src/Resources/views/storefront/component/customized-products/_include/title.html.twig")
	content, err := os.ReadFile(vendorPath)
	if err != nil {
		t.Skip("aida not available")
	}

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()
	tree := parser.Parse(content, nil)
	defer tree.Close()

	blockName := "swag_customized_products_option_type_template_label_content_text"
	node := treesitterhelper.FindIdentifierNode(tree.RootNode(), content, blockName)
	require.NotNil(t, node)

	tmp := t.TempDir()
	extIdx, err := extension.NewExtensionIndexer(tmp)
	require.NoError(t, err)
	phpPath := filepath.Join(aida, "src/WbmAidaCore/WbmAidaCore.php")
	phpContent, err := os.ReadFile(phpPath)
	require.NoError(t, err)
	phpParser := tree_sitter.NewParser()
	require.NoError(t, phpParser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())))
	defer phpParser.Close()
	phpTree := phpParser.Parse(phpContent, nil)
	defer phpTree.Close()
	require.NoError(t, extIdx.Index(phpPath, phpTree.RootNode(), phpContent))

	fs, err := indexer.NewFileScanner(aida, filepath.Join(tmp, "scanner.db"))
	require.NoError(t, err)
	server := lsp.NewServer(fs, tmp, "test")
	server.RegisterIndexer(extIdx, nil)
	provider := codeaction.NewTwigCodeActionProvider(aida, server)

	params := &protocol.CodeActionParams{
		Node:            node,
		DocumentContent: content,
	}
	params.TextDocument.URI = "file://" + vendorPath
	params.Range.Start.Line = 16

	actions := provider.GetCodeActions(t.Context(), params)
	require.NotEmpty(t, actions, "expected extend action for parent label_content block")

	var extendAction *protocol.CodeAction
	for _, action := range actions {
		if action.Title == "Extend block 'swag_customized_products_option_type_template_label_content' in WbmAidaCore" {
			extendAction = &action
			break
		}
	}
	require.NotNil(t, extendAction, "expected parent block extend action")
	require.NotNil(t, extendAction.Edit)
	require.NotEmpty(t, extendAction.Edit.DocumentChanges)
	require.NotNil(t, extendAction.Command)
	assert.Equal(t, lsp.FocusExtendedBlockCommand, extendAction.Command.Command)
	assert.Contains(t, extendAction.Edit.DocumentChanges[0].Edits[0].NewText, "shopware-block:")
}
