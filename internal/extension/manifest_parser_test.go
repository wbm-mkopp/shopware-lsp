package extension

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tree_sitter_xml "github.com/tree-sitter-grammars/tree-sitter-xml/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

func TestParseManifestXml(t *testing.T) {
	// Test cases
	testCases := []struct {
		name          string
		xmlContent    string
		expectedName  string
		expectedLabel string
		expectNil     bool
	}{
		{
			name: "Basic manifest",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<manifest xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
          xsi:noNamespaceSchemaLocation="https://raw.githubusercontent.com/shopware/shopware/trunk/src/Core/Framework/App/Manifest/Schema/manifest-3.0.xsd">
    <meta>
        <name>TestApp</name>
        <label>Test App Label</label>
        <description>A test description</description>
        <author>Test Company Ltd.</author>
        <copyright>(c) by Test Company Ltd.</copyright>
        <version>1.0.0</version>
        <license>MIT</license>
    </meta>
</manifest>`,
			expectedName:  "TestApp",
			expectedLabel: "Test App Label",
			expectNil:     false,
		},
		{
			name: "Missing name",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<manifest xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
          xsi:noNamespaceSchemaLocation="https://raw.githubusercontent.com/shopware/shopware/trunk/src/Core/Framework/App/Manifest/Schema/manifest-3.0.xsd">
    <meta>
        <label>Test App Label</label>
        <description>A test description</description>
        <author>Test Company Ltd.</author>
        <copyright>(c) by Test Company Ltd.</copyright>
        <version>1.0.0</version>
        <license>MIT</license>
    </meta>
</manifest>`,
			expectedName:  "",
			expectedLabel: "Test App Label",
			expectNil:     false,
		},
		{
			name: "Empty elements",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<manifest xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
          xsi:noNamespaceSchemaLocation="https://raw.githubusercontent.com/shopware/shopware/trunk/src/Core/Framework/App/Manifest/Schema/manifest-3.0.xsd">
    <meta>
        <name></name>
        <label></label>
        <description></description>
        <author></author>
        <copyright></copyright>
        <version></version>
        <license></license>
    </meta>
</manifest>`,
			expectedName:  "",
			expectedLabel: "",
			expectNil:     false,
		},
		{
			name: "Missing meta node",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<manifest xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
          xsi:noNamespaceSchemaLocation="https://raw.githubusercontent.com/shopware/shopware/trunk/src/Core/Framework/App/Manifest/Schema/manifest-3.0.xsd">
</manifest>`,
			expectNil: true,
		},
		{
			name: "Not a manifest file",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>
<not-manifest>
    <something>
        <name>NotApp</name>
    </something>
</not-manifest>`,
			expectNil: true,
		},
		{
			name:       "Empty XML",
			xmlContent: `<?xml version="1.0" encoding="UTF-8"?>`,
			expectNil:  true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			parser := tree_sitter.NewParser()
			err := parser.SetLanguage(tree_sitter.NewLanguage(tree_sitter_xml.LanguageXML()))
			require.NoError(t, err)

			rootNode := parser.Parse([]byte(tc.xmlContent), nil)
			defer rootNode.Close()

			// Parse the manifest file
			manifest, err := ParseManifestXml("test.xml", rootNode.RootNode(), []byte(tc.xmlContent))
			require.NoError(t, err)

			if tc.expectNil {
				assert.Nil(t, manifest, "Expected manifest to be nil")
				return
			}

			require.NotNil(t, manifest, "Expected manifest to not be nil")
			assert.Equal(t, tc.expectedName, manifest.Name, "Name should match expected value")
			assert.Equal(t, tc.expectedLabel, manifest.Label, "Label should match expected value")
			assert.Equal(t, "test.xml", manifest.Path, "Path should match expected value")
		})
	}
}
