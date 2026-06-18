package twig

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/shopware/shopware-lsp/internal/extension"
	"github.com/shopware/shopware-lsp/internal/indexer"
	"github.com/shopware/shopware-lsp/internal/lsp"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func indexPluginBundle(t *testing.T, extIndexer *extension.ExtensionIndexer, pluginPath string) {
	content, err := os.ReadFile(pluginPath)
	require.NoError(t, err)

	parser := tree_sitter.NewParser()
	require.NoError(t, parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_php.LanguagePHP())))
	defer parser.Close()

	tree := parser.Parse(content, nil)
	defer tree.Close()

	require.NoError(t, extIndexer.Index(pluginPath, tree.RootNode(), content))
}

func TestTwigCommandProvider_extendBlock_storefrontSource(t *testing.T) {
	tempDir := t.TempDir()

	storefrontPath := filepath.Join(tempDir, "vendor/shopware/storefront/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(storefrontPath), 0755))
	require.NoError(t, os.WriteFile(storefrontPath, []byte("{% block buy_widget %}core{% endblock %}"), 0644))

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
	twigIndexer, err := NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()
	server.RegisterIndexer(twigIndexer, nil)
	indexPluginBundle(t, extIndexer, pluginPath)

	provider := NewTwigCommandProvider(tempDir, server)

	args, err := json.Marshal(map[string]string{
		"textUri":   "file://" + storefrontPath,
		"blockName": "buy_widget",
		"extension": "WbmAidaCore",
	})
	require.NoError(t, err)

	raw := json.RawMessage(args)
	result, err := provider.extendBlock(context.Background(), &raw)
	require.NoError(t, err)

	success, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, success["uri"], "custom/plugins/WbmAidaCore/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")

	overridePath := filepath.Join(pluginDir, "Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	content, err := os.ReadFile(overridePath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "{% sw_extends \"@Storefront/storefront/component/buy-widget/buy-widget.html.twig\" %}")
	assert.Contains(t, string(content), "{% block buy_widget %}")
}

func TestTwigCommandProvider_extendBlock_storePluginSource(t *testing.T) {
	tempDir := t.TempDir()

	pluginRoot := filepath.Join(tempDir, "vendor/store.shopware.com/swagcustomizedproducts")
	pluginTwigPath := filepath.Join(pluginRoot, "src/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(pluginTwigPath), 0755))
	require.NoError(t, os.WriteFile(pluginTwigPath, []byte("{% block buy_widget %}plugin{% endblock %}"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(pluginRoot, "composer.json"), []byte(`{
		"extra": { "shopware-plugin-class": "Swag\\CustomizedProducts\\SwagCustomizedProducts" }
	}`), 0644))

	overrideDir := filepath.Join(tempDir, "custom/plugins/WbmAidaCore")
	overridePath := filepath.Join(overrideDir, "WbmAidaCore.php")
	require.NoError(t, os.MkdirAll(overrideDir, 0755))
	require.NoError(t, os.WriteFile(overridePath, []byte(`<?php
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
	twigIndexer, err := NewTwigIndexer(tempDir)
	require.NoError(t, err)
	defer twigIndexer.Close()
	server.RegisterIndexer(twigIndexer, nil)
	indexPluginBundle(t, extIndexer, overridePath)

	provider := NewTwigCommandProvider(tempDir, server)

	args, err := json.Marshal(map[string]string{
		"textUri":   "file://" + pluginTwigPath,
		"blockName": "buy_widget",
		"extension": "WbmAidaCore",
	})
	require.NoError(t, err)

	raw := json.RawMessage(args)
	result, err := provider.extendBlock(context.Background(), &raw)
	require.NoError(t, err)

	success, ok := result.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, success["uri"], "custom/plugins/WbmAidaCore/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")

	overrideTwigPath := filepath.Join(overrideDir, "Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	content, err := os.ReadFile(overrideTwigPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "{% sw_extends \"@SwagCustomizedProducts/storefront/component/buy-widget/buy-widget.html.twig\" %}")
	assert.Contains(t, string(content), "{% block buy_widget %}")
}
