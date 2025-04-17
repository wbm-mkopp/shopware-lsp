package symfony

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceIndex(t *testing.T) {
	// Create a temporary directory for test files
	tempDir := t.TempDir()

	// Disable debug logging for cleaner test output

	// Create test XML files
	testFile1 := filepath.Join(tempDir, "services1.xml")
	testFile2 := filepath.Join(tempDir, "services2.xml")

	// Create a simple services XML file with the format expected by the parser
	err := os.WriteFile(testFile1, []byte(`<?xml version="1.0" encoding="UTF-8" ?>
<container>
    <service id="app.service1" class="App\Service\Service1">
        <tag name="app.tag" />
    </service>
    <service id="app.service2" class="App\Service\Service2" />
</container>`), 0644)
	require.NoError(t, err, "Failed to write test file")

	// Create another services XML file with aliases
	err = os.WriteFile(testFile2, []byte(`<?xml version="1.0" encoding="UTF-8" ?>
<container>
    <service id="app.service3" class="App\Service\Service3" />
    <service id="app.service4" class="App\Service\Service4">
        <tag name="app.tag2" />
    </service>
    <alias id="app.alias1" service="app.service1" />
    <alias id="app.alias2" service="app.service3" />
</container>`), 0644)
	require.NoError(t, err, "Failed to write test file")

	// Create the service index
	index, err := NewServiceIndex(tempDir, t.TempDir())
	require.NoError(t, err, "Failed to create service index")
	defer func() {
		if err := index.Close(); err != nil {
			t.Logf("Error closing index: %v", err)
		}
	}()

	// Build the index
	err = index.Index()
	require.NoError(t, err, "Failed to build index")

	// Test tag index functionality
	t.Run("TagIndex", func(t *testing.T) {
		// Test GetTagCount
		tagCount := index.GetTagCount()
		assert.Equal(t, 2, tagCount, "Expected 2 unique tags")

		// Test GetAllTags
		allTags := index.GetAllTags()
		assert.Len(t, allTags, 2, "Expected 2 tags")
		assert.Contains(t, allTags, "app.tag", "Expected 'app.tag' in tags list")
		assert.Contains(t, allTags, "app.tag2", "Expected 'app.tag2' in tags list")

		// Test GetServicesByTag
		servicesWithTag1 := index.GetServicesByTag("app.tag")
		assert.Len(t, servicesWithTag1, 1, "Expected 1 service with tag 'app.tag'")
		assert.Contains(t, servicesWithTag1, "app.service1", "Expected 'app.service1' to have tag 'app.tag'")

		servicesWithTag2 := index.GetServicesByTag("app.tag2")
		assert.Len(t, servicesWithTag2, 1, "Expected 1 service with tag 'app.tag2'")
		assert.Contains(t, servicesWithTag2, "app.service4", "Expected 'app.service4' to have tag 'app.tag2'")

		// Test tag that doesn't exist
		servicesWithNonExistentTag := index.GetServicesByTag("non.existent.tag")
		assert.Empty(t, servicesWithNonExistentTag, "Expected no services with non-existent tag")
	})

	// Test GetAllServices
	t.Run("GetAllServices", func(t *testing.T) {
		services := index.GetAllServices()
		assert.Len(t, services, 6, "Expected 6 service IDs (4 services + 2 aliases)")

		// Check for expected service IDs
		expectedIDs := map[string]bool{
			"app.service1": false,
			"app.service2": false,
			"app.service3": false,
			"app.service4": false,
			"app.alias1":   false,
			"app.alias2":   false,
		}

		for _, id := range services {
			expectedIDs[id] = true
		}

		for id, found := range expectedIDs {
			assert.True(t, found, "Expected service ID %s not found", id)
		}
	})

	// Test GetServiceByID
	t.Run("GetServiceByID", func(t *testing.T) {
		// Test direct service lookup
		service, found := index.GetServiceByID("app.service1")
		assert.True(t, found, "Service app.service1 not found")
		assert.Equal(t, "App\\Service\\Service1", service.Class, "Expected class App\\Service\\Service1")
		assert.Len(t, service.Tags, 1, "Expected 1 tag")

		// Test alias resolution
		aliasedService, found := index.GetServiceByID("app.alias1")
		assert.True(t, found, "Aliased service app.alias1 not found")
		assert.Equal(t, "App\\Service\\Service1", aliasedService.Class, "Expected aliased service to have class App\\Service\\Service1")

		// Test non-existent service
		_, found = index.GetServiceByID("non.existent")
		assert.False(t, found, "Non-existent service should not be found")
	})

	// Test file watcher (modify a file)
	t.Run("FileWatcher", func(t *testing.T) {
		// Modify an existing file
		modifiedContent := []byte(`<?xml version="1.0" encoding="UTF-8" ?>
<container>
    <service id="app.service1" class="App\Service\Service1Modified">
        <tag name="app.tag" />
        <tag name="app.tag.new" />
    </service>
    <service id="app.service2" class="App\Service\Service2" />
    <service id="app.service5" class="App\Service\Service5" />
</container>`)
		err = os.WriteFile(testFile1, modifiedContent, 0644)
		require.NoError(t, err, "Failed to modify test file")

		// Skip debug logging

		// Force the file watcher to detect the change by explicitly triggering a rebuild
		err = index.Index()
		require.NoError(t, err, "Failed to rebuild index after file modification")

		// Check that the service was updated
		service, found := index.GetServiceByID("app.service1")
		assert.True(t, found, "Modified service app.service1 not found")
		assert.Equal(t, "App\\Service\\Service1Modified", service.Class, "Expected modified class App\\Service\\Service1Modified")
		assert.Len(t, service.Tags, 2, "Expected 2 tags after modification")

		// Check that the new service was added
		_, found = index.GetServiceByID("app.service5")
		assert.True(t, found, "New service app.service5 not found")

		// Check updated counts
		serviceCount, aliasCount := index.GetCounts()
		assert.Equal(t, 5, serviceCount, "Expected 5 services after modification (4 original + 1 new - 0 removed)")
		assert.Equal(t, 2, aliasCount, "Expected 2 aliases after modification")
	})

	// Test file watcher (delete a file)
	t.Run("FileWatcherDelete", func(t *testing.T) {
		// Remove a file
		err = os.Remove(testFile2)
		require.NoError(t, err, "Failed to remove test file")

		// Force the indexer to rebuild after file deletion
		err = index.Index()
		require.NoError(t, err, "Failed to rebuild index after file deletion")

		// Check that services from the deleted file are gone
		_, found := index.GetServiceByID("app.service3")
		assert.False(t, found, "Service from deleted file should not be found")

		_, found = index.GetServiceByID("app.alias1")
		assert.False(t, found, "Alias from deleted file should not be found")

		// Check updated counts
		serviceCount, aliasCount := index.GetCounts()
		assert.Equal(t, 3, serviceCount, "Expected 3 services after file deletion (5 before - 2 removed)")
		assert.Equal(t, 0, aliasCount, "Expected 0 aliases after file deletion (2 before - 2 removed)")
	})
}
