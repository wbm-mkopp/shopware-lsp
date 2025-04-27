package symfony

import (
	"path/filepath"
	"strings"

	"github.com/shopware/shopware-lsp/internal/indexer"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// Route represents a Symfony route from YAML, PHP, or other sources
type Route struct {
	Name       string
	Path       string
	Controller string
	FilePath   string
	Line       int
}

type RouteList []Route

func (rl RouteList) GetByController(name string) *Route {
	for _, r := range rl {
		if r.Controller == name {
			return &r
		}
	}
	return nil
}

type RouteIndexer struct {
	dataIndexer *indexer.DataIndexer[Route]
}

func NewRouteIndexer(configDir string) (*RouteIndexer, error) {
	dataIndexer, err := indexer.NewDataIndexer[Route](filepath.Join(configDir, "route.db"))
	if err != nil {
		return nil, err
	}
	return &RouteIndexer{
		dataIndexer: dataIndexer,
	}, nil
}

func (idx *RouteIndexer) ID() string {
	return "symfony.route"
}

func (idx *RouteIndexer) GetRoutes() (RouteList, error) {
	return idx.dataIndexer.GetAllValues()
}

func (idx *RouteIndexer) GetRoute(name string) ([]Route, error) {
	return idx.dataIndexer.GetValues(name)
}

func (idx *RouteIndexer) Index(path string, node *tree_sitter.Node, fileContent []byte) error {
	fileExt := strings.ToLower(filepath.Ext(path))

	switch fileExt {
	case ".yml", ".yaml":
		return idx.indexYaml(path, node, fileContent)
	case ".php":
		return idx.indexPhp(path, node, fileContent)
	default:
		return nil
	}
}

func (idx *RouteIndexer) indexYaml(path string, node *tree_sitter.Node, fileContent []byte) error {
	parsedRoutes, err := ParseYAMLRoutes(path, node, fileContent)
	if err != nil {
		return err
	}

	batchSave := make(map[string]map[string]Route)
	for _, route := range parsedRoutes {
		if _, ok := batchSave[route.FilePath]; !ok {
			batchSave[route.FilePath] = make(map[string]Route)
		}
		batchSave[route.FilePath][route.Name] = route
	}

	return idx.dataIndexer.BatchSaveItems(batchSave)
}

func (idx *RouteIndexer) indexPhp(path string, node *tree_sitter.Node, fileContent []byte) error {
	parsedRoutes := parsePHPRoutes(path, node, fileContent)

	batchSave := make(map[string]map[string]Route)
	for _, route := range parsedRoutes {
		if _, ok := batchSave[route.FilePath]; !ok {
			batchSave[route.FilePath] = make(map[string]Route)
		}
		batchSave[route.FilePath][route.Name] = route
	}

	return idx.dataIndexer.BatchSaveItems(batchSave)
}

func (idx *RouteIndexer) RemovedFiles(paths []string) error {
	return idx.dataIndexer.BatchDeleteByFilePaths(paths)
}

func (idx *RouteIndexer) Close() error {
	return idx.dataIndexer.Close()
}

func (idx *RouteIndexer) Clear() error {
	return idx.dataIndexer.Clear()
}
