package twig

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTwigParse(t *testing.T) {
	content := []byte(`{% block foo %}{% endblock %}`)

	file, err := ParseTwig(nil, content, "test")
	assert.NoError(t, err)

	assert.Equal(t, "test", file.Path)
	assert.Equal(t, map[string]TwigBlock{"foo": {Name: "foo", Line: 1}}, file.Blocks)
}
