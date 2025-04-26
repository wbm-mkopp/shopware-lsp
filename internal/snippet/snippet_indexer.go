package snippet

import (
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/indexer"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type SnippetIndexer struct {
	frontendIndex *indexer.DataIndexer[Snippet]
}

func NewSnippetIndexer(configDir string) (*SnippetIndexer, error) {
	frontendIndexer, err := indexer.NewDataIndexer[Snippet](filepath.Join(configDir, "frontend_snippet.db"))

	if err != nil {
		return nil, err
	}

	return &SnippetIndexer{
		frontendIndex: frontendIndexer,
	}, nil
}

func (s *SnippetIndexer) ID() string {
	return "snippet.indexer"
}

func (s *SnippetIndexer) Index(path string, node *tree_sitter.Node, fileContent []byte) error {
	if !strings.Contains(path, "/Resources/snippet/") || strings.Contains(path, "/_fixtures/") {
		return nil
	}

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

func (s *SnippetIndexer) RemovedFiles(paths []string) error {
	return s.frontendIndex.BatchDeleteByFilePaths(paths)
}

func (s *SnippetIndexer) Close() error {
	return s.frontendIndex.Close()
}
