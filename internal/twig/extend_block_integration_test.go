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
	"github.com/shopware/shopware-lsp/internal/twig"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

func TestExtendBlockIntegration_aidaTitlePartial(t *testing.T) {
	aida := "/Users/mkopp/Projects/Customers/aida"
	if _, err := os.Stat(aida); err != nil {
		t.Skip("aida project not available")
	}

	vendorPath := filepath.Join(aida, "vendor/store.shopware.com/swagcustomizedproducts/src/Resources/views/storefront/component/customized-products/_include/title.html.twig")
	content, err := os.ReadFile(vendorPath)
	require.NoError(t, err)

	uri := "file://" + vendorPath
	blockName := "swag_customized_products_option_type_template_label_container"

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()
	tree := parser.Parse(content, nil)
	defer tree.Close()

	node := treesitterhelper.FindIdentifierNode(tree.RootNode(), content, blockName)
	require.NotNil(t, node)

	name, ok := twig.BlockNameAtNode(node, content)
	require.True(t, ok)
	assert.Equal(t, blockName, name)

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

	exts, err := extIdx.GetAll()
	require.NoError(t, err)
	require.NotEmpty(t, exts)

	var wbmAida *extension.ShopwareExtension
	for _, ext := range exts {
		if ext.Name == "WbmAidaCore" {
			wbmAida = &ext
			break
		}
	}
	require.NotNil(t, wbmAida, "WbmAidaCore should be indexed")
	assert.True(t, wbmAida.IsLocal())

	plan, planErr := twig.PlanExtendBlock(aida, nil, uri, blockName, *wbmAida)
	require.Nil(t, planErr)
	require.NotNil(t, plan)
	assert.Contains(t, plan.Path, "WbmAidaCore/Resources/views/storefront/component/customized-products/_include/title.html.twig")

	fs, err := indexer.NewFileScanner(aida, filepath.Join(tmp, "scanner.db"))
	require.NoError(t, err)
	server := lsp.NewServer(fs, tmp, "test")
	server.RegisterIndexer(extIdx, nil)

	provider := codeaction.NewTwigCodeActionProvider(aida, server)
	params := &protocol.CodeActionParams{
		Node:            node,
		DocumentContent: content,
	}
	params.TextDocument.URI = uri

	actions := provider.GetCodeActions(t.Context(), params)
	require.NotEmpty(t, actions, "expected extend block code actions")

	var extendAction *protocol.CodeAction
	for _, action := range actions {
		if action.Title == "Extend block '"+blockName+"' in WbmAidaCore" {
			extendAction = &action
			break
		}
	}
	require.NotNil(t, extendAction, "expected WbmAidaCore extend action")
	require.NotNil(t, extendAction.Edit)
	require.NotEmpty(t, extendAction.Edit.DocumentChanges)
	require.NotNil(t, extendAction.Command)
	assert.Equal(t, lsp.FocusExtendedBlockCommand, extendAction.Command.Command)
	textEdits := extendAction.Edit.TextDocumentEdits()
	require.NotEmpty(t, textEdits)
	assert.Contains(t, textEdits[0].Edits[0].NewText, "{% block "+blockName+" %}")
}
