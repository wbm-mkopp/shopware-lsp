package twig

import (
	"os"
	"path/filepath"
	"testing"

	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestBlockNamesAtCursor_buyWidgetAida(t *testing.T) {
	aida := "/Users/mkopp/Projects/Customers/aida"
	path := filepath.Join(aida, "vendor/shopware/storefront/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	content, err := os.ReadFile(path)
	if err != nil {
		t.Skip("aida not available")
	}

	ranges := findBlockLineRanges(content)
	t.Logf("parsed block ranges: %d", len(ranges))

	for _, line := range []int{0, 50, 100, 200} {
		names := BlockNamesAtCursor(content, line)
		t.Logf("line %d names: %v", line+1, names)
		require.NotEmpty(t, names, "expected blocks at line %d", line+1)
	}

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	tree := parser.Parse(content, nil)
	defer tree.Close()

	node := treesitterhelper.FindIdentifierNode(tree.RootNode(), content, "buy_widget")
	require.NotNil(t, node)

	candidates := ExtendBlockCandidates(node, content, int(node.StartPosition().Row))
	t.Logf("ExtendBlockCandidates on buy_widget identifier: %v", candidates)
	require.NotEmpty(t, candidates)
	require.Equal(t, "buy_widget", candidates[0])
}
