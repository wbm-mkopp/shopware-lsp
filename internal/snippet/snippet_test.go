package snippet

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseSnippetFile(t *testing.T) {
	path := filepath.Join("test.json")

	result, err := ParseSnippetFile(path)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	expected := map[string]string{
		"foo.title":     "foo",
		"foo.foo.title": "foo",
	}

	if !reflect.DeepEqual(result, expected) {
		t.Errorf("Expected %v, got %v", expected, result)
	}
}
