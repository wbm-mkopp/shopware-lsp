package codeaction

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/shopware/shopware-lsp/internal/admin"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
)

func parseTwig(t *testing.T, code string) (*tree_sitter.Tree, *tree_sitter.Parser) {
	parser := tree_sitter.NewParser()
	if err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_twig.Language())); err != nil {
		t.Fatal(err)
	}
	tree := parser.Parse([]byte(code), nil)
	return tree, parser
}

// findNodeAtPosition finds the most specific node at the given position (simulating what the server does)
func findNodeAtPosition(root *tree_sitter.Node, line, character int) *tree_sitter.Node {
	pos := tree_sitter.Point{Row: uint(line), Column: uint(character)}
	return root.NamedDescendantForPointRange(pos, pos)
}

func TestAdminCodeActionProvider_AddMissingProp(t *testing.T) {
	tempDir := t.TempDir()

	// Create the admin indexer
	adminIndexer, err := admin.NewAdminComponentIndexer(tempDir)
	require.NoError(t, err)
	defer func() { _ = adminIndexer.Close() }()

	// Index a component with required props
	jsParser := tree_sitter.NewParser()
	require.NoError(t, jsParser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())))
	defer jsParser.Close()

	compCode := `
Component.register('sw-button', {
	props: {
		label: {
			type: String,
			required: true,
		},
		disabled: {
			type: Boolean,
			required: true,
		},
		count: {
			type: Number,
			required: true,
		},
		items: {
			type: Array,
			required: true,
		},
		config: {
			type: Object,
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

	provider := &AdminCodeActionProvider{
		adminIndexer: adminIndexer,
	}

	tests := []struct {
		name           string
		twigCode       string
		diagnostic     protocol.Diagnostic
		expectAction   bool
		expectPropAttr string // expected prop attribute in the edit
		expectValue    string // expected value in the edit
	}{
		{
			name:     "add string prop",
			twigCode: `<sw-button></sw-button>`,
			diagnostic: protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 1},
					End:   protocol.Position{Line: 0, Character: 10},
				},
				Code: "admin.component.missing-required-prop",
				Data: map[string]any{
					"componentName": "sw-button",
					"propName":      "label",
				},
			},
			expectAction:   true,
			expectPropAttr: "label",
			expectValue:    "",
		},
		{
			name:     "add boolean prop",
			twigCode: `<sw-button></sw-button>`,
			diagnostic: protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 1},
					End:   protocol.Position{Line: 0, Character: 10},
				},
				Code: "admin.component.missing-required-prop",
				Data: map[string]any{
					"componentName": "sw-button",
					"propName":      "disabled",
				},
			},
			expectAction:   true,
			expectPropAttr: ":disabled",
			expectValue:    "false",
		},
		{
			name:     "add number prop",
			twigCode: `<sw-button></sw-button>`,
			diagnostic: protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 1},
					End:   protocol.Position{Line: 0, Character: 10},
				},
				Code: "admin.component.missing-required-prop",
				Data: map[string]any{
					"componentName": "sw-button",
					"propName":      "count",
				},
			},
			expectAction:   true,
			expectPropAttr: ":count",
			expectValue:    "0",
		},
		{
			name:     "add array prop",
			twigCode: `<sw-button></sw-button>`,
			diagnostic: protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 1},
					End:   protocol.Position{Line: 0, Character: 10},
				},
				Code: "admin.component.missing-required-prop",
				Data: map[string]any{
					"componentName": "sw-button",
					"propName":      "items",
				},
			},
			expectAction:   true,
			expectPropAttr: ":items",
			expectValue:    "[]",
		},
		{
			name:     "add object prop",
			twigCode: `<sw-button></sw-button>`,
			diagnostic: protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 1},
					End:   protocol.Position{Line: 0, Character: 10},
				},
				Code: "admin.component.missing-required-prop",
				Data: map[string]any{
					"componentName": "sw-button",
					"propName":      "config",
				},
			},
			expectAction:   true,
			expectPropAttr: ":config",
			expectValue:    "{}",
		},
		{
			name:     "non-admin file ignored",
			twigCode: `<sw-button></sw-button>`,
			diagnostic: protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 1},
					End:   protocol.Position{Line: 0, Character: 10},
				},
				Code: "admin.component.missing-required-prop",
				Data: map[string]any{
					"componentName": "sw-button",
					"propName":      "label",
				},
			},
			expectAction: false, // Will use different URI
		},
		{
			name:     "wrong diagnostic code ignored",
			twigCode: `<sw-button></sw-button>`,
			diagnostic: protocol.Diagnostic{
				Range: protocol.Range{
					Start: protocol.Position{Line: 0, Character: 1},
					End:   protocol.Position{Line: 0, Character: 10},
				},
				Code: "some.other.code",
				Data: map[string]any{
					"componentName": "sw-button",
					"propName":      "label",
				},
			},
			expectAction: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseTwig(t, tt.twigCode)
			defer tree.Close()
			defer parser.Close()

			uri := "file:///project/src/Resources/app/administration/src/views/test.html.twig"
			if tt.name == "non-admin file ignored" {
				uri = "file:///project/src/other/test.html.twig"
			}

			// Find the node at the diagnostic position (simulating what the server does)
			nodeAtPos := findNodeAtPosition(tree.RootNode(), tt.diagnostic.Range.Start.Line, tt.diagnostic.Range.Start.Character)

			params := &protocol.CodeActionParams{
				TextDocument: struct {
					URI string `json:"uri"`
				}{URI: uri},
				Range: tt.diagnostic.Range,
				Context: protocol.CodeActionContext{
					Diagnostics: []protocol.Diagnostic{tt.diagnostic},
				},
				Node:            nodeAtPos,
				DocumentContent: []byte(tt.twigCode),
			}

			actions := provider.GetCodeActions(context.Background(), params)

			if !tt.expectAction {
				assert.Empty(t, actions)
				return
			}

			require.Len(t, actions, 1, "Should have one code action")
			action := actions[0]

			assert.Equal(t, protocol.CodeActionQuickFix, action.Kind)
			assert.Contains(t, action.Title, tt.diagnostic.Data.(map[string]any)["propName"])

			// Check the command (used for snippet insertion with cursor positioning)
			require.NotNil(t, action.Command)
			assert.Equal(t, "shopware.editor.insertSnippetAtPosition", action.Command.Command)
			require.Len(t, action.Command.Arguments, 4)

			// Arguments: [uri, line, character, snippetText]
			assert.Equal(t, uri, action.Command.Arguments[0])
			snippetText := action.Command.Arguments[3].(string)
			expectedSnippet := ` ` + tt.expectPropAttr + `="` + tt.expectValue + `$0"`
			assert.Equal(t, expectedSnippet, snippetText)
		})
	}
}

func TestAdminCodeActionProvider_CamelCaseToKebabCase(t *testing.T) {
	tempDir := t.TempDir()

	// Create the admin indexer
	adminIndexer, err := admin.NewAdminComponentIndexer(tempDir)
	require.NoError(t, err)
	defer func() { _ = adminIndexer.Close() }()

	// Index a component with camelCase props
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

	provider := &AdminCodeActionProvider{
		adminIndexer: adminIndexer,
	}

	twigCode := `<mt-card></mt-card>`
	tree, parser := parseTwig(t, twigCode)
	defer tree.Close()
	defer parser.Close()

	uri := "file:///project/src/Resources/app/administration/src/views/test.html.twig"

	// Find the node at the diagnostic position
	nodeAtPos := findNodeAtPosition(tree.RootNode(), 0, 1)

	params := &protocol.CodeActionParams{
		TextDocument: struct {
			URI string `json:"uri"`
		}{URI: uri},
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 1},
			End:   protocol.Position{Line: 0, Character: 8},
		},
		Context: protocol.CodeActionContext{
			Diagnostics: []protocol.Diagnostic{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 1},
						End:   protocol.Position{Line: 0, Character: 8},
					},
					Code: "admin.component.missing-required-prop",
					Data: map[string]any{
						"componentName": "mt-card",
						"propName":      "positionIdentifier",
					},
				},
			},
		},
		Node:            nodeAtPos,
		DocumentContent: []byte(twigCode),
	}

	actions := provider.GetCodeActions(context.Background(), params)
	require.Len(t, actions, 1)

	action := actions[0]
	require.NotNil(t, action.Command)
	assert.Equal(t, "shopware.editor.insertSnippetAtPosition", action.Command.Command)
	require.Len(t, action.Command.Arguments, 4)

	// Should use kebab-case in the snippet with cursor placeholder
	snippetText := action.Command.Arguments[3].(string)
	assert.Equal(t, ` position-identifier="$0"`, snippetText)
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

func TestAdminCodeActionProvider_SelfClosingTag(t *testing.T) {
	tempDir := t.TempDir()

	// Create the admin indexer
	adminIndexer, err := admin.NewAdminComponentIndexer(tempDir)
	require.NoError(t, err)
	defer func() { _ = adminIndexer.Close() }()

	// Index a component with required props
	jsParser := tree_sitter.NewParser()
	require.NoError(t, jsParser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_javascript.Language())))
	defer jsParser.Close()

	compCode := `
Component.register('sw-icon', {
	props: {
		name: {
			type: String,
			required: true,
		},
	},
});
`
	compTree := jsParser.Parse([]byte(compCode), nil)
	defer compTree.Close()

	compFilePath := filepath.Join(tempDir, "src", "Resources", "app", "administration", "src", "component", "sw-icon", "index.js")
	err = adminIndexer.Index(compFilePath, compTree.RootNode(), []byte(compCode))
	require.NoError(t, err)

	provider := &AdminCodeActionProvider{
		adminIndexer: adminIndexer,
	}

	twigCode := `<sw-icon />`
	tree, parser := parseTwig(t, twigCode)
	defer tree.Close()
	defer parser.Close()

	uri := "file:///project/src/Resources/app/administration/src/views/test.html.twig"

	// Find the node at the diagnostic position
	nodeAtPos := findNodeAtPosition(tree.RootNode(), 0, 1)

	params := &protocol.CodeActionParams{
		TextDocument: struct {
			URI string `json:"uri"`
		}{URI: uri},
		Range: protocol.Range{
			Start: protocol.Position{Line: 0, Character: 1},
			End:   protocol.Position{Line: 0, Character: 8},
		},
		Context: protocol.CodeActionContext{
			Diagnostics: []protocol.Diagnostic{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 1},
						End:   protocol.Position{Line: 0, Character: 8},
					},
					Code: "admin.component.missing-required-prop",
					Data: map[string]any{
						"componentName": "sw-icon",
						"propName":      "name",
					},
				},
			},
		},
		Node:            nodeAtPos,
		DocumentContent: []byte(twigCode),
	}

	actions := provider.GetCodeActions(context.Background(), params)
	require.Len(t, actions, 1)

	action := actions[0]
	require.NotNil(t, action.Command)
	assert.Equal(t, "shopware.editor.insertSnippetAtPosition", action.Command.Command)
	require.Len(t, action.Command.Arguments, 4)

	// Check that the snippet inserts before /> with cursor placeholder
	snippetText := action.Command.Arguments[3].(string)
	assert.Equal(t, ` name="$0"`, snippetText)
	// Position should be before the />
	line := action.Command.Arguments[1].(int)
	assert.Equal(t, 0, line)
}
