package symfony

import (
	"path/filepath"

	"github.com/shopware/shopware-lsp/internal/indexer"
	treesitterhelper "github.com/shopware/shopware-lsp/internal/tree_sitter_helper"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

type RouteUsage struct {
	Name string
	File string
	Line int
}

type RouteUsageIndexer struct {
	dataIndexer *indexer.DataIndexer[RouteUsage]
}

func NewRouteUsageIndexer(configDir string) (*RouteUsageIndexer, error) {
	dataIndexer, err := indexer.NewDataIndexer[RouteUsage](filepath.Join(configDir, "route_usage.db"))
	if err != nil {
		return nil, err
	}
	return &RouteUsageIndexer{
		dataIndexer: dataIndexer,
	}, nil
}

func (idx *RouteUsageIndexer) ID() string {
	return "symfony.route_usage"
}

func (idx *RouteUsageIndexer) Index(path string, node *tree_sitter.Node, fileContent []byte) error {
	matches := treesitterhelper.FindAll(node, treesitterhelper.IsPHPThisMethodCall("redirectToRoute"), fileContent)
	matches = append(matches, treesitterhelper.FindAll(node, treesitterhelper.TwigStringInFunctionPattern("seoUrl", "url", "path"), fileContent)...)

	batchSave := make(map[string]map[string]RouteUsage)

	for _, match := range matches {
		name := treesitterhelper.GetNodeText(match, fileContent)
		if _, ok := batchSave[path]; !ok {
			batchSave[path] = make(map[string]RouteUsage)
		}
		batchSave[path][name] = RouteUsage{
			Name: name,
			File: path,
			Line: int(match.Range().StartPoint.Row) + 1,
		}
	}

	return idx.dataIndexer.BatchSaveItems(batchSave)
}

func (idx *RouteUsageIndexer) RemovedFiles(paths []string) error {
	return idx.dataIndexer.BatchDeleteByFilePaths(paths)
}

func (idx *RouteUsageIndexer) Clear() error {
	return idx.dataIndexer.Clear()
}

func (idx *RouteUsageIndexer) Close() error {
	return idx.dataIndexer.Close()
}

func (idx *RouteUsageIndexer) GetRoute(name string) ([]RouteUsage, error) {
	return idx.dataIndexer.GetValues(name)
}
