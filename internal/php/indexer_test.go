package php

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetClassesOfFile(t *testing.T) {
	index, err := NewPHPIndex("testdata")
	assert.NoError(t, err)

	classes := index.GetClassesOfFile("testdata/01.php")

	assert.Len(t, classes, 1)

	expectedClassName := "Shopware\\Core\\Content\\Category\\Service\\NavigationLoader"

	assert.Contains(t, classes, expectedClassName)

	assert.Equal(t, expectedClassName, classes[expectedClassName].Name)
	assert.Equal(t, "testdata/01.php", classes[expectedClassName].Path)
	assert.Equal(t, 20, classes[expectedClassName].Line)
}

func TestSkipNodeModulesAndVarFolders(t *testing.T) {
	// Create a temporary test directory
	tempDir := t.TempDir()

	// Create a PHP file in the root directory
	rootFile := filepath.Join(tempDir, "root.php")
	writeTestPHPClass(t, rootFile, "RootClass")

	// Create node_modules directory with a PHP file
	nodeModulesDir := filepath.Join(tempDir, "node_modules")
	assert.NoError(t, os.Mkdir(nodeModulesDir, 0755))
	nodeModulesFile := filepath.Join(nodeModulesDir, "node_module.php")
	writeTestPHPClass(t, nodeModulesFile, "NodeModuleClass")

	// Create var directory with a PHP file
	varDir := filepath.Join(tempDir, "var")
	assert.NoError(t, os.Mkdir(varDir, 0755))
	varFile := filepath.Join(varDir, "var.php")
	writeTestPHPClass(t, varFile, "VarClass")

	// Create a subdirectory (not at root level) with a PHP file
	subDir := filepath.Join(tempDir, "subdir")
	assert.NoError(t, os.Mkdir(subDir, 0755))
	subFile := filepath.Join(subDir, "sub.php")
	writeTestPHPClass(t, subFile, "SubClass")

	// Create a PHP index for the temp directory
	index, err := NewPHPIndex(tempDir)
	assert.NoError(t, err)

	// Run the indexer
	err = index.Index()
	assert.NoError(t, err)

	// Get all class names from the index
	classNames := index.GetClassNames()

	// Verify that only the root and subdir classes were indexed
	assert.Contains(t, classNames, "RootClass")
	assert.Contains(t, classNames, "SubClass")

	// Verify that node_modules and var classes were not indexed
	assert.NotContains(t, classNames, "NodeModuleClass")
	assert.NotContains(t, classNames, "VarClass")
}

func writeTestPHPClass(t *testing.T, filePath, className string) {
	content := "<?php\n\nclass " + className + " {}\n"
	assert.NoError(t, os.WriteFile(filePath, []byte(content), 0644))
}
