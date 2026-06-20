package twig

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shopware/shopware-lsp/internal/extension"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
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
	assert.Contains(t, string(plan.NewContent), "shopware-block:")
	assert.False(t, plan.FileExisted)
	// Cursor lands on the empty line inside the block body (line 6: five rows below sw_extends).
	assert.Equal(t, 6, plan.BlockLine)

	edit := plan.WorkspaceEdit()
	require.NotNil(t, edit)
	require.NotEmpty(t, edit.Changes[plan.URI])
	require.NotEmpty(t, edit.DocumentChanges)
	assert.Contains(t, edit.Changes[plan.URI][0].NewText, "{% block buy_widget %}")

	// A new override file must be created via a CreateFile resource operation
	// (ordered before the text edit), so edit-only clients actually create it.
	create, ok := edit.DocumentChanges[0].(protocol.CreateFile)
	require.True(t, ok, "first document change should be a CreateFile op")
	assert.Equal(t, "create", create.Kind)
	assert.Equal(t, plan.URI, create.URI)

	textEdits := edit.TextDocumentEdits()
	require.NotEmpty(t, textEdits)
	assert.Contains(t, textEdits[0].Edits[0].NewText, "{% block buy_widget %}")
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

func TestPlanExtendBlock_existingFile_cursorInsideBlockBody(t *testing.T) {
	tempDir := t.TempDir()

	storefrontPath := filepath.Join(tempDir, "vendor/shopware/storefront/Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(storefrontPath), 0755))
	require.NoError(t, os.WriteFile(storefrontPath, []byte("{% block buy_widget %}core{% endblock %}"), 0644))

	pluginDir := filepath.Join(tempDir, "custom/plugins/WbmAidaCore")
	ext := extension.ShopwareExtension{
		Name: "WbmAidaCore",
		Path: filepath.Join(pluginDir, "WbmAidaCore.php"),
	}

	overridePath := filepath.Join(pluginDir, "Resources/views/storefront/component/buy-widget/buy-widget.html.twig")
	require.NoError(t, os.MkdirAll(filepath.Dir(overridePath), 0755))
	require.NoError(t, os.WriteFile(overridePath, []byte("{% block buy_widget_wishlist %}{% endblock %}\n"), 0644))

	plan, err := PlanExtendBlock(tempDir, nil, "file://"+storefrontPath, "buy_widget", ext)
	require.Nil(t, err)
	require.NotNil(t, plan)
	assert.True(t, plan.FileExisted)
	assert.Equal(t, 6, plan.BlockLine)
}

func TestEndOfFileRange(t *testing.T) {
	end := endOfFileRange([]byte("line one\nline two"))
	assert.Equal(t, 1, end.Start.Line)
	assert.Equal(t, 8, end.Start.Character)
}
