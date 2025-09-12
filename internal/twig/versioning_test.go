package twig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateBlockHash(t *testing.T) {
	content := `{% block test %}
    <p>Hello World</p>
{% endblock %}`

	hash := calculateBlockHash(content)
	
	// Should generate a consistent hash for the same content
	assert.NotEmpty(t, hash)
	assert.Equal(t, hash, calculateBlockHash(content))
	
	// Different content should generate different hashes
	otherContent := `{% block test %}
    <p>Different content</p>
{% endblock %}`
	
	otherHash := calculateBlockHash(otherContent)
	assert.NotEqual(t, hash, otherHash)
}

func TestParseVersionComment(t *testing.T) {
	tests := []struct {
		name     string
		comment  string
		line     int
		expected *TwigVersionComment
	}{
		{
			name:    "valid version comment",
			comment: "{# shopware-block: abc123def456@6.4.15.0 #}",
			line:    10,
			expected: &TwigVersionComment{
				Hash:    "abc123def456",
				Version: "6.4.15.0",
				Line:    10,
			},
		},
		{
			name:     "invalid comment",
			comment:  "{# just a regular comment #}",
			line:     5,
			expected: nil,
		},
		{
			name:     "malformed version comment",
			comment:  "{# shopware-block: abc123 #}",
			line:     8,
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseVersionComment(tt.comment, tt.line)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				assert.NotNil(t, result)
				assert.Equal(t, tt.expected.Hash, result.Hash)
				assert.Equal(t, tt.expected.Version, result.Version)
				assert.Equal(t, tt.expected.Line, result.Line)
			}
		})
	}
}

func TestTwigBlockHashStructure(t *testing.T) {
	blockHash := TwigBlockHash{
		Name:         "test_block",
		RelativePath: "storefront/page/checkout/cart/index.html.twig",
		AbsolutePath: "/path/to/storefront/page/checkout/cart/index.html.twig",
		Hash:         "abc123def456",
		Text:         "{% block test_block %}content{% endblock %}",
	}
	
	assert.Equal(t, "test_block", blockHash.Name)
	assert.Equal(t, "storefront/page/checkout/cart/index.html.twig", blockHash.RelativePath)
	assert.Equal(t, "abc123def456", blockHash.Hash)
}
