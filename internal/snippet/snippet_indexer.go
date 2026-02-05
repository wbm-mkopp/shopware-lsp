package snippet

import (
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/indexer"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type SnippetIndexer struct {
	frontendIndex *indexer.DataIndexer[Snippet]
	adminIndex    *indexer.DataIndexer[Snippet]
}

func NewSnippetIndexer(configDir string) (*SnippetIndexer, error) {
	frontendIndexer, err := indexer.NewDataIndexer[Snippet](filepath.Join(configDir, "frontend_snippet.db"))
	if err != nil {
		return nil, err
	}

	adminIndexer, err := indexer.NewDataIndexer[Snippet](filepath.Join(configDir, "admin_snippet.db"))
	if err != nil {
		_ = frontendIndexer.Close()
		return nil, err
	}

	return &SnippetIndexer{
		frontendIndex: frontendIndexer,
		adminIndex:    adminIndexer,
	}, nil
}

func (s *SnippetIndexer) ID() string {
	return "snippet.indexer"
}

func (s *SnippetIndexer) Index(path string, node *tree_sitter.Node, fileContent []byte) error {
	// Skip test fixtures
	if strings.Contains(path, "/_fixtures/") {
		return nil
	}

	// Check if this is a frontend snippet (Resources/snippet/)
	if strings.Contains(path, "/Resources/snippet/") {
		return s.indexFrontendSnippet(path, node, fileContent)
	}

	// Check if this is an admin snippet (Resources/app/administration/**/snippet/en-GB.json or en.json)
	if s.isAdminSnippetFile(path) {
		return s.indexAdminSnippet(path, node, fileContent)
	}

	return nil
}

// isAdminSnippetFile checks if the file is an admin snippet file
// Must be in Resources/app/administration/ and in a snippet/ folder with .json extension
func (s *SnippetIndexer) isAdminSnippetFile(path string) bool {
	if !strings.Contains(path, "/Resources/app/administration/") {
		return false
	}

	// Get the directory and filename
	dir := filepath.Dir(path)
	filename := filepath.Base(path)

	// Check if parent directory is "snippet"
	if filepath.Base(dir) != "snippet" {
		return false
	}

	// Check if it's a JSON file
	return strings.HasSuffix(filename, ".json")
}

func (s *SnippetIndexer) indexFrontendSnippet(path string, node *tree_sitter.Node, fileContent []byte) error {
	snippets, err := parseSnippetFile(node, fileContent, path)
	if err != nil {
		return err
	}

	batchSave := make(map[string]map[string]Snippet)

	for snippetKey, snippet := range snippets {
		if _, ok := batchSave[snippet.File]; !ok {
			batchSave[snippet.File] = make(map[string]Snippet)
		}
		batchSave[snippet.File][snippetKey] = snippet
	}

	return s.frontendIndex.BatchSaveItems(batchSave)
}

func (s *SnippetIndexer) indexAdminSnippet(path string, node *tree_sitter.Node, fileContent []byte) error {
	snippets, err := parseSnippetFile(node, fileContent, path)
	if err != nil {
		return err
	}

	batchSave := make(map[string]map[string]Snippet)

	for snippetKey, snippet := range snippets {
		if _, ok := batchSave[snippet.File]; !ok {
			batchSave[snippet.File] = make(map[string]Snippet)
		}
		batchSave[snippet.File][snippetKey] = snippet
	}

	return s.adminIndex.BatchSaveItems(batchSave)
}

func (s *SnippetIndexer) RemovedFiles(paths []string) error {
	// Separate paths by type
	var frontendPaths, adminPaths []string
	for _, path := range paths {
		if strings.Contains(path, "/Resources/app/administration/") {
			adminPaths = append(adminPaths, path)
		} else if strings.Contains(path, "/Resources/snippet/") {
			frontendPaths = append(frontendPaths, path)
		}
	}

	if len(frontendPaths) > 0 {
		if err := s.frontendIndex.BatchDeleteByFilePaths(frontendPaths); err != nil {
			return err
		}
	}

	if len(adminPaths) > 0 {
		if err := s.adminIndex.BatchDeleteByFilePaths(adminPaths); err != nil {
			return err
		}
	}

	return nil
}

func (s *SnippetIndexer) Close() error {
	if err := s.frontendIndex.Close(); err != nil {
		return err
	}
	return s.adminIndex.Close()
}

func (s *SnippetIndexer) Clear() error {
	if err := s.frontendIndex.Clear(); err != nil {
		return err
	}
	return s.adminIndex.Clear()
}

func (s *SnippetIndexer) GetFrontendSnippets() ([]string, error) {
	return s.frontendIndex.GetAllKeys()
}

func (s *SnippetIndexer) GetFrontendSnippet(key string) ([]Snippet, error) {
	return s.frontendIndex.GetValues(key)
}

func (s *SnippetIndexer) GetAllFrontendSnippets() ([]Snippet, error) {
	return s.frontendIndex.GetAllValues()
}

func (s *SnippetIndexer) GetAdminSnippetKeys() ([]string, error) {
	return s.adminIndex.GetAllKeys()
}

func (s *SnippetIndexer) GetAdminSnippet(key string) ([]Snippet, error) {
	return s.adminIndex.GetValues(key)
}

func (s *SnippetIndexer) GetAllAdminSnippets() ([]Snippet, error) {
	return s.adminIndex.GetAllValues()
}

// GetFrontendSnippetsWithText returns a map of snippet keys to their text (preferring English)
func (s *SnippetIndexer) GetFrontendSnippetsWithText() (map[string]string, error) {
	return s.getSnippetsWithText(s.frontendIndex)
}

// GetAdminSnippetsWithText returns a map of snippet keys to their text (preferring English)
func (s *SnippetIndexer) GetAdminSnippetsWithText() (map[string]string, error) {
	return s.getSnippetsWithText(s.adminIndex)
}

func (s *SnippetIndexer) getSnippetsWithText(idx *indexer.DataIndexer[Snippet]) (map[string]string, error) {
	allSnippets, err := idx.GetAllValues()
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, snippet := range allSnippets {
		existing, exists := result[snippet.Key]
		if !exists {
			result[snippet.Key] = snippet.Text
			continue
		}

		// Prefer English translations (en-GB, en_GB, en)
		file := strings.ToLower(snippet.File)
		if strings.Contains(file, "en-gb") || strings.Contains(file, "en_gb") || strings.Contains(file, "/en.json") || strings.Contains(file, "/en/") {
			result[snippet.Key] = snippet.Text
		} else if existing == "" {
			// Use any non-empty text if we don't have one yet
			result[snippet.Key] = snippet.Text
		}
	}

	return result, nil
}
