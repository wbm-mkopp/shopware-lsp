package diagnostics

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/shopware/shopware-lsp/internal/admin"
	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

func parseJS(t *testing.T, code string) (*tree_sitter.Tree, *tree_sitter.Parser) {
	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse([]byte(code), nil)
	return tree, parser
}

func TestAdminDiagnosticsProvider_UndefinedParent(t *testing.T) {
	tempDir := t.TempDir()

	// Create the admin indexer
	adminIndexer, err := admin.NewAdminComponentIndexer(tempDir)
	require.NoError(t, err)
	defer func() { _ = adminIndexer.Close() }()

	provider := &AdminDiagnosticsProvider{
		adminIndexer: adminIndexer,
	}

	tests := []struct {
		name            string
		code            string
		uri             string
		expectDiagCount int
		expectMessage   string
	}{
		{
			name:            "undefined parent component",
			code:            `Component.extend('my-component', 'sw-undefined-parent', () => import('./index'));`,
			uri:             "file:///project/src/Resources/app/administration/src/main.js",
			expectDiagCount: 1,
			expectMessage:   "Parent component 'sw-undefined-parent' is not registered",
		},
		{
			name:            "Component.register should not warn",
			code:            `Component.register('my-component', () => import('./index'));`,
			uri:             "file:///project/src/Resources/app/administration/src/main.js",
			expectDiagCount: 0,
		},
		{
			name:            "Shopware.Component.extend with undefined parent",
			code:            `Shopware.Component.extend('my-component', 'sw-missing', () => import('./index'));`,
			uri:             "file:///project/src/Resources/app/administration/src/main.js",
			expectDiagCount: 1,
			expectMessage:   "Parent component 'sw-missing' is not registered",
		},
		{
			name:            "non-administration file should be ignored",
			code:            `Component.extend('my-component', 'sw-undefined-parent', () => import('./index'));`,
			uri:             "file:///project/src/other/main.js",
			expectDiagCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseJS(t, tt.code)
			defer tree.Close()
			defer parser.Close()

			diagnostics, err := provider.GetDiagnostics(context.Background(), tt.uri, tree.RootNode(), []byte(tt.code))
			require.NoError(t, err)

			assert.Len(t, diagnostics, tt.expectDiagCount)

			if tt.expectDiagCount > 0 && tt.expectMessage != "" {
				assert.Equal(t, tt.expectMessage, diagnostics[0].Message)
			}
		})
	}
}

func TestAdminDiagnosticsProvider_DefinedParent(t *testing.T) {
	tempDir := t.TempDir()

	// Create the admin indexer
	adminIndexer, err := admin.NewAdminComponentIndexer(tempDir)
	require.NoError(t, err)
	defer func() { _ = adminIndexer.Close() }()

	// First, index a parent component
	parentCode := `Component.register('sw-button', () => import('./index'));`
	parentTree, parentParser := parseJS(t, parentCode)
	defer parentTree.Close()
	defer parentParser.Close()

	parentFilePath := filepath.Join(tempDir, "src", "Resources", "app", "administration", "src", "component", "sw-button", "index.js")
	err = adminIndexer.Index(parentFilePath, parentTree.RootNode(), []byte(parentCode))
	require.NoError(t, err)

	provider := &AdminDiagnosticsProvider{
		adminIndexer: adminIndexer,
	}

	// Now test extending the registered component - should not produce diagnostics
	code := `Component.extend('my-button', 'sw-button', () => import('./index'));`
	tree, parser := parseJS(t, code)
	defer tree.Close()
	defer parser.Close()

	uri := "file:///project/src/Resources/app/administration/src/main.js"
	diagnostics, err := provider.GetDiagnostics(context.Background(), uri, tree.RootNode(), []byte(code))
	require.NoError(t, err)

	assert.Empty(t, diagnostics, "Should not produce diagnostics when parent component is registered")
}

func TestExtractStringContent(t *testing.T) {
	tests := []struct {
		name     string
		code     string
		expected string
	}{
		{
			name:     "single quoted string",
			code:     `'hello'`,
			expected: "hello",
		},
		{
			name:     "double quoted string",
			code:     `"world"`,
			expected: "world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseJS(t, tt.code)
			defer tree.Close()
			defer parser.Close()

			// The root will be a program, we need to find the string node
			var stringNode *tree_sitter.Node
			var findString func(node *tree_sitter.Node)
			findString = func(node *tree_sitter.Node) {
				if node.Kind() == "string" {
					stringNode = node
					return
				}
				for i := uint(0); i < node.ChildCount(); i++ {
					findString(node.Child(i))
				}
			}
			findString(tree.RootNode())

			require.NotNil(t, stringNode, "Should find string node")

			result := extractStringContent(stringNode, []byte(tt.code))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func parseTwig(t *testing.T, code string) (*tree_sitter.Tree, *tree_sitter.Parser) {
	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse([]byte(code), nil)
	return tree, parser
}

func TestAdminDiagnosticsProvider_MissingRequiredProps(t *testing.T) {
	tempDir := t.TempDir()

	// Create the admin indexer
	adminIndexer, err := admin.NewAdminComponentIndexer(tempDir)
	require.NoError(t, err)
	defer func() { _ = adminIndexer.Close() }()

	// Index a component with required props using inline definition in register call
	jsParser := tree_sitter.NewParser()
	require.NoError(t, jsParser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())))
	defer jsParser.Close()

	// Register component with inline definition that has required props
	compCode := `
Component.register('sw-button', {
	props: {
		label: {
			type: String,
			required: true,
		},
		disabled: {
			type: Boolean,
			required: false,
		},
		variant: {
			type: String,
			required: true,
		},
	},
});
`
	compTree := jsParser.Parse([]byte(compCode), nil)
	defer compTree.Close()

	compFilePath := filepath.Join(tempDir, "src", "Resources", "app", "administration", "src", "component", "sw-button", "index.js")
	err = adminIndexer.Index(compFilePath, compTree.RootNode(), []byte(compCode))
	require.NoError(t, err)

	// Verify component was indexed correctly
	comps, err := adminIndexer.GetComponentWithDefinition("sw-button")
	require.NoError(t, err)
	require.Len(t, comps, 1, "Component should be indexed")
	require.Len(t, comps[0].Props, 3, "Component should have 3 props")

	provider := &AdminDiagnosticsProvider{
		adminIndexer: adminIndexer,
	}

	tests := []struct {
		name            string
		twigCode        string
		expectDiagCount int
		expectProps     []string // props that should be reported as missing
	}{
		{
			name:            "missing all required props",
			twigCode:        `<sw-button></sw-button>`,
			expectDiagCount: 2,
			expectProps:     []string{"label", "variant"},
		},
		{
			name:            "missing one required prop",
			twigCode:        `<sw-button label="Click me"></sw-button>`,
			expectDiagCount: 1,
			expectProps:     []string{"variant"},
		},
		{
			name:            "all required props present",
			twigCode:        `<sw-button label="Click me" variant="primary"></sw-button>`,
			expectDiagCount: 0,
		},
		{
			name:            "required prop with Vue binding",
			twigCode:        `<sw-button :label="buttonLabel" variant="primary"></sw-button>`,
			expectDiagCount: 0,
		},
		{
			name:            "required prop with v-bind",
			twigCode:        `<sw-button v-bind:label="buttonLabel" variant="primary"></sw-button>`,
			expectDiagCount: 0,
		},
		{
			name:            "non-required prop missing is ok",
			twigCode:        `<sw-button label="Click" variant="primary"></sw-button>`,
			expectDiagCount: 0,
		},
		{
			name:            "unknown component ignored",
			twigCode:        `<sw-unknown required-prop="value"></sw-unknown>`,
			expectDiagCount: 0,
		},
		{
			name:            "standard HTML elements ignored",
			twigCode:        `<div class="test"></div>`,
			expectDiagCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseTwig(t, tt.twigCode)
			defer tree.Close()
			defer parser.Close()

			uri := "file:///project/src/Resources/app/administration/src/views/test.html.twig"
			diagnostics, err := provider.GetDiagnostics(context.Background(), uri, tree.RootNode(), []byte(tt.twigCode))
			require.NoError(t, err)

			assert.Len(t, diagnostics, tt.expectDiagCount, "Unexpected number of diagnostics")

			// Check that the expected props are reported
			for _, expectedProp := range tt.expectProps {
				found := false
				for _, diag := range diagnostics {
					if data, ok := diag.Data.(map[string]any); ok {
						if data["propName"] == expectedProp {
							found = true
							break
						}
					}
				}
				assert.True(t, found, "Expected diagnostic for missing prop '%s'", expectedProp)
			}
		})
	}
}

func TestAdminDiagnosticsProvider_KebabCaseProps(t *testing.T) {
	tempDir := t.TempDir()

	// Create the admin indexer
	adminIndexer, err := admin.NewAdminComponentIndexer(tempDir)
	require.NoError(t, err)
	defer func() { _ = adminIndexer.Close() }()

	// Index a component with camelCase required props
	jsParser := tree_sitter.NewParser()
	require.NoError(t, jsParser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())))
	defer jsParser.Close()

	compCode := `
Component.register('mt-card', {
	props: {
		positionIdentifier: {
			type: String,
			required: true,
		},
	},
});
`
	compTree := jsParser.Parse([]byte(compCode), nil)
	defer compTree.Close()

	compFilePath := filepath.Join(tempDir, "src", "Resources", "app", "administration", "src", "component", "mt-card", "index.js")
	err = adminIndexer.Index(compFilePath, compTree.RootNode(), []byte(compCode))
	require.NoError(t, err)

	provider := &AdminDiagnosticsProvider{
		adminIndexer: adminIndexer,
	}

	tests := []struct {
		name            string
		twigCode        string
		expectDiagCount int
	}{
		{
			name:            "camelCase prop in template",
			twigCode:        `<mt-card positionIdentifier="test"></mt-card>`,
			expectDiagCount: 0,
		},
		{
			name:            "kebab-case prop in template",
			twigCode:        `<mt-card position-identifier="test"></mt-card>`,
			expectDiagCount: 0,
		},
		{
			name:            "kebab-case with Vue binding",
			twigCode:        `<mt-card :position-identifier="myVar"></mt-card>`,
			expectDiagCount: 0,
		},
		{
			name:            "missing prop should warn",
			twigCode:        `<mt-card></mt-card>`,
			expectDiagCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseTwig(t, tt.twigCode)
			defer tree.Close()
			defer parser.Close()

			uri := "file:///project/src/Resources/app/administration/src/views/test.html.twig"
			diagnostics, err := provider.GetDiagnostics(context.Background(), uri, tree.RootNode(), []byte(tt.twigCode))
			require.NoError(t, err)

			assert.Len(t, diagnostics, tt.expectDiagCount, "Unexpected number of diagnostics")
		})
	}
}

func TestCamelToKebab(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"positionIdentifier", "position-identifier"},
		{"myPropName", "my-prop-name"},
		{"simple", "simple"},
		{"ABC", "a-b-c"},
		{"camelCase", "camel-case"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := camelToKebab(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAdminDiagnosticsProvider_BlockReferences(t *testing.T) {
	tempDir := t.TempDir()

	// Create the admin indexer
	adminIndexer, err := admin.NewAdminComponentIndexer(tempDir)
	require.NoError(t, err)
	defer func() { _ = adminIndexer.Close() }()

	// Setup paths
	parentCompPath := filepath.Join(tempDir, "src", "Resources", "app", "administration", "src", "component", "sw-page", "index.js")
	parentTemplatePath := filepath.Join(tempDir, "src", "Resources", "app", "administration", "src", "component", "sw-page", "sw-page.html.twig")
	extendCompPath := filepath.Join(tempDir, "src", "Resources", "app", "administration", "src", "extension", "my-custom-page", "index.js")
	extendTemplatePath := filepath.Join(tempDir, "src", "Resources", "app", "administration", "src", "extension", "my-custom-page", "my-custom-page.html.twig")

	// Save parent component directly with blocks already populated
	parentComp := admin.VueComponent{
		Name:         "sw-page",
		FilePath:     parentCompPath,
		TemplatePath: parentTemplatePath,
		Blocks: []admin.TwigBlock{
			{Name: "sw_page_content", Line: 10},
			{Name: "sw_page_smart_bar", Line: 20},
			{Name: "sw_page_actions", Line: 30},
		},
	}
	err = adminIndexer.SaveComponent(parentComp)
	require.NoError(t, err)

	// Save extended component that extends sw-page
	extendComp := admin.VueComponent{
		Name:             "my-custom-page",
		FilePath:         extendCompPath,
		TemplatePath:     extendTemplatePath,
		ExtendsComponent: "sw-page",
	}
	err = adminIndexer.SaveComponent(extendComp)
	require.NoError(t, err)

	provider := &AdminDiagnosticsProvider{
		adminIndexer: adminIndexer,
	}

	tests := []struct {
		name             string
		twigCode         string
		expectDiagCount  int
		expectBlockNames []string // block names that should be reported as invalid
	}{
		{
			name:            "valid block reference",
			twigCode:        `{% block sw_page_content %}<div>Custom content</div>{% endblock %}`,
			expectDiagCount: 0,
		},
		{
			name:             "invalid block reference",
			twigCode:         `{% block sw_nonexistent_block %}<div>Custom content</div>{% endblock %}`,
			expectDiagCount:  1,
			expectBlockNames: []string{"sw_nonexistent_block"},
		},
		{
			name:            "multiple valid blocks",
			twigCode:        `{% block sw_page_content %}content{% endblock %}{% block sw_page_smart_bar %}bar{% endblock %}`,
			expectDiagCount: 0,
		},
		{
			name:             "mixed valid and invalid blocks",
			twigCode:         `{% block sw_page_content %}content{% endblock %}{% block invalid_block %}invalid{% endblock %}`,
			expectDiagCount:  1,
			expectBlockNames: []string{"invalid_block"},
		},
		{
			name:             "multiple invalid blocks",
			twigCode:         `{% block invalid_one %}one{% endblock %}{% block invalid_two %}two{% endblock %}`,
			expectDiagCount:  2,
			expectBlockNames: []string{"invalid_one", "invalid_two"},
		},
	}

	// Verify setup: GetComponentByTemplatePath should find the extended component
	compByPath, err := adminIndexer.GetComponentByTemplatePath(extendTemplatePath)
	require.NoError(t, err)
	require.NotNil(t, compByPath, "GetComponentByTemplatePath should find the component")
	require.Equal(t, "my-custom-page", compByPath.Name)
	require.Equal(t, "sw-page", compByPath.ExtendsComponent)

	// Verify parent has blocks
	parentComps, err := adminIndexer.GetComponent("sw-page")
	require.NoError(t, err)
	require.Len(t, parentComps, 1, "Parent component should be found")
	require.Len(t, parentComps[0].Blocks, 3, "Parent component should have 3 blocks")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseTwig(t, tt.twigCode)
			defer tree.Close()
			defer parser.Close()

			// Use the extended component template path as URI
			uri := "file://" + extendTemplatePath
			diagnostics, err := provider.GetDiagnostics(context.Background(), uri, tree.RootNode(), []byte(tt.twigCode))
			require.NoError(t, err)

			assert.Len(t, diagnostics, tt.expectDiagCount, "Unexpected number of diagnostics")

			// Check that the expected blocks are reported
			for _, expectedBlock := range tt.expectBlockNames {
				found := false
				for _, diag := range diagnostics {
					if data, ok := diag.Data.(map[string]any); ok {
						if data["blockName"] == expectedBlock {
							found = true
							assert.Equal(t, "admin.component.block-not-found", diag.Code)
							break
						}
					}
				}
				assert.True(t, found, "Expected diagnostic for invalid block '%s'", expectedBlock)
			}
		})
	}
}

func TestAdminDiagnosticsProvider_BlockReferences_NoOverride(t *testing.T) {
	tempDir := t.TempDir()

	// Create the admin indexer
	adminIndexer, err := admin.NewAdminComponentIndexer(tempDir)
	require.NoError(t, err)
	defer func() { _ = adminIndexer.Close() }()

	// Setup paths
	compPath := filepath.Join(tempDir, "src", "Resources", "app", "administration", "src", "component", "my-component", "index.js")
	templatePath := filepath.Join(tempDir, "src", "Resources", "app", "administration", "src", "component", "my-component", "my-component.html.twig")

	// Save a regular component (not an override - no ExtendsComponent)
	comp := admin.VueComponent{
		Name:         "my-component",
		FilePath:     compPath,
		TemplatePath: templatePath,
		// Note: ExtendsComponent is empty - this is a regular component
	}
	err = adminIndexer.SaveComponent(comp)
	require.NoError(t, err)

	provider := &AdminDiagnosticsProvider{
		adminIndexer: adminIndexer,
	}

	// Regular components (not extending others) should not produce block diagnostics
	// because they define their own blocks
	twigCode := `{% block any_block_name %}content{% endblock %}`

	tree, parser := parseTwig(t, twigCode)
	defer tree.Close()
	defer parser.Close()

	uri := "file://" + templatePath
	diagnostics, err := provider.GetDiagnostics(context.Background(), uri, tree.RootNode(), []byte(twigCode))
	require.NoError(t, err)

	assert.Empty(t, diagnostics, "Regular components should not produce block reference diagnostics")
}
