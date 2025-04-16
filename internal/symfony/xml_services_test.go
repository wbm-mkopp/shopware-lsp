package symfony

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseXMLServices(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Test cases
	testCases := []struct {
		name             string
		xmlContent       string
		expectedServices int
		expectedAliases  int
		expectedTags     map[string][]string // map[serviceID][]tagNames
		expectError      bool
	}{
		{
			name: "Basic service",
			xmlContent: `<?xml version="1.0" encoding="UTF-8" ?>
<container>
    <service id="app.service1" class="App\Service\Service1" />
</container>`,
			expectedServices: 1,
			expectedAliases:  0,
			expectedTags:     map[string][]string{},
			expectError:      false,
		},
		{
			name: "Service with tags",
			xmlContent: `<?xml version="1.0" encoding="UTF-8" ?>
<container>
    <service id="app.service1" class="App\Service\Service1">
        <tag name="app.tag" />
    </service>
</container>`,
			expectedServices: 1,
			expectedAliases:  0,
			expectedTags:     map[string][]string{"app.service1": {"app.tag"}},
			expectError:      false,
		},
		{
			name: "Service with multiple tags",
			xmlContent: `<?xml version="1.0" encoding="UTF-8" ?>
<container>
    <service id="app.service1" class="App\Service\Service1">
        <tag name="app.tag1" />
        <tag name="app.tag2" />
    </service>
</container>`,
			expectedServices: 1,
			expectedAliases:  0,
			expectedTags:     map[string][]string{"app.service1": {"app.tag1", "app.tag2"}},
			expectError:      false,
		},
		{
			name: "Multiple services",
			xmlContent: `<?xml version="1.0" encoding="UTF-8" ?>
<container>
    <service id="app.service1" class="App\Service\Service1" />
    <service id="app.service2" class="App\Service\Service2" />
</container>`,
			expectedServices: 2,
			expectedAliases:  0,
			expectedTags:     map[string][]string{},
			expectError:      false,
		},
		{
			name: "Services with aliases",
			xmlContent: `<?xml version="1.0" encoding="UTF-8" ?>
<container>
    <service id="app.service1" class="App\Service\Service1" />
    <alias id="app.alias1" service="app.service1" />
</container>`,
			expectedServices: 1,
			expectedAliases:  1,
			expectedTags:     map[string][]string{},
			expectError:      false,
		},
		{
			name: "Complex XML with services, tags, and aliases",
			xmlContent: `<?xml version="1.0" encoding="UTF-8" ?>
<container>
    <service id="app.service1" class="App\Service\Service1">
        <tag name="app.tag1" />
        <tag name="app.tag2" />
    </service>
    <service id="app.service2" class="App\Service\Service2">
        <tag name="app.tag3" />
    </service>
    <alias id="app.alias1" service="app.service1" />
    <alias id="app.alias2" service="app.service2" />
</container>`,
			expectedServices: 2,
			expectedAliases:  2,
			expectedTags:     map[string][]string{
				"app.service1": {"app.tag1", "app.tag2"},
				"app.service2": {"app.tag3"},
			},
			expectError: false,
		},
		{
			name: "Symfony namespaced XML with nested services",
			xmlContent: `<?xml version="1.0" encoding="UTF-8" ?>
<container xmlns="http://symfony.com/schema/dic/services"
    xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
    xsi:schemaLocation="http://symfony.com/schema/dic/services
        https://symfony.com/schema/dic/services/services-1.0.xsd">

    <services>
        <!-- Default configuration for services in *this* file -->
        <defaults autowire="true" autoconfigure="true"/>

        <!-- makes classes in src/ available to be used as services -->
        <!-- this creates a service per class whose id is the fully-qualified class name -->
        <prototype namespace="App\" resource="../src/" exclude="../src/{DependencyInjection,Entity,Kernel.php}"/>

        <service id="App\Service\SiteUpdateManager">
            <argument key="$adminEmail">manager@example.com</argument>
        </service>

        <service id="bla">
            <argument type="service" id=""/>
        </service>
    </services>
</container>`,
			expectedServices: 2,
			expectedAliases:  0,
			expectedTags:     map[string][]string{},
			expectError:      false,
		},
		// Add test cases for invalid/non-service XML
		{
			name: "Non-service XML - HTML document",
			xmlContent: `<!DOCTYPE html>
<html>
<head>
    <title>Test HTML</title>
</head>
<body>
    <h1>This is not a service file</h1>
    <p>Just a regular HTML document</p>
</body>
</html>`,
			expectedServices: 0,
			expectedAliases:  0,
			expectedTags:     map[string][]string{},
			expectError:      false,
		},
		{
			name: "XML without container tag",
			xmlContent: `<?xml version="1.0" encoding="UTF-8" ?>
<config>
    <parameters>
        <parameter name="test">value</parameter>
    </parameters>
</config>`,
			expectedServices: 0,
			expectedAliases:  0,
			expectedTags:     map[string][]string{},
			expectError:      false,
		},
		{
			name: "Empty XML",
			xmlContent: `<?xml version="1.0" encoding="UTF-8" ?>
<container>
</container>`,
			expectedServices: 0,
			expectedAliases:  0,
			expectedTags:     map[string][]string{},
			expectError:      false,
		},
		{
			name: "Services with missing attributes",
			xmlContent: `<?xml version="1.0" encoding="UTF-8" ?>
<container>
    <service />
    <service id="" />
    <service class="App\Service\MissingId" />
    <alias />
    <alias id="missing.service.reference" />
</container>`,
			expectedServices: 0,
			expectedAliases:  0,
			expectedTags:     map[string][]string{},
			expectError:      false,
		},
		{
			name:             "Malformed XML",
			xmlContent:       `<?xml version="1.0" encoding="UTF-8" ?><container><service></container>`,
			expectedServices: 0,
			expectedAliases:  0,
			expectedTags:     map[string][]string{},
			expectError:      false, // Tree-sitter can still parse malformed XML without errors
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a temporary XML file
			testFile := filepath.Join(tempDir, "test.xml")
			err := os.WriteFile(testFile, []byte(tc.xmlContent), 0644)
			require.NoError(t, err, "Failed to write test file")

			// Parse the XML file
			services, aliases, err := ParseXMLServices(testFile)
			
			if tc.expectError {
				require.Error(t, err, "Expected ParseXMLServices to fail")
				return
			}
			
			require.NoError(t, err, "ParseXMLServices failed")

			// Check service count
			assert.Len(t, services, tc.expectedServices, "Expected %d services, got %d", tc.expectedServices, len(services))

			// Check alias count
			assert.Len(t, aliases, tc.expectedAliases, "Expected %d aliases, got %d", tc.expectedAliases, len(aliases))

			// Check tags
			for serviceID, expectedTags := range tc.expectedTags {
				var service *Service
				for i := range services {
					if services[i].ID == serviceID {
						service = &services[i]
						break
					}
				}

				assert.NotNil(t, service, "Service %s not found", serviceID)
				if service == nil {
					continue
				}

				// Check that all expected tags are present
				for _, expectedTag := range expectedTags {
					_, found := service.Tags[expectedTag]
					assert.True(t, found, "Expected tag %s for service %s not found", expectedTag, serviceID)
				}

				// Check that there are no unexpected tags
				assert.Len(t, service.Tags, len(expectedTags), "Expected %d tags for service %s, got %d", len(expectedTags), serviceID, len(service.Tags))
			}
		})
	}
}