package diagnostics

import (
	"context"
	"testing"

	"github.com/shopware/shopware-lsp/internal/extension"
	"github.com/shopware/shopware-lsp/internal/indexer"
	"github.com/shopware/shopware-lsp/internal/lsp"
	"github.com/shopware/shopware-lsp/internal/lsp/protocol"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_twig "github.com/shopware/shopware-lsp/internal/tree_sitter_grammars/twig/bindings/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThemeDiagnosticsProvider_twigDiagnostics(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected []protocol.Diagnostic
	}{
		{
			name: "valid icon with default pack",
			content: `{% sw_icon 'heart' %}`,
			expected: []protocol.Diagnostic{},
		},
		{
			name: "invalid icon with default pack",
			content: `{% sw_icon 'nonexistent-icon' %}`,
			expected: []protocol.Diagnostic{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 11},
						End:   protocol.Position{Line: 0, Character: 29},
					},
					Message:  "Icon 'nonexistent-icon' not found in pack 'default'",
					Source:   "shopware",
					Severity: protocol.DiagnosticSeverityError,
					Code:     "theme.icon.missing",
					Data: map[string]any{
						"iconName": "nonexistent-icon",
						"pack":     "default",
					},
				},
			},
		},
		{
			name: "invalid icon with custom pack",
			content: `{% sw_icon 'missing' {'pack': 'custom'} %}`,
			expected: []protocol.Diagnostic{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 0, Character: 11},
						End:   protocol.Position{Line: 0, Character: 20},
					},
					Message:  "Icon 'missing' not found in pack 'custom'",
					Source:   "shopware",
					Severity: protocol.DiagnosticSeverityError,
					Code:     "theme.icon.missing",
					Data: map[string]any{
						"iconName": "missing",
						"pack":     "custom",
					},
				},
			},
		},
		{
			name: "multiple icons with mixed validity",
			content: `{% sw_icon 'heart' %}
{% sw_icon 'invalid-icon' %}
{% sw_icon 'home' %}`,
			expected: []protocol.Diagnostic{
				{
					Range: protocol.Range{
						Start: protocol.Position{Line: 1, Character: 11},
						End:   protocol.Position{Line: 1, Character: 25},
					},
					Message:  "Icon 'invalid-icon' not found in pack 'default'",
					Source:   "shopware",
					Severity: protocol.DiagnosticSeverityError,
					Code:     "theme.icon.missing",
					Data: map[string]any{
						"iconName": "invalid-icon",
						"pack":     "default",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			
			// Create temp directory with mock icon structure
			tempDir := t.TempDir()
			
			// Create file scanner and server
			fileScanner, err := indexer.NewFileScanner(tempDir, tempDir+"/scanner.db")
			require.NoError(t, err)
			
			server := lsp.NewServer(fileScanner, tempDir, "test")
			
			// Create and register extension indexer
			extIndexer, _ := extension.NewExtensionIndexer(tempDir)
			server.RegisterIndexer(extIndexer, nil)
			
			// Create mock icon provider that returns specific icons
			mockIconProvider := &mockIconProvider{
				icons: map[string]map[string]string{
					"default": {
						"heart": tempDir + "/icon/default/heart.svg",
						"home":  tempDir + "/icon/default/home.svg",
					},
				},
			}
			
			provider := &ThemeDiagnosticsProvider{
				iconProvider: mockIconProvider,
			}

			// Parse content
			lang := tree_sitter.NewLanguage(tree_sitter_twig.Language())
			parser := tree_sitter.NewParser()
			err = parser.SetLanguage(lang)
			require.NoError(t, err)
			tree := parser.Parse([]byte(tt.content), nil)
			defer tree.Close()

			diagnostics, err := provider.GetDiagnostics(ctx, "test.twig", tree.RootNode(), []byte(tt.content))
			require.NoError(t, err)

			assert.Equal(t, len(tt.expected), len(diagnostics))
			for i, expected := range tt.expected {
				assert.Equal(t, expected.Range, diagnostics[i].Range)
				assert.Equal(t, expected.Message, diagnostics[i].Message)
				assert.Equal(t, expected.Severity, diagnostics[i].Severity)
				assert.Equal(t, expected.Code, diagnostics[i].Code)
				assert.Equal(t, expected.Data, diagnostics[i].Data)
			}
		})
	}
}

// mockIconProvider is a test mock for theme.IconProvider
type mockIconProvider struct {
	icons map[string]map[string]string
}

func (m *mockIconProvider) GetIconPacks() []string {
	packs := make([]string, 0, len(m.icons))
	for pack := range m.icons {
		packs = append(packs, pack)
	}
	return packs
}

func (m *mockIconProvider) GetIcons(pack string) []string {
	icons := make([]string, 0)
	if packIcons, ok := m.icons[pack]; ok {
		for icon := range packIcons {
			icons = append(icons, icon)
		}
	}
	return icons
}

func (m *mockIconProvider) GetIcon(pack, icon string) string {
	if packIcons, ok := m.icons[pack]; ok {
		return packIcons[icon]
	}
	return ""
}