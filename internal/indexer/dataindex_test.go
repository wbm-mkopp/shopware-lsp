package indexer

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testStruct struct {
	Name  string
	Value int
}

// Helper function to set up a temporary database for testing
func setupTestDB[T any](t *testing.T) (*DataIndexer[T], func()) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	indexer, err := NewDataIndexer[T](dbPath)
	require.NoError(t, err, "Failed to create new data indexer")

	cleanup := func() {
		err := indexer.Close()
		if err != nil {
			// Log or handle the error during cleanup if necessary, but don't fail the test here
			t.Logf("Warning: error closing test database: %v", err)
		}
		// os.RemoveAll(tempDir) // t.TempDir() handles cleanup
	}

	return indexer, cleanup
}

func TestDataIndexer_GetAllValues_Empty(t *testing.T) {
	indexer, cleanup := setupTestDB[testStruct](t)
	defer cleanup()

	values, err := indexer.GetAllValues()
	require.NoError(t, err, "GetAllValues failed on empty DB")
	assert.Empty(t, values, "Expected empty slice for empty DB")
}

func TestDataIndexer_GetAllValues_WithData(t *testing.T) {
	indexer, cleanup := setupTestDB[testStruct](t)
	defer cleanup()

	// Prepare test data
	item1 := testStruct{Name: "ItemA", Value: 10}
	item2 := testStruct{Name: "ItemB", Value: 20}
	item3 := testStruct{Name: "ItemC", Value: 30} // Different key, same file
	item4 := testStruct{Name: "ItemD", Value: 40} // Different file

	itemsToSave := map[string]map[string]testStruct{
		"file1.txt": {
			"keyA": item1,
			"keyB": item2,
		},
		"file2.txt": {
			"keyC": item3,
		},
		"file3.txt": {
			"keyA": item4, // Same key as item1, but different file/value
		},
	}

	// Save items
	err := indexer.BatchSaveItems(itemsToSave)
	require.NoError(t, err, "BatchSaveItems failed")

	// Retrieve all values
	values, err := indexer.GetAllValues()
	require.NoError(t, err, "GetAllValues failed")

	// Assertions
	expectedValues := []testStruct{item1, item2, item3, item4}
	assert.Len(t, values, len(expectedValues), "Incorrect number of values returned")

	// Check that all expected values are present (order doesn't matter)
	assert.ElementsMatch(t, expectedValues, values, "Returned values do not match expected values")
}

func TestDataIndexer_GetAllValues_SpecificKeyRetrievalStillWorks(t *testing.T) {
	// This test ensures GetValues (key-specific) still works after modifying GetAllValues
	indexer, cleanup := setupTestDB[testStruct](t)
	defer cleanup()

	item1 := testStruct{Name: "ItemA1", Value: 10}
	item2 := testStruct{Name: "ItemA2", Value: 20}
	item3 := testStruct{Name: "ItemB1", Value: 30}

	itemsToSave := map[string]map[string]testStruct{
		"file1.txt": {
			"keyA": item1,
		},
		"file2.txt": {
			"keyA": item2,
			"keyB": item3,
		},
	}

	err := indexer.BatchSaveItems(itemsToSave)
	require.NoError(t, err, "BatchSaveItems failed")

	// Retrieve values for "keyA"
	keyAValues, err := indexer.GetValues("keyA")
	require.NoError(t, err, "GetValues(keyA) failed")
	assert.Len(t, keyAValues, 2, "Incorrect number of values for keyA")
	assert.ElementsMatch(t, []testStruct{item1, item2}, keyAValues, "Incorrect values returned for keyA")

	// Retrieve values for "keyB"
	keyBValues, err := indexer.GetValues("keyB")
	require.NoError(t, err, "GetValues(keyB) failed")
	assert.Len(t, keyBValues, 1, "Incorrect number of values for keyB")
	assert.ElementsMatch(t, []testStruct{item3}, keyBValues, "Incorrect values returned for keyB")
}

func TestDataIndexer_GetAllKeysByPath(t *testing.T) {
	indexer, cleanup := setupTestDB[testStruct](t)
	defer cleanup()

	// Prepare test data with different keys and files
	item1 := testStruct{Name: "Item1", Value: 10}
	item2 := testStruct{Name: "Item2", Value: 20}
	item3 := testStruct{Name: "Item3", Value: 30}
	item4 := testStruct{Name: "Item4", Value: 40}
	item5 := testStruct{Name: "Item5", Value: 50}

	// Structure: map[filePath]map[key]value
	itemsToSave := map[string]map[string]testStruct{
		"file1.txt": {
			"keyA": item1,
			"keyB": item2,
			"keyC": item3,
		},
		"file2.txt": {
			"keyA": item4,
			"keyD": item5,
		},
	}

	// Save items
	err := indexer.BatchSaveItems(itemsToSave)
	require.NoError(t, err, "BatchSaveItems failed")

	// Test GetAllKeysByPath for file1.txt
	file1Keys, err := indexer.GetAllKeysByPath("file1.txt")
	require.NoError(t, err, "GetAllKeysByPath failed for file1.txt")
	assert.Len(t, file1Keys, 3, "Incorrect number of keys for file1.txt")
	assert.ElementsMatch(t, []string{"keyA", "keyB", "keyC"}, file1Keys, "Incorrect keys returned for file1.txt")

	// Test GetAllKeysByPath for file2.txt
	file2Keys, err := indexer.GetAllKeysByPath("file2.txt")
	require.NoError(t, err, "GetAllKeysByPath failed for file2.txt")
	assert.Len(t, file2Keys, 2, "Incorrect number of keys for file2.txt")
	assert.ElementsMatch(t, []string{"keyA", "keyD"}, file2Keys, "Incorrect keys returned for file2.txt")

	// Test GetAllKeysByPath for non-existent file
	nonExistentKeys, err := indexer.GetAllKeysByPath("non-existent.txt")
	require.NoError(t, err, "GetAllKeysByPath should not error for non-existent file")
	assert.Empty(t, nonExistentKeys, "Expected empty keys for non-existent file")
}
