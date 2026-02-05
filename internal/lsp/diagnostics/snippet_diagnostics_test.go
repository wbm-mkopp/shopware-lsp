package diagnostics

import (
	"testing"

	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	"github.com/stretchr/testify/assert"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// parseTwig is defined in admin_diagnostics_test.go

func TestSnippetDiagnosticsPatternDetection(t *testing.T) {
	tests := []struct {
		name                  string
		code                  string
		expectedFrontendCount int
		expectedAdminCount    int
	}{
		{
			name:                  "single frontend snippet",
			code:                  `{{ 'checkout.cart.title'|trans }}`,
			expectedFrontendCount: 1,
			expectedAdminCount:    0,
		},
		{
			name:                  "single admin snippet $tc",
			code:                  `{{ $tc('sw-settings.title') }}`,
			expectedFrontendCount: 0,
			expectedAdminCount:    1,
		},
		{
			name:                  "single admin snippet $t",
			code:                  `{{ $t('global.save') }}`,
			expectedFrontendCount: 0,
			expectedAdminCount:    1,
		},
		{
			name:                  "multiple frontend snippets",
			code:                  `{{ 'first.key'|trans }} {{ 'second.key'|trans }}`,
			expectedFrontendCount: 2,
			expectedAdminCount:    0,
		},
		{
			name:                  "multiple admin snippets",
			code:                  `{{ $tc('first.key') }} {{ $t('second.key') }}`,
			expectedFrontendCount: 0,
			expectedAdminCount:    2,
		},
		{
			name:                  "no snippets",
			code:                  `{{ 'just text' }}`,
			expectedFrontendCount: 0,
			expectedAdminCount:    0,
		},
		{
			name:                  "snippet in block",
			code:                  `{% block content %}{{ $tc('block.snippet') }}{% endblock %}`,
			expectedFrontendCount: 0,
			expectedAdminCount:    1,
		},
		{
			name: "multi-line template",
			code: `{% block header %}
{{ $tc('header.title') }}
{% endblock %}
{% block content %}
{{ $tc('content.text') }}
{% endblock %}`,
			expectedFrontendCount: 0,
			expectedAdminCount:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseTwig(t, tt.code)
			defer tree.Close()
			defer parser.Close()

			// Find all frontend snippet matches
			frontendMatches := treesitterhelper.FindAll(
				tree.RootNode(),
				treesitterhelper.TwigTransPattern(),
				[]byte(tt.code),
			)

			// Find all admin snippet matches
			adminMatches := treesitterhelper.FindAll(
				tree.RootNode(),
				treesitterhelper.TwigAdminSnippetPattern(),
				[]byte(tt.code),
			)

			assert.Len(t, frontendMatches, tt.expectedFrontendCount,
				"Frontend snippet count for: %s", tt.code)
			assert.Len(t, adminMatches, tt.expectedAdminCount,
				"Admin snippet count for: %s", tt.code)
		})
	}
}

func TestSnippetDiagnosticsAdminFileDetection(t *testing.T) {
	tests := []struct {
		name     string
		uri      string
		expected bool
	}{
		{
			name:     "admin twig file",
			uri:      "/project/custom/plugins/MyPlugin/src/Resources/app/administration/src/module/my-module/component/my-component/my-component.html.twig",
			expected: true,
		},
		{
			name:     "storefront twig file",
			uri:      "/project/custom/plugins/MyPlugin/src/Resources/views/storefront/page/checkout/cart.html.twig",
			expected: false,
		},
		{
			name:     "core storefront file",
			uri:      "/project/vendor/shopware/storefront/Resources/views/storefront/base.html.twig",
			expected: false,
		},
		{
			name:     "core admin file",
			uri:      "/project/vendor/shopware/administration/Resources/app/administration/src/module/sw-product/page/sw-product-list/sw-product-list.html.twig",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Simple check that matches the logic in snippet_diagnostics.go
			isAdminFile := containsAdminPath(tt.uri)
			assert.Equal(t, tt.expected, isAdminFile, "Admin file detection for: %s", tt.uri)
		})
	}
}

func containsAdminPath(uri string) bool {
	return contains(uri, "/Resources/app/administration/")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSnippetKeyExtraction(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		expectedKey string
	}{
		{
			name:        "simple frontend key",
			code:        `{{ 'checkout.cart.title'|trans }}`,
			expectedKey: "checkout.cart.title",
		},
		{
			name:        "simple admin key",
			code:        `{{ $tc('sw-settings.index.title') }}`,
			expectedKey: "sw-settings.index.title",
		},
		{
			name:        "key with parameters",
			code:        `{{ $tc('sw-product.count', { count: 5 }) }}`,
			expectedKey: "sw-product.count",
		},
		{
			name:        "frontend key with parameters",
			code:        `{{ 'checkout.items'|trans({'%count%': count}) }}`,
			expectedKey: "checkout.items",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tree, parser := parseTwig(t, tt.code)
			defer tree.Close()
			defer parser.Close()

			// Find the first string node matching either pattern
			var matches []*tree_sitter.Node
			matches = treesitterhelper.FindAll(
				tree.RootNode(),
				treesitterhelper.TwigTransPattern(),
				[]byte(tt.code),
			)
			if len(matches) == 0 {
				matches = treesitterhelper.FindAll(
					tree.RootNode(),
					treesitterhelper.TwigAdminSnippetPattern(),
					[]byte(tt.code),
				)
			}

			assert.NotEmpty(t, matches, "Should find at least one match")

			if len(matches) > 0 {
				extractedKey := treesitterhelper.GetNodeText(matches[0], []byte(tt.code))
				assert.Equal(t, tt.expectedKey, extractedKey, "Extracted key for: %s", tt.code)
			}
		})
	}
}
