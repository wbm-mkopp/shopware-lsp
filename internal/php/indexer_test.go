package php

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetClassesOfFile(t *testing.T) {
	index, err := NewPHPIndex(t.TempDir())
	assert.NoError(t, err)

	classes := index.GetClassesOfFile("testdata/01.php")

	assert.Len(t, classes, 1)

	expectedClassName := "Shopware\\Core\\Content\\Category\\Service\\NavigationLoader"

	assert.Contains(t, classes, expectedClassName)

	assert.Equal(t, expectedClassName, classes[expectedClassName].Name)
	assert.Equal(t, "testdata/01.php", classes[expectedClassName].Path)
	assert.Equal(t, 20, classes[expectedClassName].Line)
}
