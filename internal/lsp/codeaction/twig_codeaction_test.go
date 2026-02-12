package codeaction

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
