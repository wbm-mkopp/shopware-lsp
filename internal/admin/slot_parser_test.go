package admin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSlotsFromContent(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectedNames []string
		expectedLines []int // 1-based line numbers
	}{
		{
			name:          "no slots",
			content:       "<div>Hello World</div>",
			expectedNames: nil,
			expectedLines: nil,
		},
		{
			name:          "default slot only",
			content:       "<div><slot></slot></div>",
			expectedNames: []string{"default"},
			expectedLines: []int{1},
		},
		{
			name:          "named slot",
			content:       `<div><slot name="header"></slot></div>`,
			expectedNames: []string{"header"},
			expectedLines: []int{1},
		},
		{
			name:          "named slot with single quotes",
			content:       `<div><slot name='footer'></slot></div>`,
			expectedNames: []string{"footer"},
			expectedLines: []int{1},
		},
		{
			name: "multiple slots",
			content: `<div>
	<slot name="header"></slot>
	<slot></slot>
	<slot name="footer"></slot>
</div>`,
			expectedNames: []string{"header", "default", "footer"},
			expectedLines: []int{2, 3, 4},
		},
		{
			name: "duplicate slots deduplicated",
			content: `<div>
	<slot name="header"></slot>
	<slot name="header"></slot>
</div>`,
			expectedNames: []string{"header"},
			expectedLines: []int{2}, // Only first occurrence
		},
		{
			name:          "self-closing slot",
			content:       `<div><slot name="icon" /></div>`,
			expectedNames: []string{"icon"},
			expectedLines: []int{1},
		},
		{
			name:          "slot with other attributes",
			content:       `<div><slot name="content" :data="someData"></slot></div>`,
			expectedNames: []string{"content"},
			expectedLines: []int{1},
		},
		{
			name: "slot in twig template",
			content: `{% block sw_card %}
	<div class="sw-card">
		<slot name="header">
			{{ $tc('sw-card.defaultHeader') }}
		</slot>
		<div class="sw-card__content">
			<slot></slot>
		</div>
		<slot name="footer"></slot>
	</div>
{% endblock %}`,
			expectedNames: []string{"header", "default", "footer"},
			expectedLines: []int{3, 7, 9},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSlotsFromContent(tt.content)

			// Check slot count
			if tt.expectedNames == nil {
				assert.Nil(t, result)
				return
			}

			assert.Len(t, result, len(tt.expectedNames))

			// Check each slot
			for i, slot := range result {
				assert.Equal(t, tt.expectedNames[i], slot.Name, "slot name mismatch at index %d", i)
				if tt.expectedLines != nil && i < len(tt.expectedLines) {
					assert.Equal(t, tt.expectedLines[i], slot.Line, "slot line mismatch for %s", slot.Name)
				}
			}
		})
	}
}

func TestParseBlocksFromContent(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectedNames []string
		expectedLines []int // 1-based line numbers
	}{
		{
			name:          "no blocks",
			content:       "<div>Hello World</div>",
			expectedNames: nil,
			expectedLines: nil,
		},
		{
			name:          "single block",
			content:       "{% block sw_card %}<div>content</div>{% endblock %}",
			expectedNames: []string{"sw_card"},
			expectedLines: []int{1},
		},
		{
			name: "multiple blocks",
			content: `{% block sw_page %}
<div>
	{% block sw_page_header %}
	<header></header>
	{% endblock %}
	{% block sw_page_content %}
	<main></main>
	{% endblock %}
</div>
{% endblock %}`,
			expectedNames: []string{"sw_page", "sw_page_header", "sw_page_content"},
			expectedLines: []int{1, 3, 6},
		},
		{
			name: "blocks with slots",
			content: `{% block sw_card %}
<div class="sw-card">
	<slot name="header"></slot>
	{% block sw_card_content %}
	<slot></slot>
	{% endblock %}
</div>
{% endblock %}`,
			expectedNames: []string{"sw_card", "sw_card_content"},
			expectedLines: []int{1, 4},
		},
		{
			name: "duplicate blocks deduplicated",
			content: `{% block sw_card %}
<div>first</div>
{% endblock %}
{% block sw_card %}
<div>second</div>
{% endblock %}`,
			expectedNames: []string{"sw_card"},
			expectedLines: []int{1}, // Only first occurrence
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTemplateContent(tt.content)

			// Check block count
			if tt.expectedNames == nil {
				assert.Nil(t, result.Blocks)
				return
			}

			assert.Len(t, result.Blocks, len(tt.expectedNames))

			// Check each block
			for i, block := range result.Blocks {
				assert.Equal(t, tt.expectedNames[i], block.Name, "block name mismatch at index %d", i)
				if tt.expectedLines != nil && i < len(tt.expectedLines) {
					assert.Equal(t, tt.expectedLines[i], block.Line, "block line mismatch for %s", block.Name)
				}
			}
		})
	}
}

func TestResolveTemplatePath(t *testing.T) {
	tests := []struct {
		name           string
		definitionPath string
		templateImport string
		expected       string
	}{
		{
			name:           "relative path same dir",
			definitionPath: "/project/src/component/sw-card/index.js",
			templateImport: "./sw-card.html.twig",
			expected:       "/project/src/component/sw-card/sw-card.html.twig",
		},
		{
			name:           "relative path parent dir",
			definitionPath: "/project/src/component/sw-card/index.js",
			templateImport: "../template/sw-card.html.twig",
			expected:       "/project/src/component/template/sw-card.html.twig",
		},
		{
			name:           "filename only",
			definitionPath: "/project/src/component/sw-card/index.js",
			templateImport: "sw-card.html.twig",
			expected:       "/project/src/component/sw-card/sw-card.html.twig",
		},
		{
			name:           "empty template import",
			definitionPath: "/project/src/component/sw-card/index.js",
			templateImport: "",
			expected:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveTemplatePath(tt.definitionPath, tt.templateImport)
			assert.Equal(t, tt.expected, result)
		})
	}
}
