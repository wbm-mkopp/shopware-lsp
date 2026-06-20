package codeaction

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/shopware/shopware-lsp/internal/extension"
	"github.com/shopware/shopware-lsp/internal/indexer"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	"github.com/shopware/shopware-lsp/internal/twig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
)

func indexPluginBundleForTest(t *testing.T, extIndexer *extension.ExtensionIndexer, pluginPath string) {
	content, err := os.ReadFile(pluginPath)
	require.NoError(t, err)

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())))
	defer parser.Close()

	tree := parser.Parse(content, nil)
	defer tree.Close()

	require.NoError(t, extIndexer.Index(pluginPath, tree.RootNode(), content))
}

func TestExtractLineIndent(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		line     int
		maxCol   int
		expected string
	}{
		{
			name:     "no indentation",
			content:  "{% block my_block %}",
			line:     0,
			maxCol:   0,
			expected: "",
		},
		{
			name:     "spaces indentation",
			content:  "        {% block my_block %}",
			line:     0,
			maxCol:   8,
			expected: "        ",
		},
		{
			name:     "tab indentation",
			content:  "\t\t{% block my_block %}",
			line:     0,
			maxCol:   2,
			expected: "\t\t",
		},
		{
			name:     "mixed tabs and spaces",
			content:  "\t    {% block my_block %}",
			line:     0,
			maxCol:   5,
			expected: "\t    ",
		},
		{
			name:     "block on second line",
			content:  "{% sw_extends '@Storefront/page.html.twig' %}\n        {% block my_block %}",
			line:     1,
			maxCol:   8,
			expected: "        ",
		},
		{
			name:     "block on third line with preceding lines",
			content:  "line one\nline two\n    {% block nested %}",
			line:     2,
			maxCol:   4,
			expected: "    ",
		},
		{
			name:     "deeply nested block",
			content:  "{% extends %}\n\n            {% block deep_block %}",
			line:     2,
			maxCol:   12,
			expected: "            ",
		},
		{
			name:     "maxCol is zero returns empty",
			content:  "{% block top_level %}",
			line:     0,
			maxCol:   0,
			expected: "",
		},
		{
			name:     "maxCol beyond content length is safe",
			content:  "  x",
			line:     0,
			maxCol:   100,
			expected: "  ",
		},
		{
			name:     "non-whitespace before maxCol stops early",
			content:  "  x  {% block foo %}",
			line:     0,
			maxCol:   5,
			expected: "  ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractLineIndent([]byte(tt.content), tt.line, tt.maxCol)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetVersioningHashActionUsesExtendsFallbackWhenBlockHashesMissing(t *testing.T) {
	tempDir := t.TempDir()

	indexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer indexer.Close()

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	vendorPath := filepath.Join(tempDir, "vendor/shopware/storefront/Resources/views/storefront/component/product/card/price-unit.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(vendorPath), 0755))
	vendorContent := []byte(`{% block component_product_box_price_info %}
    <div class="product-price-info">
        {% block component_product_box_price_unit %}
            <p class="product-price-unit">
                {% block component_product_box_price_purchase_unit %}
                    {% if referencePrice and referencePrice.unitName %}
                        <span class="product-unit-label"></span>
                    {% endif %}
                {% endblock %}
            </p>
        {% endblock %}
    </div>
{% endblock %}`)
	require.NoError(t, os.WriteFile(vendorPath, vendorContent, 0644))

	// Index only twig file metadata (no block hash entries), reproducing stale/missing hash index.
	require.NoError(t, indexer.IndexTwigFile(twig.TwigFile{
		Path:    vendorPath,
		RelPath: "@Storefront/storefront/component/product/card/price-unit.html.twig",
		Blocks:  map[string]twig.TwigBlock{},
	}))

	overridePath := filepath.Join(tempDir, "src/WbmAidaCore/Resources/views/storefront/component/product/card/price-unit.html.twig")
	overrideContent := []byte(`{% sw_extends '@Storefront/storefront/component/product/card/price-unit.html.twig' %}
{% block component_product_box_price_unit %}
    {% if referencePrice and referencePrice.unitName %}
        {{ parent() }}
    {% endif %}
{% endblock %}`)
	tree := parser.Parse(overrideContent, nil)
	defer tree.Close()

	node := treesitterhelper.FindIdentifierNode(tree.RootNode(), overrideContent, "component_product_box_price_unit")
	require.NotNil(t, node)

	params := &protocol.CodeActionParams{
		Node:            node,
		DocumentContent: overrideContent,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, overridePath)

	provider := &TwigCodeActionProvider{
		twigIndexer: indexer,
		projectRoot: tempDir,
	}

	actions := provider.GetCodeActions(context.Background(), params)

	var hasVersioningAction bool
	for _, action := range actions {
		if action.Title == "Add twig versioning hash" {
			hasVersioningAction = true
			require.NotNil(t, action.Edit)
			edits := action.Edit.Changes[params.TextDocument.URI]
			require.NotEmpty(t, edits)
			assert.Contains(t, edits[0].NewText, "shopware-block:")
			break
		}
	}

	assert.True(t, hasVersioningAction, "expected quick-fix to add twig versioning hash")
}

// Blocks whose body contains raw HTML are wrapped by tree-sitter in an ERROR
// node, so the identifier's parent is not "block". The versioning action must
// still be offered for them.
func TestGetVersioningHashActionForHTMLBlockWrappedInErrorNode(t *testing.T) {
	tempDir := t.TempDir()

	indexer, err := twig.NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer indexer.Close()

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	vendorPath := filepath.Join(tempDir, "vendor/shopware/storefront/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(vendorPath), 0755))
	vendorContent := []byte(`{% block buy_widget_delivery_informations %}
    <div class="product-detail-delivery-information"></div>
{% endblock %}`)
	require.NoError(t, os.WriteFile(vendorPath, vendorContent, 0644))

	require.NoError(t, indexer.IndexTwigFile(twig.TwigFile{
		Path:    vendorPath,
		RelPath: "@Storefront/storefront/component/buy-widget/buy-widget.html.twig",
		Blocks:  map[string]twig.TwigBlock{},
	}))

	overridePath := filepath.Join(tempDir, "src/WbmAidaCore/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	overrideContent := []byte(`{% sw_extends '@Storefront/storefront/component/buy-widget/buy-widget.html.twig' %}
{% block buy_widget_delivery_informations %}
    <div class="product-availability">
        {% if (product.stock > 0 and product.isCloseout == true) or product.isCloseout == false %}
            <span class="product-availability-available">
                {{ 'listing.boxAvailabilityAvailable'|trans }}
            </span>
        {% else %}
            <span class="product-availability-unavailable">
                {{ 'listing.boxAvailabilityUnavailable'|trans }}
            </span>
        {% endif %}
    </div>
{% endblock %}`)
	tree := parser.Parse(overrideContent, nil)
	defer tree.Close()

	node := treesitterhelper.FindIdentifierNode(tree.RootNode(), overrideContent, "buy_widget_delivery_informations")
	require.NotNil(t, node)
	// Guard: confirm this reproduces the ERROR-node case the fix targets.
	require.NotEqual(t, "block", node.Parent().Kind())

	params := &protocol.CodeActionParams{
		Node:            node,
		DocumentContent: overrideContent,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, overridePath)

	provider := &TwigCodeActionProvider{
		twigIndexer: indexer,
		projectRoot: tempDir,
	}

	actions := provider.GetCodeActions(context.Background(), params)

	var hasVersioningAction bool
	for _, action := range actions {
		if action.Title == "Add twig versioning hash" {
			hasVersioningAction = true
			require.NotNil(t, action.Edit)
			edits := action.Edit.Changes[params.TextDocument.URI]
			require.NotEmpty(t, edits)
			assert.Contains(t, edits[0].NewText, "shopware-block:")
			break
		}
	}

	assert.True(t, hasVersioningAction, "expected quick-fix to add twig versioning hash for HTML block")
}

func TestGetExtendBlockActionsForStorefrontAndStorePlugin(t *testing.T) {
	tempDir := t.TempDir()

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	pluginDir := filepath.Join(tempDir, "custom/plugins/WbmAidaCore")
	pluginPath := filepath.Join(pluginDir, "WbmAidaCore.php")
	require.NoError(t, os.MkdirAll(pluginDir, 0755))
	require.NoError(t, os.WriteFile(pluginPath, []byte(`<?php
namespace Wbm\AidaCore;

use Shopware\Core\Framework\Plugin;

class WbmAidaCore extends Plugin {}
`), 0644))

	fileScanner, err := indexer.NewFileScanner(tempDir, filepath.Join(tempDir, "scanner.db"))
	require.NoError(t, err)

	server := lsp.NewServer(fileScanner, tempDir, "test")
	extIndexer, err := extension.NewExtensionIndexer(tempDir)
	require.NoError(t, err)
	server.RegisterIndexer(extIndexer, nil)
	indexPluginBundleForTest(t, extIndexer, pluginPath)

	provider := NewTwigCodeActionProvider(tempDir, server)

	testCases := []struct {
		name     string
		twigPath string
		block    string
	}{
		{
			name:     "shopware storefront",
			twigPath: filepath.Join(tempDir, "vendor/shopware/storefront/Resources/views/storefront/component/buy-widget/buy-widget.html.twig"),
			block:    "buy_widget",
		},
		{
			name:     "store.shopware.com plugin",
			twigPath: filepath.Join(tempDir, "vendor/store.shopware.com/swagcustomizedproducts/src/Resources/views/storefront/component/buy-widget/buy-widget.html.twig"),
			block:    "buy_widget",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.NoError(t, os.MkdirAll(filepath.Dir(tc.twigPath), 0755))
			content := []byte("{% block " + tc.block + " %}content{% endblock %}")
			require.NoError(t, os.WriteFile(tc.twigPath, content, 0644))

			tree := parser.Parse(content, nil)
			defer tree.Close()

			node := treesitterhelper.FindIdentifierNode(tree.RootNode(), content, tc.block)
			require.NotNil(t, node)

			params := &protocol.CodeActionParams{
				Node:            node,
				DocumentContent: content,
			}
			params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, tc.twigPath)

			actions := provider.GetCodeActions(context.Background(), params)
			require.NotEmpty(t, actions)

			var extendAction *protocol.CodeAction
			for _, action := range actions {
				if action.Title == fmt.Sprintf("Extend block '%s' in WbmAidaCore", tc.block) {
					extendAction = &action
					break
				}
			}

			require.NotNil(t, extendAction, "expected extend block code action")
			require.NotNil(t, extendAction.Edit)
			require.NotEmpty(t, extendAction.Edit.DocumentChanges)
			require.NotNil(t, extendAction.Command)
			assert.Equal(t, lsp.FocusExtendedBlockCommand, extendAction.Command.Command)
			textEdits := extendAction.Edit.TextDocumentEdits()
			require.NotEmpty(t, textEdits)
			assert.Contains(t, textEdits[0].Edits[0].NewText, "{% block "+tc.block+" %}")
		})
	}
}

func TestGetExtendBlockActionsSkipsVendorExtensions(t *testing.T) {
	tempDir := t.TempDir()

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())))
	defer parser.Close()

	localDir := filepath.Join(tempDir, "custom/plugins/WbmAidaCore")
	localPath := filepath.Join(localDir, "WbmAidaCore.php")
	require.NoError(t, os.MkdirAll(localDir, 0755))
	require.NoError(t, os.WriteFile(localPath, []byte(`<?php
namespace Wbm\AidaCore;

use Shopware\Core\Framework\Plugin;

class WbmAidaCore extends Plugin {}
`), 0644))

	vendorDir := filepath.Join(tempDir, "vendor/store.shopware.com/VendorPlugin")
	vendorPath := filepath.Join(vendorDir, "src/VendorPlugin.php")
	require.NoError(t, os.MkdirAll(filepath.Dir(vendorPath), 0755))
	require.NoError(t, os.WriteFile(vendorPath, []byte(`<?php
namespace VendorPlugin;

use Shopware\Core\Framework\Plugin;

class VendorPlugin extends Plugin {}
`), 0644))

	fileScanner, err := indexer.NewFileScanner(tempDir, filepath.Join(tempDir, "scanner.db"))
	require.NoError(t, err)

	server := lsp.NewServer(fileScanner, tempDir, "test")
	extIndexer, err := extension.NewExtensionIndexer(tempDir)
	require.NoError(t, err)
	server.RegisterIndexer(extIndexer, nil)
	indexPluginBundleForTest(t, extIndexer, localPath)
	indexPluginBundleForTest(t, extIndexer, vendorPath)

	provider := NewTwigCodeActionProvider(tempDir, server)

	storefrontPath := filepath.Join(tempDir, "vendor/shopware/storefront/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(storefrontPath), 0755))
	content := []byte("{% block buy_widget %}content{% endblock %}")
	require.NoError(t, os.WriteFile(storefrontPath, content, 0644))

	tree := parser.Parse(content, nil)
	defer tree.Close()

	node := treesitterhelper.FindIdentifierNode(tree.RootNode(), content, "buy_widget")
	require.NotNil(t, node)

	params := &protocol.CodeActionParams{
		Node:            node,
		DocumentContent: content,
	}
	params.TextDocument.URI = fmt.Sprintf(lsp.FileURIFormat, storefrontPath)

	actions := provider.GetCodeActions(context.Background(), params)

	for _, action := range actions {
		assert.NotContains(t, action.Title, "VendorPlugin")
	}

	var hasLocalAction bool
	for _, action := range actions {
		if action.Title == "Extend block 'buy_widget' in WbmAidaCore" {
			hasLocalAction = true
			break
		}
	}
	assert.True(t, hasLocalAction)
}
