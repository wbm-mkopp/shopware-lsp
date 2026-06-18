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
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

func setupAidaCodeActionProvider(t *testing.T, aida string) *codeaction.TwigCodeActionProvider {
	t.Helper()
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
	return codeaction.NewTwigCodeActionProvider(aida, server)
}

func TestExtendCodeAction_buyWidgetAida(t *testing.T) {
	aida := "/Users/mkopp/Projects/Customers/aida"
	path := filepath.Join(aida, "vendor/shopware/storefront/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Skip("aida not available")
	}

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	tree := parser.Parse(content, nil)
	defer tree.Close()

	node := treesitterhelper.FindIdentifierNode(tree.RootNode(), content, "buy_widget")
	require.NotNil(t, node)

	provider := setupAidaCodeActionProvider(t, aida)
	params := &protocol.CodeActionParams{Node: node, DocumentContent: content}
	params.TextDocument.URI = "file://" + path
	params.Range.Start.Line = int(node.StartPosition().Row)

	actions := provider.GetCodeActions(t.Context(), params)
	require.NotEmpty(t, actions, "expected extend action for buy_widget")
}

func TestExtendCodeAction_titleContentTextAida(t *testing.T) {
	aida := "/Users/mkopp/Projects/Customers/aida"
	path := filepath.Join(aida, "vendor/store.shopware.com/swagcustomizedproducts/src/Resources/views/storefront/component/customized-products/_include/title.html.twig")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Skip("aida not available")
	}

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	tree := parser.Parse(content, nil)
	defer tree.Close()

	blockName := "swag_customized_products_option_type_template_label_content_text"
	node := treesitterhelper.FindIdentifierNode(tree.RootNode(), content, blockName)
	require.NotNil(t, node)

	provider := setupAidaCodeActionProvider(t, aida)
	params := &protocol.CodeActionParams{Node: node, DocumentContent: content}
	params.TextDocument.URI = "file://" + path
	params.Range.Start.Line = int(node.StartPosition().Row)

	actions := provider.GetCodeActions(t.Context(), params)
	require.NotEmpty(t, actions, "expected fallback extend action for parent label_content")
}

func TestExtendCodeAction_nilNodeWithLineAida(t *testing.T) {
	aida := "/Users/mkopp/Projects/Customers/aida"
	path := filepath.Join(aida, "vendor/shopware/storefront/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Skip("aida not available")
	}

	provider := setupAidaCodeActionProvider(t, aida)
	params := &protocol.CodeActionParams{DocumentContent: content}
	params.TextDocument.URI = "file://" + path
	params.Range.Start.Line = 0

	actions := provider.GetCodeActions(t.Context(), params)
	require.NotEmpty(t, actions, "expected extend action without AST node")
}
