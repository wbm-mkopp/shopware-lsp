package twig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shopware/shopware-lsp/internal/extension"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanExtendBlock_newFile(t *testing.T) {
	tempDir := t.TempDir()

	storefrontPath := filepath.Join(tempDir, "vendor/shopware/storefront/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(storefrontPath), 0755))
	require.NoError(t, os.WriteFile(storefrontPath, []byte("{% block buy_widget %}core{% endblock %}"), 0644))

	pluginDir := filepath.Join(tempDir, "custom/plugins/WbmAidaCore")
	ext := extension.ShopwareExtension{
		Name: "WbmAidaCore",
		Path: filepath.Join(pluginDir, "WbmAidaCore.php"),
	}

	plan, err := PlanExtendBlock(tempDir, nil, "file://"+storefrontPath, "buy_widget", ext)
	require.Nil(t, err)
	require.NotNil(t, plan)

	assert.Contains(t, plan.Path, "custom/plugins/WbmAidaCore/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	assert.Contains(t, string(plan.NewContent), "{% sw_extends \"@Storefront/storefront/component/buy-widget/buy-widget.html.twig\" %}")
	assert.Contains(t, string(plan.NewContent), "{% block buy_widget %}")
	assert.False(t, plan.FileExisted)
	assert.Greater(t, plan.BlockLine, 0)

	edit := plan.WorkspaceEdit()
	require.NotNil(t, edit)
	require.NotEmpty(t, edit.DocumentChanges)
	assert.Contains(t, edit.DocumentChanges[0].Edits[0].NewText, "{% block buy_widget %}")
}

func TestPlanExtendBlock_storePluginSource(t *testing.T) {
	tempDir := t.TempDir()

	pluginTwigPath := filepath.Join(tempDir, "vendor/store.shopware.com/swagcustomizedproducts/src/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(pluginTwigPath), 0755))
	require.NoError(t, os.WriteFile(pluginTwigPath, []byte("{% block buy_widget %}plugin{% endblock %}"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "vendor/store.shopware.com/swagcustomizedproducts/composer.json"), []byte(`{
		"extra": { "shopware-plugin-class": "Swag\\CustomizedProducts\\SwagCustomizedProducts" }
	}`), 0644))

	pluginDir := filepath.Join(tempDir, "custom/plugins/WbmAidaCore")
	ext := extension.ShopwareExtension{
		Name: "WbmAidaCore",
		Path: filepath.Join(pluginDir, "WbmAidaCore.php"),
	}

	plan, planErr := PlanExtendBlock(tempDir, nil, "file://"+pluginTwigPath, "buy_widget", ext)
	require.Nil(t, planErr)
	require.NotNil(t, plan)

	assert.Contains(t, string(plan.NewContent), "{% sw_extends \"@SwagCustomizedProducts/storefront/component/buy-widget/buy-widget.html.twig\" %}")
}

func TestEndOfFileRange(t *testing.T) {
	end := endOfFileRange([]byte("line one\nline two"))
	assert.Equal(t, 1, end.Start.Line)
	assert.Equal(t, 8, end.Start.Character)
}
