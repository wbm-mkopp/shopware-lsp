package php

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGroupUseStatements(t *testing.T) {
	idx, err := NewPHPIndex("")
	assert.NoError(t, err)
	path := filepath.Join("testdata", "05.php")
	classes := idx.GetClassesOfFile(path)

	// Verify the class was found
	assert.Contains(t, classes, "App\\Controller\\TestController")
	class := classes["App\\Controller\\TestController"]

	// Check property types are correctly resolved
	expectedTypes := map[string]string{
		"request":    "Symfony\\Component\\HttpFoundation\\Request",
		"response":   "Symfony\\Component\\HttpFoundation\\Response",
		"kernel":     "Symfony\\Component\\HttpKernel\\Kernel",
		"connection": "Doctrine\\DBAL\\Connection",
		"statement":  "Doctrine\\DBAL\\Statement",
		"context":    "Shopware\\Core\\Framework\\Context",
		"repository": "Shopware\\Core\\Framework\\DataAbstractionLayer\\EntityRepository",
		"criteria":   "Shopware\\Core\\Framework\\DataAbstractionLayer\\Search\\Criteria",
		"filter":     "Shopware\\Core\\Framework\\DataAbstractionLayer\\Search\\Filter\\EqualsFilter",
	}

	for propName, expectedType := range expectedTypes {
		assert.Contains(t, class.Properties, propName, "Property %s should exist", propName)
		assert.Equal(t, expectedType, class.Properties[propName].Type, "Property %s should have type %s", propName, expectedType)
	}
}
